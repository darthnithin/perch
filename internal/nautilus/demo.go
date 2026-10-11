package nautilus

import (
	"context"
	"encoding/json"
	"fmt"
	fixtures "perch/testdata/nautilus"
	"time"
)

func NewDemoClient() *Client {
	c, err := NewClient(Scope{Cluster: "nrp-demo", Namespace: "research"}, demoReader{})
	if err != nil {
		panic(err)
	}
	return c
}

func (d demoReader) ReadPod(ctx context.Context, r PodRequest) ([]byte, error) {
	data, err := d.ReadPage(ctx, ListRequest{Scope: r.Scope, Resource: Pods, Limit: 250})
	if err != nil {
		return nil, err
	}
	if _, err := r.Path(); err != nil {
		return nil, err
	}
	var list struct{ Items []map[string]any }
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	for _, pod := range list.Items {
		metadata := pod["metadata"].(map[string]any)
		if metadata["name"] == r.Name {
			pod["kind"], pod["apiVersion"] = "Pod", "v1"
			metadata["resourceVersion"] = "synthetic-pod-version"
			return json.Marshal(pod)
		}
	}
	return nil, queryError(NotFound, "")
}

func (d demoReader) ReadContainerLog(ctx context.Context, r LogRequest) ([]byte, error) {
	if _, err := r.Path(); err != nil {
		return nil, err
	}
	data, err := d.ReadPod(ctx, PodRequest{r.Pod.Scope, r.Pod.Name})
	if err != nil {
		return nil, err
	}
	pod, err := parsePod(data, r.Pod.Scope)
	if err != nil {
		return nil, err
	}
	c, ok := findContainer(pod, r.Container)
	if !ok || r.Previous && c.Last.Phase != "Terminated" || !r.Previous && c.State.Phase != "Running" && c.State.Phase != "Terminated" {
		return nil, queryError(ReadFailed, "")
	}
	if r.Previous {
		return []byte(fmt.Sprintf("[demo] Previous instance of %s\nProcessing batch 17\nContainer stopped; inspect its recorded termination state.\n", c.Name)), nil
	}
	return []byte(fmt.Sprintf("[demo] Current instance of %s\nLoaded configuration\nCheckpoint saved\n", c.Name)), nil
}
func (c *Client) Scope() Scope { return c.scope }

type demoReader struct{}

func (demoReader) MetricsVersion(context.Context) (string, error) { return "v1beta1", nil }

func (d demoReader) ReadPodMetrics(ctx context.Context, r MetricsRequest) ([]byte, error) {
	if _, err := r.Path(); err != nil {
		return nil, err
	}
	data, err := d.ReadPod(ctx, PodRequest{r.Pod.Scope, r.Pod.Name})
	if err != nil {
		return nil, err
	}
	pod, err := parsePod(data, r.Pod.Scope)
	if err != nil {
		return nil, err
	}
	containers := []map[string]any{}
	for _, c := range pod.Containers {
		if c.State.Phase == "Running" {
			containers = append(containers, map[string]any{"name": c.Name, "usage": map[string]string{"cpu": "215000000n", "memory": "149504Ki"}})
		}
	}
	return json.Marshal(map[string]any{"kind": "PodMetrics", "apiVersion": "metrics.k8s.io/" + r.Version, "metadata": map[string]string{"name": pod.Name, "namespace": pod.Namespace}, "timestamp": time.Now().UTC().Add(-10 * time.Second), "window": "30s", "containers": containers})
}

func (demoReader) ReadPage(ctx context.Context, r ListRequest) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Scope != (Scope{Cluster: "nrp-demo", Namespace: "research"}) || r.Continue != "" {
		return nil, queryError(Invalid, "Invalid demo namespace request")
	}
	switch r.Resource {
	case Jobs:
		return append([]byte(nil), fixtures.Jobs...), nil
	case Pods:
		return append([]byte(nil), fixtures.Pods...), nil
	case Quotas:
		return append([]byte(nil), fixtures.Quotas...), nil
	}
	return nil, queryError(Invalid, "Unsupported demo resource")
}
