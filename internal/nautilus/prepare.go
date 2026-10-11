package nautilus

// Local workload preparation deliberately has no Client or credential path.
import (
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
)

type PrepareOptions struct {
	Name, Namespace, Image, CPU, Memory, EphemeralStorage string
	GPUResource                                           string
	GPUs                                                  int
	Deadline                                              time.Duration
	Command                                               []string
	PVC, MountPath, WorkDir                               string
	Opportunistic                                         bool
}

// Draft is a locally validated plan, not an admitted or submitted workload.
type Draft struct {
	SchemaVersion int         `json:"schema_version"`
	Provider      string      `json:"provider"`
	Source        string      `json:"source"`
	Command       []string    `json:"command"`
	Notes         []string    `json:"notes"`
	Manifest      JobManifest `json:"manifest"`
}

type JobManifest struct {
	APIVersion string        `json:"apiVersion"`
	Kind       string        `json:"kind"`
	Metadata   DraftMetadata `json:"metadata"`
	Spec       DraftJobSpec  `json:"spec"`
}
type DraftMetadata struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}
type DraftJobSpec struct {
	Parallelism           int           `json:"parallelism"`
	Completions           int           `json:"completions"`
	BackoffLimit          int           `json:"backoffLimit"`
	ActiveDeadlineSeconds int64         `json:"activeDeadlineSeconds"`
	Template              DraftTemplate `json:"template"`
}
type DraftTemplate struct {
	Metadata DraftMetadata `json:"metadata"`
	Spec     DraftPodSpec  `json:"spec"`
}
type DraftPodSpec struct {
	RestartPolicy                string           `json:"restartPolicy"`
	AutomountServiceAccountToken bool             `json:"automountServiceAccountToken"`
	PriorityClassName            string           `json:"priorityClassName,omitempty"`
	Containers                   []DraftContainer `json:"containers"`
	Volumes                      []DraftVolume    `json:"volumes,omitempty"`
	Affinity                     map[string]any   `json:"affinity,omitempty"`
}
type DraftContainer struct {
	Name         string         `json:"name"`
	Image        string         `json:"image"`
	Command      []string       `json:"command"`
	Args         []string       `json:"args,omitempty"`
	WorkingDir   string         `json:"workingDir,omitempty"`
	Resources    DraftResources `json:"resources"`
	VolumeMounts []DraftMount   `json:"volumeMounts,omitempty"`
}
type DraftResources struct {
	Requests map[string]string `json:"requests"`
	Limits   map[string]string `json:"limits"`
}
type DraftMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
}
type DraftVolume struct {
	Name                  string     `json:"name"`
	PersistentVolumeClaim DraftClaim `json:"persistentVolumeClaim"`
}
type DraftClaim struct {
	ClaimName string `json:"claimName"`
}

var draftCPU = regexp.MustCompile(`^(?:[0-9]+(?:\.[0-9]{1,3})?|[0-9]+m)$`)
var draftBytes = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?(?:Ki|Mi|Gi|Ti|Pi|K|M|G|T|P)$`)
var draftImage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]*$`)

func draftQuantity(value, field string, cpu bool) (string, error) {
	pattern := draftBytes
	if cpu {
		pattern = draftCPU
	}
	q, err := quotaQuantity(value)
	if err != nil || !pattern.MatchString(value) || q.Sign() <= 0 || q.Cmp(*resource.NewQuantity(math.MaxInt64, resource.DecimalSI)) > 0 {
		if cpu {
			return "", fmt.Errorf("%s must be positive CPU cores with at most 3 decimal places, or millicores (for example 500m)", field)
		}
		return "", fmt.Errorf("%s must be a positive quantity with an explicit unit, for example 512Mi or 2Gi", field)
	}
	return q.String(), nil
}

func draftPath(s string) bool {
	return len(s) <= 4096 && utf8.ValidString(s) && strings.HasPrefix(s, "/") && path.Clean(s) == s && strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }) < 0
}

