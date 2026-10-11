package nautilus

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func configDocument(cfg HTTPSConfig) map[string]any {
	return map[string]any{"kind": "Config", "apiVersion": "v1", "current-context": "selected",
		"contexts": []any{map[string]any{"name": "selected", "context": map[string]any{"cluster": "cluster-alias", "user": "account", "namespace": "wrong-default"}}},
		"clusters": []any{map[string]any{"name": "cluster-alias", "cluster": map[string]any{"server": cfg.Server, "certificate-authority-data": cfg.CertificateAuthority}}},
		"users":    []any{map[string]any{"name": "account", "user": map[string]any{"token": cfg.BearerToken}}}}
}
func configJSON(t *testing.T, doc map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func configMember(doc map[string]any, kind, key string) map[string]any {
	return doc[kind].([]any)[0].(map[string]any)[key].(map[string]any)
}
func staticConfigDocument() map[string]any {
	return configDocument(HTTPSConfig{Server: "https://api.example.test", BearerToken: testBearer, CertificateAuthority: []byte{}})
}

func TestKubeconfigSelectionAndIdentity(t *testing.T) {
	doc := staticConfigDocument()
	scope, cfg, err := parseKubeconfig(configJSON(t, doc), "selected", "research")
	if err != nil || scope.Namespace != "research" || cfg.Server != "https://api.example.test" || cfg.BearerToken != testBearer {
		t.Fatal("lost explicit configuration selection", err)
	}
	// Aliases may change without changing the connection; never use them as UID scope.
	doc["clusters"].([]any)[0].(map[string]any)["name"] = "renamed"
	configMember(doc, "contexts", "context")["cluster"] = "renamed"
	same, _, err := parseKubeconfig(configJSON(t, doc), "selected", "research")
	if err != nil || same != scope {
		t.Fatal("alias changed identity", err)
	}
	configMember(doc, "clusters", "cluster")["server"] = "https://different.example.test"
	changed, _, err := parseKubeconfig(configJSON(t, doc), "selected", "research")
	if err != nil || changed.Cluster == scope.Cluster {
		t.Fatal("endpoint reuse inherited previous cluster identity", err)
	}
}

func TestKubeconfigRejectsUnimplementedAuthAndContextSettings(t *testing.T) {
	changes := []func(map[string]any){
		func(d map[string]any) { d["current-context"] = "other" },
		func(d map[string]any) { d["contexts"] = []any{} },
		func(d map[string]any) { configMember(d, "contexts", "context")["user"] = "other" },
		func(d map[string]any) {
			configMember(d, "users", "user")["exec"] = map[string]any{"command": "never-run", "args": []string{"server-secret"}}
		},
		func(d map[string]any) {
			configMember(d, "users", "user")["auth-provider"] = map[string]any{"name": "oidc"}
		},
		func(d map[string]any) { configMember(d, "users", "user")["tokenFile"] = "server-secret" },
		func(d map[string]any) { configMember(d, "users", "user")["as"] = "someone-else" },
		func(d map[string]any) { configMember(d, "users", "user")["username"] = "server-secret" },
		func(d map[string]any) { configMember(d, "users", "user")["future-auth"] = "server-secret" },
		func(d map[string]any) { configMember(d, "clusters", "cluster")["insecure-skip-tls-verify"] = true },
		func(d map[string]any) {
			configMember(d, "clusters", "cluster")["tls-server-name"] = "different.example.test"
		},
		func(d map[string]any) {
			configMember(d, "clusters", "cluster")["proxy-url"] = "https://user:proxy-secret@example.test"
		},
		func(d map[string]any) {
			configMember(d, "clusters", "cluster")["certificate-authority"] = "/private/ca"
		},
		func(d map[string]any) { configMember(d, "clusters", "cluster")["certificate-authority-data"] = false },
		func(d map[string]any) { configMember(d, "clusters", "cluster")["server"] = nil },
		func(d map[string]any) { configMember(d, "clusters", "cluster")["other-route"] = "server-secret" },
	}
	for _, change := range changes {
		doc := staticConfigDocument()
		change(doc)
		_, cfg, err := parseKubeconfig(configJSON(t, doc), "selected", "research")
		checkQueryError(t, err, Invalid)
		if cfg.BearerToken != "" {
			t.Fatal("invalid configuration returned partial credential")
		}
	}
	doc := staticConfigDocument()
	data := configJSON(t, doc)
	duplicate := append([]byte(`{"current-context":"other",`), data[1:]...)
	_, _, err := parseKubeconfig(duplicate, "selected", "research")
	checkQueryError(t, err, Invalid)
}

func TestOpenKubeconfigRunsOnlyOfflineSelectedConfigCommand(t *testing.T) {
	data := fixture(t, "jobs")
	s := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/namespaces/research/") || r.Header.Get("Authorization") != "Bearer "+testBearer {
			t.Error("lost scope/credentials")
		}
		w.Write(data)
	})
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "selected.json")
	argsPath := filepath.Join(dir, "args")
	if err := os.WriteFile(cfgPath, configJSON(t, configDocument(serverConfig(s))), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PERCH_TEST_ARGS\"\n/bin/cat \"$PERCH_TEST_CONFIG\"\n"
	if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("PERCH_TEST_ARGS", argsPath)
	t.Setenv("PERCH_TEST_CONFIG", cfgPath)
	supplied := filepath.Join(dir, "config ; this is an argument")
	c, err := OpenKubeconfig(context.Background(), "selected", "research", supplied)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	got := c.Client.ListJobs(context.Background())
	if !got.OK() || len(got.Collection.Items) != 8 {
		t.Fatal("selected config did not reach HTTP adapter", got.Error)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"config", "view", "--context=selected", "--minify", "--flatten", "--raw", "--output=json", "--kubeconfig=" + supplied}
	if !reflect.DeepEqual(strings.Split(strings.TrimSuffix(string(args), "\n"), "\n"), want) {
		t.Fatal("configuration command changed", string(args))
	}
}

func TestConfigCommandFailureAndBoundsNeverExposeOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	for _, script := range []string{"#!/bin/sh\nprintf 'server-secret' >&2\nexit 1\n", "#!/bin/sh\nprintf 'server-secret'\n"} {
		if err := os.WriteFile(filepath.Join(dir, "kubectl"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		_, err := OpenKubeconfig(context.Background(), "selected", "research", "")
		if err == nil || strings.Contains(err.Error(), "server-secret") {
			t.Fatal("config failure exposed output")
		}
	}
	var b configBuffer
	if _, err := b.Write(make([]byte, maxKubeconfigBytes+1)); err != errConfigLimit || !b.overflow || b.Len() != 0 {
		t.Fatal("configuration limit failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := OpenKubeconfig(ctx, "selected", "research", "")
	checkQueryError(t, err, Cancelled)
}
