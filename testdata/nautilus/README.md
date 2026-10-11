# Nautilus parser fixtures

These are **synthetic Kubernetes API-shaped fixtures**, created October 10,
2026. They were not captured from NRP and do not prove its Kubernetes version,
configuration, namespace access or feature availability. Names, UIDs, timestamps,
images and node names are invented. No workload was submitted.

- `jobs.json`: retried active Job, a compact million-index Job, success/failure
  targets awaiting termination, terminal complete/failed Jobs, a reused name,
  and an observed suspended Job. Pod counts remain distinct from index progress.
- `pods.json`: failed OOM Pod plus replacement, current/previous container state,
  app/init/restartable-init/ephemeral containers, image-pull and scheduling
  problems, and a Pod owned by a deleted Job despite a matching name label.

Fields are selected from the official [Job API](https://kubernetes.io/docs/reference/kubernetes-api/batch/job-v1/)
and [Pod API](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/).
Terminal-state interpretation follows [Job conditions](https://kubernetes.io/docs/concepts/workloads/controllers/job/).
Grouping follows controller references and [object UIDs](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/).
The optional fields deliberately exercise multiple API capabilities; their
presence here is not evidence of support by a particular cluster.

`internal/nautilus/parse_test.go` also mutates these fixtures to exercise wrong
namespaces, duplicate identities, conflicting conditions/states, malformed
counts/timestamps/index ranges, unknown future fields and control sequences.
List pagination, envelope errors, input limits and omitted values have separate
checks. Fuzzing feeds both parsers with bounded arbitrary JSON.


`resourcequotas.json` is synthetic ResourceQuota API data: CPU millicores, binary
memory quantities, exhausted GPU requests, overlapping priority-class scopes,
missing usage and a configured limit awaiting controller reconciliation. It is
not a statement of any real namespace's allowance or current hardware capacity.