func Prepare(o PrepareOptions) (Draft, error) {
	fail := func(s string) (Draft, error) { return Draft{}, fmt.Errorf("%s", s) }
	if len(o.Name) > 63 || len(validation.IsDNS1123Label(o.Name)) != 0 {
		return fail("--name must be a lowercase DNS label of at most 63 characters")
	}
	if len(validation.IsDNS1123Label(o.Namespace)) != 0 {
		return fail("--namespace must be an explicit lowercase DNS label")
	}
	if len(o.Image) > 1024 || !draftImage.MatchString(o.Image) {
		return fail("--image requires a container image reference without spaces or a URL scheme")
	}
	if strings.Contains(o.Image, "://") {
		return fail("--image must be a container image reference, not a URL")
	}
	if o.Deadline < time.Second || o.Deadline%time.Second != 0 {
		return fail("--time must be a positive whole-second duration, for example 30m or 2h")
	}
	cpu, err := draftQuantity(o.CPU, "--cpu", true)
	if err != nil {
		return Draft{}, err
	}
	memory, err := draftQuantity(o.Memory, "--memory", false)
	if err != nil {
		return Draft{}, err
	}
	scratch, err := draftQuantity(o.EphemeralStorage, "--ephemeral-storage", false)
	if err != nil {
		return Draft{}, err
	}
	if len(o.Command) == 0 || strings.TrimSpace(o.Command[0]) == "" || len(o.Command) > 1024 {
		return fail("supply a finite workload after --, for example -- python /data/train.py")
	}
	size := 0
	encoded := make([]string, len(o.Command))
	for i, arg := range o.Command {
		size += len(arg)
		if size > 65536 || !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			return fail("command must be valid UTF-8 without NUL and at most 64 KiB")
		}
		// Kubernetes expands $(NAME) and reduces $$. Escape every dollar to
		// deliver the original argv, including shell code, to the container.
		encoded[i] = strings.ReplaceAll(arg, "$", "$$")
	}
	if o.WorkDir != "" && !draftPath(o.WorkDir) {
		return fail("--workdir must be a clean absolute path inside the container")
	}
	if (o.PVC == "") != (o.MountPath == "") {
		return fail("use --pvc and --mount-path together")
	}
	if o.PVC != "" && (len(validation.IsDNS1123Subdomain(o.PVC)) != 0 || !draftPath(o.MountPath) || o.MountPath == "/") {
		return fail("use a valid existing PVC name and a clean absolute mount path other than /")
	}
	if o.GPUs < 0 || (o.GPUs == 0) != (o.GPUResource == "") {
		return fail("use a positive --gpus count and explicit --gpu-resource together")
	}
	if o.GPUResource != "" && (!strings.HasPrefix(o.GPUResource, "nvidia.com/") || len(validation.IsQualifiedName(o.GPUResource)) != 0) {
		return fail("--gpu-resource must be an explicit NVIDIA resource key, for example nvidia.com/gpu; availability is checked on the cluster")
	}
	requests := map[string]string{"cpu": cpu, "memory": memory, "ephemeral-storage": scratch}
	if o.GPUs > 0 {
		requests[o.GPUResource] = strconv.Itoa(o.GPUs)
	}
	limits := make(map[string]string, len(requests))
	for k, v := range requests {
		limits[k] = v
	}
	c := DraftContainer{Name: "workload", Image: o.Image, Command: encoded[:1], Args: encoded[1:], WorkingDir: o.WorkDir, Resources: DraftResources{Requests: requests, Limits: limits}}
	pod := DraftPodSpec{RestartPolicy: "Never", Containers: []DraftContainer{c}}
	notes := []string{
		"Local draft only: namespace access, quota, image availability, scheduling and admission have not been checked.",
		"Requests equal limits. Size them for your workload and check actual utilization after launch.",
		"One completion, one Pod at a time, no failure retries. Kubernetes Jobs do not guarantee exactly-once execution.",
		"The deadline includes time waiting for a Pod to start. Finished objects are not automatically deleted by this manifest.",
		"Use a finite computation; NRP batch Jobs must not reserve idle resources with sleep or equivalent.",
	}
	if o.Opportunistic {
		pod.PriorityClassName = "opportunistic"
		notes = append(notes, "Opportunistic priority: the workload can be preempted. Checkpoint results to persistent storage.")
	}
	if o.GPUs == 0 {
		pod.Affinity = map[string]any{"nodeAffinity": map[string]any{"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{"nodeSelectorTerms": []any{map[string]any{"matchExpressions": []any{map[string]any{"key": "feature.node.kubernetes.io/pci-10de.present", "operator": "NotIn", "values": []string{"true"}}}}}}}}
		notes = append(notes, "CPU-only placement excludes nodes labeled as having NVIDIA GPUs.")
	} else {
		notes = append(notes, "GPU resource keys, permissions, driver/image compatibility and architecture require cluster checks; requesting a key does not guarantee access.")
	}
	if o.PVC != "" {
		pod.Containers[0].VolumeMounts = []DraftMount{{Name: "data", MountPath: o.MountPath}}
		pod.Volumes = []DraftVolume{{Name: "data", PersistentVolumeClaim: DraftClaim{ClaimName: o.PVC}}}
		notes = append(notes, "The PVC must already exist in this namespace. Its capacity, access mode and mount permissions have not been checked.")
	} else {
		notes = append(notes, "No persistent volume: container files are disposable. Export results before deleting the Pod.")
	}
	labels := map[string]string{"app.kubernetes.io/managed-by": "perch"}
	return Draft{SchemaVersion: 1, Provider: "nautilus", Source: "local-draft", Command: append([]string(nil), o.Command...), Notes: notes, Manifest: JobManifest{
		APIVersion: "batch/v1", Kind: "Job", Metadata: DraftMetadata{Name: o.Name, Namespace: o.Namespace, Labels: labels},
		Spec: DraftJobSpec{Parallelism: 1, Completions: 1, BackoffLimit: 0, ActiveDeadlineSeconds: int64(o.Deadline / time.Second), Template: DraftTemplate{Metadata: DraftMetadata{Labels: labels}, Spec: pod}},
	}}, nil
}
