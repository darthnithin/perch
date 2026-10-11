package nautilus

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

var fixtureScope = Scope{Cluster: "fixture-nautilus", Namespace: "research"}

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/nautilus/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestJobOutcomesAndIndexedProgress(t *testing.T) {
	page, err := ParseJobs(fixture(t, "jobs"), fixtureScope)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 8 || page.ResourceVersion != "opaque/jobs:100" {
		t.Fatalf("wrong page: %+v", page)
	}
	want := []string{"Running", "Running", "Completing", "Complete", "Failed", "Pending", "Failing", "Suspended"}
	for i, job := range page.Items {
		if job.State() != want[i] || job.Terminal() != (i == 3 || i == 4) {
			t.Errorf("%s state=%s terminal=%v", job.Name, job.State(), job.Terminal())
		}
	}
	j := page.Items[1]
	if *j.Completions != 1000000 || *j.Parallelism != 4 || *j.Succeeded != 999990 || j.CompletedIndexes != "0-999989" || j.FailedIndexes != "999993" {
		t.Fatalf("lost indexed semantics: %+v", j)
	}
	if page.Items[5].Completions != nil || page.Items[5].Succeeded != nil || page.Items[5].Started != nil {
		t.Fatal("invented omitted count/timestamp")
	}
	if page.Items[3].Completed == nil || page.Items[3].Completed.Format("15:04:05") != "10:03:00" {
		t.Fatal("lost completion timestamp")
	}
}

func TestPodRetriesContainersAndOwnerIdentity(t *testing.T) {
	jobs, err := ParseJobs(fixture(t, "jobs"), fixtureScope)
	if err != nil {
		t.Fatal(err)
	}
	pods, err := ParsePods(fixture(t, "pods"), fixtureScope)
	if err != nil {
		t.Fatal(err)
	}
	owned := PodsForJob(jobs.Items[0], pods.Items)
	if len(owned) != 2 || owned[0].Identity() == owned[1].Identity() {
		t.Fatal("replacement pod lost or treated as restart")
	}
	if owned[0].Phase != "Failed" || owned[0].Containers[0].State.Reason != "OOMKilled" || jobs.Items[0].Terminal() {
		t.Fatal("pod failure became Job outcome")
	}
	cs := owned[1].Containers
	if len(cs) != 4 || cs[0].Role != "init" || cs[1].Role != "sidecar" || cs[2].Name != "compute" || cs[3].Name != "results" {
		t.Fatalf("wrong container order/roles: %+v", cs)
	}
	compute := cs[2]
	if *compute.Restarts != 2 || compute.State.Phase != "Running" || compute.Last.Phase != "Terminated" || compute.Last.Reason != "OOMKilled" || *compute.Last.ExitCode != 137 {
		t.Fatalf("restart/current/last conflated: %+v", compute)
	}
	if cs[3].State.Reason != "ImagePullBackOff" || *cs[3].Ready {
		t.Fatal("missing image pull problem")
	}
	if pods.Items[2].Conditions[0].Reason != "Unschedulable" || pods.Items[2].CompletionIndex != "999990" {
		t.Fatal("missing scheduling/index evidence")
	}
	unknown := pods.Items[2].Containers[0]
	if unknown.State.Phase != "Unknown" || unknown.Restarts != nil || unknown.Ready != nil {
		t.Fatalf("invented container status: %+v", unknown)
	}
	if len(PodsForJob(jobs.Items[5], pods.Items)) != 0 {
		t.Fatal("adopted stale owner or mutable label after name reuse")
	}
	if pods.Items[4].Containers[1].Role != "ephemeral" {
		t.Fatal("ephemeral container disappeared")
	}

	// Equal UID strings from another cluster/namespace are not the same object.
	for _, scope := range []Scope{{"other", "research"}, {"fixture-nautilus", "other"}} {
		j := jobs.Items[0]
		j.Scope = scope
		if j.Identity() == jobs.Items[0].Identity() || len(PodsForJob(j, pods.Items)) != 0 {
			t.Fatal("cross-scope identity collision")
		}
	}
	old := jobs.Items[5]
	old.UID = "job-deleted"
	if old.Identity() == jobs.Items[5].Identity() || len(PodsForJob(old, pods.Items)) != 1 {
		t.Fatal("name reused as identity")
	}
	changed := owned[0]
	changed.Owners = append([]Owner(nil), changed.Owners...)
	changed.Owners[0].Controller = false
	if len(PodsForJob(jobs.Items[0], []Pod{changed})) != 0 {
		t.Fatal("noncontroller owner grouped")
	}
}

