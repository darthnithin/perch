package nautilus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

// Connection exposes only the selected scope and client. Credentials remain in
// the private HTTPS reader. Close it when a command or workspace finishes.
type Connection struct {
	Scope
	Context string
	Client  *Client
	reader  *HTTPSReader
}

func (c *Connection) Close() { c.reader.Close() }

// OpenKubeconfig reads one explicitly named context, without changing current
// context or executing its auth plugins. Exec authentication requires the separate
// explicitly requested OpenKubeconfigWithLogin path.
func OpenKubeconfig(ctx context.Context, contextName, namespace, filename string) (*Connection, error) {
	return openKubeconfig(ctx, contextName, namespace, filename, nil)
}

// OpenKubeconfigWithLogin permits one foreground credential-plugin invocation.
// Never call this from a refresh/retry loop. The caller must obtain explicit
// login intent and provide interactive terminal streams.
func OpenKubeconfigWithLogin(ctx context.Context, contextName, namespace, filename string, login LoginIO) (*Connection, error) {
	if login.Stdin == nil || login.Stderr == nil {
		return nil, queryError(Invalid, "Sign-in requires an interactive terminal")
	}
	return openKubeconfig(ctx, contextName, namespace, filename, &login)
}

func openKubeconfig(ctx context.Context, contextName, namespace, filename string, login *LoginIO) (*Connection, error) {
	if !safeIdentity(contextName) || !namespacePattern.MatchString(namespace) || len(filename) > 4096 || strings.ContainsAny(filename, "\x00\r\n") {
		return nil, queryError(Invalid, "An explicit context and namespace are required")
	}
	data, err := readKubeconfig(ctx, contextName, filename)
	if err != nil {
		return nil, err
	}
	scope, cfg, plugin, err := parseKubeconfigSelection(data, contextName, namespace, login != nil)
	if err != nil {
		return nil, err
	}
	if plugin != nil {
		// Validate the endpoint/trust before starting sign-in. Construction and close
		// do not contact a server; the placeholder is never sent or persisted.
		probe := cfg
		probe.BearerToken = "connection-validation-only"
		check, err := NewHTTPSReader(scope, probe)
		if err != nil {
			return nil, err
		}
		check.Close()
		cfg, err = plugin.credentials(ctx, cfg, filename, *login)
		if err != nil {
			return nil, err
		}
	}
	reader, err := NewHTTPSReader(scope, cfg)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(scope, reader)
	if err != nil {
		reader.Close()
		return nil, err
	}
	return &Connection{Scope: scope, Context: contextName, Client: client, reader: reader}, nil
}

const maxKubeconfigBytes = 8 << 20

var errConfigLimit = errors.New("configuration output limit")

type configBuffer struct {
	// Do not embed bytes.Buffer: its promoted ReadFrom lets io.Copy bypass Write.
	buffer   bytes.Buffer
	overflow bool
	cancel   context.CancelFunc
}

func (b *configBuffer) Len() int      { return b.buffer.Len() }
func (b *configBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *configBuffer) Write(p []byte) (int, error) {
	if len(p) > maxKubeconfigBytes-b.Len() {
		b.overflow = true
		if b.cancel != nil {
			b.cancel()
		}
		return 0, errConfigLimit
	}
	return b.buffer.Write(p)
}
func readKubeconfig(parent context.Context, name, filename string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	args := []string{"config", "view", "--context=" + name, "--minify", "--flatten", "--raw", "--output=json"}
	if filename != "" {
		args = append(args, "--kubeconfig="+filename)
	}
	// This kubectl subcommand only reads/minifies configuration. Never substitute
	// `get`, which could execute an arbitrary credential plugin before we inspect
	// the selected user's authentication settings.
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output := configBuffer{cancel: cancel}
	cmd.Stdout = &output
	cmd.Stderr = io.Discard // Raw configuration errors may include confidential data.
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if output.overflow {
		return nil, queryError(LimitExceeded, "Selected Kubernetes configuration exceeds 8 MiB")
	}
	if ctx.Err() != nil {
		return nil, readError(ctx.Err())
	}
	if err != nil {
		return nil, queryError(ReadFailed, "Unable to read selected Kubernetes context; check kubectl and kubeconfig")
	}
	return output.Bytes(), nil
}

