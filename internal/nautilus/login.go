package nautilus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LoginIO is supplied only for a foreground, explicitly requested sign-in.
// Stdout is intentionally absent: ExecCredential output always stays private.
type LoginIO struct {
	Stdin  io.Reader
	Stderr io.Writer
}

type execCredentialConfig struct {
	APIVersion string   `json:"apiVersion"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Env        []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"env"`
	InteractiveMode    string `json:"interactiveMode"`
	ProvideClusterInfo bool   `json:"provideClusterInfo"`
	clusterConfig      json.RawMessage
}

func (*execCredentialConfig) String() string               { return "nautilus.ExecCredentialConfig{redacted}" }
func (e *execCredentialConfig) GoString() string           { return e.String() }
func (*execCredentialConfig) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func parseExecConfig(data []byte) (*execCredentialConfig, error) {
	invalid := func() (*execCredentialConfig, error) {
		return nil, queryError(Invalid, "Unsupported or invalid exec credential configuration")
	}
	var raw map[string]json.RawMessage
	if len(data) > 64<<10 || validateJSON(data) != nil || json.Unmarshal(data, &raw) != nil {
		return invalid()
	}
	for key := range raw {
		switch key {
		case "apiVersion", "command", "args", "env", "interactiveMode", "provideClusterInfo", "installHint":
		default:
			return invalid()
		}
	}
	var cfg execCredentialConfig
	if json.Unmarshal(data, &cfg) != nil || cfg.Command == "" || len(cfg.Command) > 4096 || strings.ContainsAny(cfg.Command, "\x00\r\n") || len(cfg.Args) > 128 || len(cfg.Env) > 128 {
		return invalid()
	}
	switch cfg.APIVersion {
	case "client.authentication.k8s.io/v1":
		if cfg.InteractiveMode == "" {
			return invalid()
		}
	case "client.authentication.k8s.io/v1beta1":
		if cfg.InteractiveMode == "" {
			cfg.InteractiveMode = "IfAvailable"
		}
	default:
		return invalid()
	}
	switch cfg.InteractiveMode {
	case "Always", "IfAvailable", "Never":
	default:
		return invalid()
	}
	for _, arg := range cfg.Args {
		if strings.ContainsRune(arg, 0) {
			return invalid()
		}
	}
	seen := map[string]bool{}
	for _, env := range cfg.Env {
		if env.Name == "" || strings.ContainsAny(env.Name, "=\x00\r\n") || strings.ContainsRune(env.Value, 0) || env.Name == "KUBERNETES_EXEC_INFO" || seen[env.Name] {
			return invalid()
		}
		seen[env.Name] = true
	}
	return &cfg, nil
}

// Stream the plugin's foreground instructions to the requesting terminal, with
// a byte cap. These messages may contain login URLs/codes; never record them in
// diagnostics or send them through JSON/MCP. Credentials on stdout are separate.
type loginMessages struct {
	writer    io.Writer
	remaining int
}

func (w *loginMessages) Write(p []byte) (int, error) {
	n := len(p)
	count := min(n, w.remaining)
	if count > 0 {
		if _, err := w.writer.Write(p[:count]); err != nil {
			return 0, err
		}
		w.remaining -= count
	}
	return n, nil
}

func (e *execCredentialConfig) credentials(parent context.Context, cfg HTTPSConfig, filename string, streams LoginIO) (HTTPSConfig, error) {
	fail := func(kind ErrorKind, message string) (HTTPSConfig, error) {
		return HTTPSConfig{}, queryError(kind, message)
	}
	command := e.Command
	if !filepath.IsAbs(command) && strings.ContainsAny(command, "/\\") {
		if filename == "" {
			return fail(Invalid, "Relative credential command needs an explicit --kubeconfig file")
		}
		absolute, err := filepath.Abs(filename)
		if err != nil {
			return fail(Invalid, "Unable to resolve credential configuration path")
		}
		command = filepath.Join(filepath.Dir(absolute), command)
	}
	executable, err := exec.LookPath(command)
	if err != nil {
		return fail(Unauthenticated, "Configured credential plugin is unavailable; install or configure it before signing in")
	}
	spec := map[string]any{"interactive": e.InteractiveMode != "Never"}
	if e.ProvideClusterInfo {
		cluster := map[string]any{"server": cfg.Server}
		if len(cfg.CertificateAuthority) > 0 {
			cluster["certificate-authority-data"] = cfg.CertificateAuthority
		}
		if e.clusterConfig != nil {
			cluster["config"] = e.clusterConfig
		}
		spec["cluster"] = cluster
	}
	info, err := json.Marshal(map[string]any{"apiVersion": e.APIVersion, "kind": "ExecCredential", "spec": spec})
	if err != nil || len(info) > 64<<10 {
		return fail(LimitExceeded, "Credential plugin input exceeds its limit")
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, e.Args...)
	cmd.Env = os.Environ()
	for _, env := range e.Env {
		cmd.Env = append(cmd.Env, env.Name+"="+env.Value)
	}
	cmd.Env = append(cmd.Env, "KUBERNETES_EXEC_INFO="+string(info))
	if e.InteractiveMode != "Never" {
		cmd.Stdin = streams.Stdin
	}
	cmd.Stderr = &loginMessages{writer: streams.Stderr, remaining: 32 << 10}
	result := configBuffer{cancel: cancel} // Private bounded output; never written to a terminal.
	cmd.Stdout = &result
	cmd.WaitDelay = time.Second
	if _, err := fmt.Fprintln(streams.Stderr, "Perch: signing in through the selected context's credential plugin…"); err != nil {
		return fail(ReadFailed, "Unable to display sign-in instructions")
	}
	err = cmd.Run()
	if result.overflow {
		return fail(LimitExceeded, "Credential plugin response exceeded its limit")
	}
	if ctx.Err() != nil {
		return HTTPSConfig{}, readError(ctx.Err())
	}
	if err != nil {
		return fail(Unauthenticated, "Credential sign-in did not complete; no API request was made")
	}
	return parseExecCredential(result.Bytes(), e.APIVersion, cfg)
}

func parseExecCredential(data []byte, version string, cfg HTTPSConfig) (HTTPSConfig, error) {
	invalid := func() (HTTPSConfig, error) {
		return HTTPSConfig{}, queryError(InvalidResponse, "Invalid credential plugin response")
	}
	if len(data) > maxKubeconfigBytes || validateJSON(data) != nil {
		return invalid()
	}
	var response struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Status     *struct {
			Token       string     `json:"token"`
			Certificate string     `json:"clientCertificateData"`
			Key         string     `json:"clientKeyData"`
			Expires     *time.Time `json:"expirationTimestamp"`
		} `json:"status"`
	}
	if json.Unmarshal(bytes.TrimSpace(data), &response) != nil || response.Kind != "ExecCredential" || response.APIVersion != version || response.Status == nil {
		return invalid()
	}
	s := response.Status
	if (s.Token != "") == (s.Certificate != "" || s.Key != "") || (s.Certificate == "") != (s.Key == "") {
		return invalid()
	}
	if s.Expires != nil {
		if !time.Now().Before(*s.Expires) {
			return HTTPSConfig{}, queryError(Unauthenticated, "Credential plugin returned an expired session")
		}
		cfg.ExpiresAt = *s.Expires
	}
	cfg.BearerToken = s.Token
	cfg.ClientCertificate = []byte(s.Certificate)
	cfg.ClientKey = []byte(s.Key)
	return cfg, nil
}