func TestPageEnvelopeAndBounds(t *testing.T) {
	base := `{"apiVersion":"batch/v1","kind":"JobList","metadata":{"resourceVersion":"rv"},"items":[]}`
	for _, data := range []string{
		`null`, `{}`, `[]`, `{"kind":"Status","status":"Failure"}`, strings.Replace(base, `"items":[]`, `"items":null`, 1),
		strings.Replace(base, `"items":[]`, `"items":{}`, 1), strings.Replace(base, `"items":[]`, `"items":[null]`, 1),
		strings.Replace(base, "JobList", "PodList", 1), strings.Replace(base, "batch/v1", "batch/v2", 1),
		strings.Replace(base, `"resourceVersion":"rv"`, `"resourceVersion":""`, 1),
		strings.Replace(base, `"resourceVersion":"rv"`, `"resourceVersion":"rv","remainingItemCount":-1`, 1),
		strings.Replace(base, `"kind":"JobList"`, `"kind":"JobList","kind":"JobList"`, 1),
		base + `{}`, strings.Repeat("[", 66) + "0" + strings.Repeat("]", 66),
	} {
		if got, err := ParseJobs([]byte(data), fixtureScope); err == nil || len(got.Items) != 0 {
			t.Errorf("accepted invalid input %.180s", data)
		}
	}
	if _, err := ParseJobs(bytes.Repeat([]byte(" "), MaxPageBytes+1), fixtureScope); err == nil {
		t.Fatal("unbounded input")
	}
	large := strings.Replace(base, `"items":[]`, `"items":[`+strings.Repeat("{},", MaxPageItems)+`{}`+`]`, 1)
	if _, err := ParseJobs([]byte(large), fixtureScope); err == nil {
		t.Fatal("unbounded items")
	}
	for _, scope := range []Scope{{}, {Cluster: "c"}, {Cluster: "c", Namespace: "\x1b[31mresearch"}} {
		if _, err := ParseJobs([]byte(base), scope); err == nil {
			t.Fatal("accepted missing/unsafe scope")
		}
	}
	page, err := ParseJobs([]byte(strings.Replace(base, `"resourceVersion":"rv"`, `"resourceVersion":"rv","continue":"opaque+/=","remainingItemCount":5`, 1)), fixtureScope)
	if err != nil || page.Continue != "opaque+/=" || page.Remaining == nil || *page.Remaining != 5 {
		t.Fatalf("lost pagination: %+v %v", page, err)
	}
	if len(page.Items) != 0 {
		t.Fatal("empty list was not empty")
	}
}

func mutate(t *testing.T, data []byte, change func(map[string]any)) []byte {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	change(obj)
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func item(root map[string]any, i int) map[string]any {
	return root["items"].([]any)[i].(map[string]any)
}
func field(root map[string]any, name string) map[string]any { return root[name].(map[string]any) }

func TestInvalidJobRecordsNeverBecomeEmptySuccess(t *testing.T) {
	changes := map[string]func(map[string]any){
		"wrong namespace":  func(r map[string]any) { field(item(r, 0), "metadata")["namespace"] = "elsewhere" },
		"missing uid":      func(r map[string]any) { delete(field(item(r, 0), "metadata"), "uid") },
		"wrong kind":       func(r map[string]any) { item(r, 0)["kind"] = "Pod" },
		"bad timestamp":    func(r map[string]any) { field(item(r, 0), "metadata")["creationTimestamp"] = "yesterday" },
		"negative count":   func(r map[string]any) { field(item(r, 0), "status")["active"] = -1 },
		"fractional count": func(r map[string]any) { field(item(r, 0), "status")["active"] = 1.5 },
		"duplicate object": func(r map[string]any) { r["items"] = append(r["items"].([]any), item(r, 0)) },
		"conflicting terminal": func(r map[string]any) {
			field(item(r, 0), "status")["conditions"] = []any{map[string]any{"type": "Complete", "status": "True"}, map[string]any{"type": "Failed", "status": "True"}}
		},
		"bad condition": func(r map[string]any) {
			field(item(r, 0), "status")["conditions"] = []any{map[string]any{"type": "Complete", "status": true}}
		},
		"duplicate condition": func(r map[string]any) {
			field(item(r, 0), "status")["conditions"] = []any{map[string]any{"type": "Complete", "status": "True"}, map[string]any{"type": "Complete", "status": "False"}}
		},
		"overlapping indexes": func(r map[string]any) { field(item(r, 1), "status")["completedIndexes"] = "0-7,5-9" },
		"out of bound index":  func(r map[string]any) { field(item(r, 1), "status")["failedIndexes"] = "1000000" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			got, err := ParseJobs(mutate(t, fixture(t, "jobs"), change), fixtureScope)
			if err == nil || len(got.Items) != 0 {
				t.Fatalf("accepted corrupt page: %+v", got)
			}
		})
	}
}