func parseKubeconfig(data []byte, contextName, namespace string) (Scope, HTTPSConfig, error) {
	scope, cfg, _, err := parseKubeconfigSelection(data, contextName, namespace, false)
	return scope, cfg, err
}
func parseKubeconfigSelection(data []byte, contextName, namespace string, allowExec bool) (Scope, HTTPSConfig, *execCredentialConfig, error) {
	fail := func(message string) (Scope, HTTPSConfig, *execCredentialConfig, error) {
		return Scope{}, HTTPSConfig{}, nil, queryError(Invalid, message)
	}
	invalid := func() (Scope, HTTPSConfig, *execCredentialConfig, error) {
		return fail("Invalid selected Kubernetes configuration")
	}
	if len(data) > maxKubeconfigBytes || validateJSON(data) != nil || !safeIdentity(contextName) || !namespacePattern.MatchString(namespace) {
		return invalid()
	}
	var selected struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
		Current    string `json:"current-context"`
		Contexts   []struct {
			Name    string `json:"name"`
			Context struct {
				Cluster string `json:"cluster"`
				User    string `json:"user"`
			} `json:"context"`
		} `json:"contexts"`
		Clusters []struct {
			Name    string                     `json:"name"`
			Cluster map[string]json.RawMessage `json:"cluster"`
		} `json:"clusters"`
		Users []struct {
			Name string                     `json:"name"`
			User map[string]json.RawMessage `json:"user"`
		} `json:"users"`
	}
	if json.Unmarshal(data, &selected) != nil || selected.Kind != "Config" || selected.APIVersion != "v1" || selected.Current != contextName || len(selected.Contexts) != 1 || len(selected.Clusters) != 1 || len(selected.Users) != 1 {
		return invalid()
	}
	named, cluster, user := selected.Contexts[0], selected.Clusters[0], selected.Users[0]
	if named.Name != contextName || named.Context.Cluster != cluster.Name || named.Context.User != user.Name || !safeIdentity(cluster.Name) || !safeIdentity(user.Name) {
		return invalid()
	}
	var plugin *execCredentialConfig
	if raw, ok := user.User["exec"]; ok {
		if !allowExec {
			return fail("This context uses exec authentication; use --login in a terminal to sign in; no credential command was run")
		}
		var err error
		plugin, err = parseExecConfig(raw)
		if err != nil {
			return Scope{}, HTTPSConfig{}, nil, err
		}
		for key := range user.User {
			if key != "exec" && key != "extensions" {
				return fail("Exec authentication cannot be combined with other credentials or impersonation")
			}
		}
	}
	if _, ok := user.User["auth-provider"]; ok {
		return fail("This context uses an unsupported authentication provider; no credential refresh was attempted")
	}
	var cfg HTTPSConfig
	decode := func(raw json.RawMessage, target any) bool {
		return string(raw) != "null" && json.Unmarshal(raw, target) == nil
	}
	for key, value := range cluster.Cluster {
		var str string
		var flag bool
		switch key {
		case "server":
			if !decode(value, &cfg.Server) {
				return invalid()
			}
		case "certificate-authority-data":
			if !decode(value, &cfg.CertificateAuthority) {
				return invalid()
			}
		case "certificate-authority", "tls-server-name", "proxy-url":
			if !decode(value, &str) {
				return invalid()
			}
			if str != "" {
				return fail("Selected context has unsupported certificate, TLS-name or proxy settings")
			}
		case "insecure-skip-tls-verify":
			if !decode(value, &flag) {
				return invalid()
			}
			if flag {
				return fail("Selected context disables TLS verification; Perch requires verified TLS")
			}
		case "disable-compression":
			if !decode(value, &flag) {
				return invalid()
			}
		case "extensions":
			if plugin != nil && plugin.ProvideClusterInfo {
				var extensions []struct {
					Name      string          `json:"name"`
					Extension json.RawMessage `json:"extension"`
				}
				if json.Unmarshal(value, &extensions) != nil {
					return invalid()
				}
				for _, extension := range extensions {
					if extension.Name == "client.authentication.k8s.io/exec" {
						if plugin.clusterConfig != nil {
							return invalid()
						}
						plugin.clusterConfig = extension.Extension
					}
				}
			}
		default:
			return fail("Selected context has unsupported cluster settings")
		}
	}
	for key, value := range user.User {
		var str string
		switch key {
		case "exec": // Already validated; execution is deferred to explicit login.
		case "token":
			if !decode(value, &cfg.BearerToken) {
				return invalid()
			}
		case "client-certificate-data":
			if !decode(value, &cfg.ClientCertificate) {
				return invalid()
			}
		case "client-key-data":
			if !decode(value, &cfg.ClientKey) {
				return invalid()
			}
		case "client-certificate", "client-key":
			if !decode(value, &str) {
				return invalid()
			}
			if str != "" {
				return fail("Selected client certificate was not flattened")
			}
		case "extensions":
		default:
			return fail("Selected context has unsupported credentials or impersonation settings")
		}
	}
	// Context/cluster aliases can be renamed or reused for a different endpoint.
	// Scope identity uses endpoint + trust material, not those mutable aliases.
	fingerprint := sha256.Sum256(append([]byte(strings.TrimSuffix(cfg.Server, "/")+"\x00"), cfg.CertificateAuthority...))
	scope := Scope{Cluster: "k8s:" + hex.EncodeToString(fingerprint[:]), Namespace: namespace}
	return scope, cfg, plugin, nil
}
