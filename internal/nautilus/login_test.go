package nautilus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecCredentialHelperProcess(t *testing.T) {
	if os.Getenv("PERCH_EXEC_HELPER") != "1" {
		return
	}
	var info map[string]any
	if json.Unmarshal([]byte(os.Getenv("KUBERNETES_EXEC_INFO")), &info) != nil {
		os.Exit(2)
	}
	spec := info["spec"].(map[string]any)
	input, _ := io.ReadAll(io.LimitReader(os.Stdin, 64))
	record := map[string]any{"version": info["apiVersion"], "interactive": spec["interactive"], "input": string(input), "cluster": spec["cluster"], "env": os.Getenv("PERCH_EXEC_EXTRA"), "args": os.Args}
	b, _ := json.Marshal(record)
	if p := os.Getenv("PERCH_EXEC_MARKER"); p != "" {
		if err := os.WriteFile(p, b, 0600); err != nil {
			os.Exit(3)
		}
	}
	mode := os.Getenv("PERCH_EXEC_MODE")
	if mode == "oversize" {
		os.Stdout.Write(bytes.Repeat([]byte("x"), maxKubeconfigBytes+1))
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}
	if mode == "slow" {
		time.Sleep(5 * time.Second)
	}
	fmt.Fprintln(os.Stderr, "Synthetic sign-in instructions")
	if mode == "failed" {
		fmt.Fprintln(os.Stdout, testBearer)
		os.Exit(7)
	}
	if mode == "invalid" {
		fmt.Fprintln(os.Stdout, "invalid "+testBearer)
		os.Exit(0)
	}
	expiry := time.Now().Add(time.Hour)
	if mode == "expired" {
		expiry = time.Now().Add(-time.Hour)
	}
	result := map[string]any{"apiVersion": info["apiVersion"], "kind": "ExecCredential", "status": map[string]any{"token": testBearer, "expirationTimestamp": expiry}}
	json.NewEncoder(os.Stdout).Encode(result)
	os.Exit(0)
}