func TestPodStateValidation(t *testing.T) {
	changes := map[string]func(map[string]any){
		"wrong namespace":   func(r map[string]any) { field(item(r, 0), "metadata")["namespace"] = "elsewhere" },
		"duplicate uid":     func(r map[string]any) { r["items"] = append(r["items"].([]any), item(r, 0)) },
		"missing container": func(r map[string]any) { field(item(r, 0), "spec")["containers"] = []any{} },
		"duplicate name": func(r map[string]any) {
			field(item(r, 0), "spec")["containers"] = []any{map[string]any{"name": "compute"}, map[string]any{"name": "compute"}}
		},
		"unmatched status": func(r map[string]any) {
			field(item(r, 0), "spec")["containers"] = []any{map[string]any{"name": "other"}}
		},
		"two controllers": func(r map[string]any) {
			m := field(item(r, 0), "metadata")
			m["ownerReferences"] = append(m["ownerReferences"].([]any), map[string]any{"apiVersion": "batch/v1", "kind": "Job", "name": "other", "uid": "other", "controller": true})
		},
		"bad index": func(r map[string]any) {
			field(field(item(r, 2), "metadata"), "annotations")["batch.kubernetes.io/job-completion-index"] = "0-3"
		},
		"negative restart": func(r map[string]any) {
			field(item(r, 0), "status")["containerStatuses"].([]any)[0].(map[string]any)["restartCount"] = -1
		},
		"missing exit": func(r map[string]any) {
			state := field(field(item(r, 0), "status")["containerStatuses"].([]any)[0].(map[string]any), "state")
			delete(field(state, "terminated"), "exitCode")
		},
		"multiple states": func(r map[string]any) {
			state := field(field(item(r, 0), "status")["containerStatuses"].([]any)[0].(map[string]any), "state")
			state["running"] = map[string]any{}
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			if got, err := ParsePods(mutate(t, fixture(t, "pods"), change), fixtureScope); err == nil || len(got.Items) != 0 {
				t.Fatal("accepted corrupt Pod page")
			}
		})
	}
}

func TestUnknownFieldsAndControlSanitization(t *testing.T) {
	data := mutate(t, fixture(t, "pods"), func(r map[string]any) {
		field(item(r, 0), "status")["message"] = "plain\x1b[31mred\x1b[0m\ntext\x1b]52;c;payload\x07"
		field(item(r, 0), "status")["futureFeature"] = map[string]any{"enabled": true}
		field(item(r, 0), "spec")["env"] = []any{map[string]any{"name": "NOT_EXPORTED", "value": "SENSITIVE_FIXTURE_MARKER"}}
	})
	pods, err := ParsePods(data, fixtureScope)
	if err != nil {
		t.Fatal(err)
	}
	if pods.Items[0].Message != "plainred text" {
		t.Fatalf("unsafe or altered message: %q", pods.Items[0].Message)
	}
	out, err := json.Marshal(pods)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("SENSITIVE_FIXTURE_MARKER")) {
		t.Fatal("exported unrelated configuration")
	}
	data = bytes.Replace(data, []byte(`"phase":"Failed"`), []byte(`"phase":123`), 1)
	_, err = ParsePods(data, fixtureScope)
	if err == nil || strings.Contains(err.Error(), "SENSITIVE_FIXTURE_MARKER") {
		t.Fatal("invalid fields accepted or raw response leaked")
	}
}

func TestNoTerminalInferenceFromCountsOrDeletion(t *testing.T) {
	for _, conditions := range [][]Condition{nil, {{Type: "Complete", Status: "False"}}, {{Type: "Complete", Status: "Unknown"}}} {
		j := Job{Succeeded: new(int64(1)), Completions: new(int64(1)), Conditions: conditions}
		if j.Terminal() {
			t.Fatal("count or false condition interpreted as terminal")
		}
	}
	page, err := ParseJobs(fixture(t, "jobs"), fixtureScope)
	if err != nil {
		t.Fatal(err)
	}
	j := page.Items[0]
	j.Deleted = j.Created
	if j.State() != "Deleting" || j.Terminal() {
		t.Fatal("deletion implies completion")
	}
	j.SuspendRequested = true
	j.Deleted = nil
	if j.State() == "Suspended" {
		t.Fatal("suspension request is not observed suspension")
	}
}

func TestCompactIndexes(t *testing.T) {
	for _, s := range []string{"", "0", "0-999999", "0-1,3,5-7", "2147483647"} {
		if !validIndexes(s, nil) {
			t.Errorf("rejected valid %q", s)
		}
	}
	for _, s := range []string{"-1", "1-0", "1,1", "0-3,3-4", "1,0", "2147483648", "1-", "1,", "+1", "1:3", " 1", "1-2-3"} {
		if validIndexes(s, nil) {
			t.Errorf("accepted invalid %q", s)
		}
	}
}

func FuzzParseLists(f *testing.F) {
	f.Add(fixture(f, "jobs"))
	f.Add(fixture(f, "pods"))
	f.Add([]byte(`{"items":[]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ParseJobs(data, fixtureScope)
		_, _ = ParsePods(data, fixtureScope)
	})
}

func TestContradictoryJobOutcomeConditions(t *testing.T) {
	for _, success := range []string{"Complete", "SuccessCriteriaMet"} {
		for _, failure := range []string{"Failed", "FailureTarget"} {
			data := mutate(t, fixture(t, "jobs"), func(r map[string]any) {
				field(item(r, 0), "status")["conditions"] = []any{
					map[string]any{"type": success, "status": "True"},
					map[string]any{"type": failure, "status": "True"},
				}
			})
			if _, err := ParseJobs(data, fixtureScope); err == nil {
				t.Errorf("contradictory %s/%s accepted", success, failure)
			}
		}
	}
}
