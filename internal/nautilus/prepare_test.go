package nautilus

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func draftExample() PrepareOptions {
	return PrepareOptions{Name: "analysis", Namespace: "research", Image: "python:3.13-slim", CPU: "0.5", Memory: "512Mi", EphemeralStorage: "1Gi", Deadline: 5 * time.Minute, Command: []string{"python", "-c", "print(42)"}}
}
func TestPrepareCPUAndStorageSemantics(t *testing.T) {
	o := draftExample()
	o.PVC, o.MountPath, o.WorkDir, o.Opportunistic = "research-data", "/data", "/data/project", true
	d, err := Prepare(o)
	if err != nil {
		t.Fatal(err)
	}
	j := d.Manifest
	p := j.Spec.Template.Spec
	c := p.Containers[0]
	if d.Source != "local-draft" || j.APIVersion != "batch/v1" || j.Kind != "Job" || j.Metadata.Namespace != o.Namespace || j.Metadata.Name != o.Name {
		t.Fatal(d)
	}
	if j.Spec.ActiveDeadlineSeconds != 300 || j.Spec.BackoffLimit != 0 || j.Spec.Parallelism != 1 || j.Spec.Completions != 1 || p.RestartPolicy != "Never" || p.AutomountServiceAccountToken {
		t.Fatal(j.Spec)
	}
	if !reflect.DeepEqual(c.Resources.Requests, c.Resources.Limits) || c.Resources.Requests["cpu"] != "500m" || c.Resources.Requests["ephemeral-storage"] != "1Gi" {
		t.Fatal(c.Resources)
	}
	if p.Affinity == nil || p.PriorityClassName != "opportunistic" || c.WorkingDir != "/data/project" || p.Volumes[0].PersistentVolumeClaim.ClaimName != o.PVC || c.VolumeMounts[0].MountPath != "/data" {
		t.Fatal(p)
	}
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`"backoffLimit":0`, `"automountServiceAccountToken":false`, `"operator":"NotIn"`, `"values":["true"]`} {
		if !strings.Contains(string(data), required) {
			t.Fatal(string(data))
		}
	}
	for _, unwanted := range []string{"ttlSecondsAfterFinished", "hostPath", "serviceAccountName", "status"} {
		if strings.Contains(string(data), unwanted) {
			t.Fatal(unwanted)
		}
	}
}
func TestPrepareGPUAndLiteralArguments(t *testing.T) {
	o := draftExample()
	o.GPUs, o.GPUResource = 2, "nvidia.com/rtxa6000"
	o.Command = []string{"bash", "-c", "printf '%s' \"$(echo hi) $$ $HOME\"", "", "--namespace", "other", "a\nb", "\x1b]52;c;secret\a"}
	d, err := Prepare(o)
	if err != nil {
		t.Fatal(err)
	}
	p := d.Manifest.Spec.Template.Spec
	if p.Affinity != nil || p.PriorityClassName != "" || len(p.Volumes) != 0 {
		t.Fatal(p)
	}
	c := p.Containers[0]
	if c.Resources.Requests[o.GPUResource] != "2" || c.Resources.Limits[o.GPUResource] != "2" {
		t.Fatal(c.Resources)
	}
	encoded := append(append([]string{}, c.Command...), c.Args...)
	for i, s := range encoded {
		if strings.ReplaceAll(s, "$$", "$") != o.Command[i] {
			t.Fatalf("argument %d changed: %q", i, s)
		}
	}
	if !reflect.DeepEqual(d.Command, o.Command) {
		t.Fatal("review argv differs from intended argv")
	}
	o.Command[0] = "changed"
	if d.Command[0] != "bash" {
		t.Fatal("draft shares mutable input")
	}
}
func TestPrepareRejectsInvalidDrafts(t *testing.T) {
	cases := map[string]func(*PrepareOptions){
		"namespace missing":       func(o *PrepareOptions) { o.Namespace = "" },
		"namespace dots":          func(o *PrepareOptions) { o.Namespace = "bad.namespace" },
		"name uppercase":          func(o *PrepareOptions) { o.Name = "Analysis" },
		"name long":               func(o *PrepareOptions) { o.Name = strings.Repeat("a", 64) },
		"image empty":             func(o *PrepareOptions) { o.Image = "" },
		"image escape":            func(o *PrepareOptions) { o.Image = "python\x1b[2J" },
		"image url":               func(o *PrepareOptions) { o.Image = "https://registry/image" },
		"zero cpu":                func(o *PrepareOptions) { o.CPU = "0" },
		"cpu precision":           func(o *PrepareOptions) { o.CPU = "0.0001" },
		"cpu negative":            func(o *PrepareOptions) { o.CPU = "-2" },
		"memory unitless":         func(o *PrepareOptions) { o.Memory = "512" },
		"memory fractional bytes": func(o *PrepareOptions) { o.Memory = "500m" },
		"memory huge":             func(o *PrepareOptions) { o.Memory = strings.Repeat("9", 130) + "Gi" },
		"zero memory":             func(o *PrepareOptions) { o.Memory = "0Gi" },
		"scratch missing":         func(o *PrepareOptions) { o.EphemeralStorage = "" },
		"deadline zero":           func(o *PrepareOptions) { o.Deadline = 0 },
		"deadline fractional":     func(o *PrepareOptions) { o.Deadline = 1500 * time.Millisecond },
		"no command":              func(o *PrepareOptions) { o.Command = nil },
		"empty executable":        func(o *PrepareOptions) { o.Command = []string{""} },
		"nul argv":                func(o *PrepareOptions) { o.Command = []string{"echo", "\x00"} },
		"invalid utf8":            func(o *PrepareOptions) { o.Command = []string{"echo", string([]byte{255})} },
		"large argv":              func(o *PrepareOptions) { o.Command = []string{"echo", strings.Repeat("a", 65536)} },
		"workdir relative":        func(o *PrepareOptions) { o.WorkDir = "data" },
		"workdir traversal":       func(o *PrepareOptions) { o.WorkDir = "/data/../tmp" },
		"workdir control":         func(o *PrepareOptions) { o.WorkDir = "/data\tbad" },
		"claim no mount":          func(o *PrepareOptions) { o.PVC = "data" },
		"mount no claim":          func(o *PrepareOptions) { o.MountPath = "/data" },
		"mount root":              func(o *PrepareOptions) { o.PVC = "data"; o.MountPath = "/" },
		"claim invalid":           func(o *PrepareOptions) { o.PVC = "../data"; o.MountPath = "/data" },
		"gpu no resource":         func(o *PrepareOptions) { o.GPUs = 1 },
		"resource no gpu":         func(o *PrepareOptions) { o.GPUResource = "nvidia.com/gpu" },
		"negative gpu":            func(o *PrepareOptions) { o.GPUs = -1 },
		"gpu collision":           func(o *PrepareOptions) { o.GPUs = 1; o.GPUResource = "cpu" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			o := draftExample()
			change(&o)
			d, err := Prepare(o)
			if err == nil || d.Source != "" {
				t.Fatalf("invalid draft accepted: %+v %v", d, err)
			}
		})
	}
}