func helperConfig(t *testing.T, mode string) *execCredentialConfig {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{"apiVersion": "client.authentication.k8s.io/v1", "command": exe, "args": []string{"-test.run=^TestExecCredentialHelperProcess$", "--", "literal;not-a-shell"}, "interactiveMode": "Always", "provideClusterInfo": true,
		"env": []map[string]string{{"name": "PERCH_EXEC_HELPER", "value": "1"}, {"name": "PERCH_EXEC_MODE", "value": mode}, {"name": "PERCH_EXEC_EXTRA", "value": "literal=value"}}}
	data, _ := json.Marshal(raw)
	cfg, err := parseExecConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestExplicitExecUsesProtocolAndNeverCopiesCredentialsToConsole(t *testing.T) {
	for _, mode := range []string{"Always", "IfAvailable", "Never"} {
		t.Run(mode, func(t *testing.T) {
			plugin := helperConfig(t, "")
			plugin.InteractiveMode = mode
			plugin.clusterConfig = json.RawMessage(`{"audience":"test-cluster"}`)
			marker := filepath.Join(t.TempDir(), "record")
			t.Setenv("PERCH_EXEC_MARKER", marker)
			t.Setenv("KUBERNETES_EXEC_INFO", `{"must":"be replaced"}`)
			var console bytes.Buffer
			cfg, err := plugin.credentials(context.Background(), HTTPSConfig{Server: "https://api.example.test", CertificateAuthority: []byte("synthetic CA")}, "", LoginIO{strings.NewReader("approved\n"), &console})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.BearerToken != testBearer || cfg.ExpiresAt.IsZero() {
				t.Fatal("lost private credential/expiry")
			}
			if strings.Contains(console.String(), testBearer) || !strings.Contains(console.String(), "Synthetic sign-in instructions") {
				t.Fatal("credential stdout and foreground instructions mixed")
			}
			data, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			var record map[string]any
			if json.Unmarshal(data, &record) != nil {
				t.Fatal("invalid helper record")
			}
			interactive := mode != "Never"
			if record["interactive"] != interactive || record["env"] != "literal=value" {
				t.Fatal("plugin contract lost", record)
			}
			if interactive && record["input"] != "approved\n" || !interactive && record["input"] != "" {
				t.Fatal("stdin did not respect mode", record)
			}
			cluster := record["cluster"].(map[string]any)
			if cluster["server"] != "https://api.example.test" || cluster["config"].(map[string]any)["audience"] != "test-cluster" {
				t.Fatal("cluster input missing")
			}
			args := record["args"].([]any)
			if args[len(args)-1] != "literal;not-a-shell" {
				t.Fatal("arguments were interpreted")
			}
		})
	}
}

func TestExecFailureExpiryCancellationAndRedaction(t *testing.T) {
	for mode, kind := range map[string]ErrorKind{"failed": Unauthenticated, "invalid": InvalidResponse, "expired": Unauthenticated, "slow": TimedOut, "oversize": LimitExceeded} {
		t.Run(mode, func(t *testing.T) {
			plugin := helperConfig(t, mode)
			ctx := context.Background()
			cancel := func() {}
			if mode == "slow" {
				ctx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
			}
			defer cancel()
			var console bytes.Buffer
			started := time.Now()
			got, err := plugin.credentials(ctx, HTTPSConfig{}, "", LoginIO{strings.NewReader(""), &console})
			checkQueryError(t, err, kind)
			if mode == "oversize" && time.Since(started) >= 4*time.Second {
				t.Fatal("oversized output did not promptly stop the credential process")
			}
			if got.BearerToken != "" || strings.Contains(console.String(), testBearer) {
				t.Fatal("failed login exposed credential output")
			}
		})
	}
	plugin := helperConfig(t, "")
	encoded, _ := json.Marshal(plugin)
	if text := fmt.Sprintf("%v %+v %#v %s", plugin, plugin, plugin, encoded); strings.Contains(text, "literal") || strings.Contains(text, "PERCH_EXEC") {
		t.Fatal("exec configuration formatting exposed args/env")
	}
}

func TestExecConfigAndCredentialValidation(t *testing.T) {
	base := `{"apiVersion":"client.authentication.k8s.io/v1","command":"kubectl","interactiveMode":"Never"}`
	for _, raw := range []string{`null`, `{}`, strings.Replace(base, `"Never"`, `"Sometimes"`, 1), strings.Replace(base, "/v1", "/v2", 1), strings.Replace(base, `,"interactiveMode":"Never"`, "", 1), strings.Replace(base, `"command":"kubectl"`, `"command":"kubectl","env":[{"name":"KUBERNETES_EXEC_INFO","value":"override"}]`, 1)} {
		_, err := parseExecConfig([]byte(raw))
		if err == nil {
			t.Fatal("invalid exec config accepted", raw)
		}
	}
	beta := strings.Replace(base, "/v1", "/v1beta1", 1)
	beta = strings.Replace(beta, `,"interactiveMode":"Never"`, "", 1)
	cfg, err := parseExecConfig([]byte(beta))
	if err != nil || cfg.InteractiveMode != "IfAvailable" {
		t.Fatal("v1beta1 default not supported", err)
	}
	for _, status := range []string{`null`, `{}`, `{"token":"x","clientCertificateData":"cert","clientKeyData":"key"}`, `{"clientCertificateData":"cert"}`, `{"token":"x","expirationTimestamp":"bad"}`} {
		data := []byte(`{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1","status":` + status + `}`)
		_, err := parseExecCredential(data, "client.authentication.k8s.io/v1", HTTPSConfig{})
		checkQueryError(t, err, InvalidResponse)
	}
	data := []byte(`{"kind":"ExecCredential","apiVersion":"client.authentication.k8s.io/v1beta1","status":{"token":"x"}}`)
	_, err = parseExecCredential(data, "client.authentication.k8s.io/v1", HTTPSConfig{})
	checkQueryError(t, err, InvalidResponse)
}

func TestExplicitLoginOpensOneCredentialSessionThenUsesHTTPSOnly(t *testing.T) {
	var requests atomic.Int32
	data := fixture(t, "jobs")
	server := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if requests.Load() > 1 {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+testBearer {
			t.Error("missing plugin credential")
		}
		w.Write(data)
	})
	dir := t.TempDir()
	marker := filepath.Join(dir, "plugin-record")
	t.Setenv("PERCH_EXEC_MARKER", marker)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	doc := configDocument(serverConfig(server))
	doc["users"].([]any)[0].(map[string]any)["user"] = map[string]any{"exec": map[string]any{"apiVersion": "client.authentication.k8s.io/v1", "command": exe, "args": []string{"-test.run=^TestExecCredentialHelperProcess$"}, "interactiveMode": "Never", "env": []map[string]string{{"name": "PERCH_EXEC_HELPER", "value": "1"}}}}
	config := filepath.Join(dir, "config.json")
	if err = os.WriteFile(config, configJSON(t, doc), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n/bin/cat \"$PERCH_TEST_CONFIG\"\n"
	if err = os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PERCH_TEST_CONFIG", config)
	if _, err = OpenKubeconfig(context.Background(), "selected", "research", config); err == nil {
		t.Fatal("background path allowed exec")
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("background config path ran plugin")
	}
	var console bytes.Buffer
	conn, err := OpenKubeconfigWithLogin(context.Background(), "selected", "research", config, LoginIO{strings.NewReader(""), &console})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	markerBefore, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	if result := conn.Client.ListJobs(context.Background()); !result.OK() {
		t.Fatal("plugin-backed HTTPS request failed", result.Error)
	}
	if result := conn.Client.ListJobs(context.Background()); result.Error == nil || result.Error.Kind != Unauthenticated {
		t.Fatal("401 did not require explicit sign-in")
	}
	markerAfter, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !markerBefore.ModTime().Equal(markerAfter.ModTime()) {
		t.Fatal("API query silently renewed credential")
	}
}
