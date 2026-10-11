# NRP / Nautilus support assessment

**Deferred October 10, 2026, at the user’s request.** All existing implementation,
tests, fixtures and documentation are preserved. This work is outside the active
Hummingbird release scope; the historical assessment below is retained as context.

Checked October 10, 2026. **Experimental CLI queries, parsers, bounded lists and HTTPS transport
implemented. Explicit exec-plugin sign-in is tested with a synthetic plugin;
an experimental namespace workspace is implemented and tested in real PTYs.
Namespace quota reads, Pod CPU/memory samples, exact quantity handling and
offline batch Job preparation and non-persisting server Job validation are also implemented.
Real OIDC/MFA and live NRP acceptance remain open.** This assessment and the
parser work do not establish complete NRP support. Hummingbird remains
Perch's working backend.

## Access and local evidence

UCSC lists Nautilus as available to students and researchers through CruzID Gold
and states a VPN requirement. Its service page distinguishes this container-based
platform from the Hummingbird service. Login eligibility alone does not prove
namespace access. [UCSC service description](https://its.ucsc.edu/services/research-computing/research-specific-computing-and-applications/national-research-platform-nautilus-hyperconverged-gpu-cluster/)

NRP's current instructions use institutional CILogon authentication, namespace
membership, `kubectl`, and the int128 `kubelogin` plugin. Students are directed to
ask their supervisor for namespace membership. NRP also requires reading its
policies and joining its support channel. These are external onboarding steps;
Perch cannot grant membership. [NRP access guide](https://nrp.ai/documentation/userdocs/start/getting-started/)

Local, offline checks returned:

| Check | Observed result | Meaning |
|---|---|---|
| `kubectl version --client -o json` | v1.33.9, darwin/arm64 | Existing client; server compatibility untested |
| `kubectl config get-contexts -o name` | Exit 0, no context names | No configured connection in the current effective configuration |
| Executable lookup | `/usr/local/bin/kubectl`; no `kubectl-oidc_login` on PATH | NRP's documented login plugin is not currently discoverable |

No account login, configuration download/overwrite, context switch, plugin
installation, Kubernetes API query, or workload submission occurred. These
checks do not establish whether the user already has an NRP portal account or
namespace membership. Preserve existing configuration when onboarding later.

## Product direction (proposal)

Add an explicit NRP workspace with the same warm palette, Chip, command input,
selection behavior and responsive inspector. Its tabs should be **Workloads,
Pods, Namespace, Connection**. Keep the Hummingbird workspace and its Slurm
vocabulary intact. A namespace can contain collaborators' workloads: label the
scope as namespace workloads rather than inventing a “my jobs” ownership filter.

Start with real read-only monitoring: Jobs and their Pods, controller conditions,
container status/restarts, bounded logs, namespace quota, and diagnostics for
pending/image-pull/OOM states. Keep actions unavailable until their complete
review, authorization, execution and recovery paths have been tested. The separate
`perch nrp …` command family now exposes read-only Job/Pod queries, without
changing today's numeric Slurm job-ID commands. The experimental namespace UI is
implemented; container logs are available through both the CLI and explicit
Pod/container selection in the workspace.

## Adapter boundaries and semantics

Current `query.Backend` returns `hummingbird.Refresh` and uses Slurm job, log-cursor
and resource types. It is not already a provider-neutral interface. Introduce an
NRP adapter with typed Job/Pod/container DTOs and a small presentation boundary;
do not force Pods into `domain.Job` or synthesize Slurm partitions/array IDs.

The workload reference should include provider, cluster identity, namespace,
kind, UID and display name. A recreated name must not inherit prior outcomes or
log cursors. Follow controller owner references when grouping Pods; display
container restarts separately from replacement Pods. A failed Pod may be retried;
use Job conditions to establish overall completion/failure. Indexed completion
and parallelism must retain their own semantics. [Kubernetes Jobs](https://kubernetes.io/docs/concepts/workloads/controllers/job/)

Use a bounded list-based adapter first, with explicit context and namespace on
every operation, request deadlines, output limits and per-resource success/stale
state. Never change the user's global current context. Later watches must handle
resource versions, expiration, relisting and cancellation; an expired watch is
not evidence that a workload completed. [API consistency and watches](https://kubernetes.io/docs/reference/using-api/api-concepts/)

Logs need a Pod/container selector and an explicit current/previous-container
choice. Retain the existing terminal-control sanitization and byte bounds, but
replace file-offset cursors with Kubernetes-specific retrieval semantics. Do not
promise durable history after objects disappear: node/container log retention
is limited and separate logging infrastructure may be needed. [Kubernetes logging](https://kubernetes.io/docs/concepts/cluster-administration/logging/)

CPU usage from the Metrics API is a rate over a measurement window; memory is a
working-set estimate. Neither is Slurm TotalCPU or MaxRSS. Keep timestamp, window,
unit and scope explicit, and report missing permissions/metrics as unavailable.
GPU utilization needs a separately verified source. [Resource metrics](https://kubernetes.io/docs/tasks/debug/debug-cluster/resource-metrics-pipeline/)

The experimental metrics implementation exposes an on-demand selected-Pod
CPU/memory snapshot through `perch nrp metrics POD` and the TUI's `u resources`
or `/metrics`. From a Job, choose a controller-owned Pod first. The view retains
sample age/window, and Escape restores the calling list or inspector. It discovers
a served Metrics API version: current upstream docs
describe `metrics.k8s.io/v1`, while older clusters expose `v1beta1`; do not infer
NRP's version or availability from the public documentation. It keeps forbidden,
missing API, missing sample and stale sample distinct from zero usage. Container
usage must not silently become a Job aggregate or a GPU-utilization reading.
[Metrics API reference](https://kubernetes.io/docs/reference/external-api/metrics.v1beta1/)

The reader preserves the existing Pod UID/replacement boundary. It brackets
the read with current Pod identity checks, rejects sample windows predating
Pod creation, and makes container-restart attribution explicit. CPU is average
core usage over the sample window; memory is an estimated working set at the
sample time. Decimal strings retain exact quantities and absent values stay
null. Samples older than twice their window or two minutes (whichever is larger)
are labelled stale. A failed reread retains the earlier sample with a failure
label and persistent sample age, including when the content is scrolled. App containers appear before sidecars; charts and GPU metrics are not
implemented. Real NRP authentication and API availability remain separate
acceptance gates; validation uses synthetic fixtures and local TLS servers.

## NRP-specific behavior to preserve

NRP recommends finite batch Jobs and prohibits idle/sleep batch workloads. Its
resource-request/limit and interactive-lifetime rules also affect any future
launcher; check current policy before implementing defaults. A smoke test must
perform bounded useful computation and exit, not reserve an idle GPU.
[Cluster policies](https://nrp.ai/documentation/userdocs/start/policies/)

GPU resources include type-specific keys and quota/priority restrictions. Do not
promise an accelerator because a hardware catalog lists it; discover namespace
permissions and quota, and make preemptible operation explicit. [GPU guide](https://nrp.ai/documentation/userdocs/running/gpu-pods/)

Keep authentication explicit. Background refresh must never launch an unexpected
browser login. Merely closing stdin does not prove an external credential plugin
will stay noninteractive; verify a supported noninteractive credential path and
separate reconnect flow. Credentials, kubeconfig contents and authentication
output must stay out of Perch diagnostics and MCP responses.

## Implementation and acceptance gates

1. Build deterministic fixtures for Job retries, multiple containers, OOM,
   unschedulable and image-pull states, indexed Jobs, deletion and name reuse.
   Implement the typed parser and list adapter with deterministic transport tests;
   test any credential commands separately before enabling the connection flow.
2. Add namespace-scoped CLI/demo flows; render and navigate Workloads → Pods →
   container logs at wide and 48×18 sizes. Verify explicit destination on every
   command and unsupported-capability behavior without contacting a cluster.
3. After onboarding is available, verify the server version, namespace/RBAC,
   Jobs/Pods/log access, resource quantities, quota and optional metrics. Capture
   deliberately scrubbed real fixtures. Missing access blocks this gate only;
   Hummingbird and offline development can continue.
4. Exercise login expiry/reconnect without hidden browser prompts, partial API
   failures, removed objects, stale replies and process/terminal cleanup.
5. Only then test an explicitly reviewed finite CPU Job with tiny resources,
   exact-ID cleanup and confirmed completion. GPU submission and interactive
   actions need their own capability and recovery evidence.

The experimental query subset below is available with its stated credential
limitations. Do not advertise complete NRP support before these gates pass. The remaining Hummingbird readiness audit is independent.


## Implemented foundation · October 10

`internal/nautilus` now parses individual `batch/v1` JobList and `v1` PodList
pages into separate workload, Pod and container models. Identity includes
cluster, namespace, kind and UID; grouping requires the Job controller UID
rather than a name label. Current container state, previous state, restart count
and replacement Pods remain separate. Job outcomes require terminal conditions;
`FailureTarget` and `SuccessCriteriaMet` remain nonterminal. Counts do not imply
completion, and deletion does not imply success.

The parser preserves missing values, compact index ranges, resource versions
and continuation tokens. It accepts unknown fields, validates required identity
and list structure, rejects duplicate JSON keys, contradictory outcomes and
malformed records, and bounds pages at 8 MiB / 10,000 items with a 64-level JSON
nesting limit. A rejected page returns an error rather than an empty-success
snapshot. User-visible prose is sanitized; unrelated configuration fields are
not exported. These models are not a durable workload history.

Evidence: deterministic synthetic fixtures in `testdata/nautilus`, race-tested
parser/identity tests, malformed-input tests, and a bounded fuzz run. Full
repository race tests, vet and formatting checks passed. Validation logs are
`output/validation/nautilus-parser-check.txt` and
`output/validation/nautilus-parser-fuzz.txt`.

The bounded list layer and HTTPS transport below now consume these parsers.
The scoped CLI, workspace UI, container logs and quota quantities are now
implemented in the later sections below. Metrics and live OIDC/reconnect
acceptance remain open. The explicit CLI credential acquisition path is described below. No NRP API was contacted, no real
fixtures were captured, and no runtime support is advertised by this change.


## Bounded namespace lists · October 10

`nautilus.Client` now reads complete Job and Pod collections through an injected
`PageReader`. Each request includes its cluster identity and namespace; the path
builder allows only the two namespaced list endpoints, validates namespace
syntax, and encodes continuation values without treating them as query options.
Jobs and Pods run independently, at most two reads concurrently per refresh.
Their resource versions and observation times are separate.

Pages are requested in groups of 250. Per resource, one list has a 20-second
context deadline, 32-MiB aggregate response cap, 20,000-object cap and 100-page
cap, in addition to parser page limits. Continuation tokens are limited to
16 KiB. A list succeeds only after its final page, with a consistent resource
version and no duplicate UID/name. Empty intermediate pages are supported.
Expired pages, token loops, version changes and read failures discard the
incomplete collection; a later refresh starts without a continuation token.
These choices follow [Kubernetes list consistency](https://kubernetes.io/docs/reference/using-api/api-concepts/).

Results distinguish an empty successful collection from a failed query. Jobs
can succeed while Pods fail, or vice versa. API status codes classify login,
permission, expired-list and temporary service errors; arbitrary server/plugin
messages are not copied into errors. This layer does not itself store old data:
the eventual UI coordinator must retain the previous successful collection and
mark it stale on a failed result.

Evidence: `internal/nautilus/client_test.go` exercises fake page readers, actual
request-path encoding, multi-page results, partial resource errors, limits,
expired-list recovery, cancellation and error-message redaction. `make check`
and the fresh targeted behavior run passed; logs are
`output/validation/nautilus-list-check.txt` and
`output/validation/nautilus-list-behavior.txt`.

The list layer is **not a kubectl runner**. The reader contract requires bounded,
cancellable reads that cannot launch interactive login. The HTTPS implementation
below now meets that transport boundary. Credential acquisition/fake-command
acceptance, UI stale-state handling and live connection remain open. The kubelogin documentation describes browser-based flows even
when its Kubernetes exec example uses noninteractive mode; setting that mode or
closing stdin is insufficient evidence of browser-free polling.
[Plugin setup](https://github.com/int128/kubelogin/blob/master/docs/setup.md),
[authentication flow flags](https://github.com/int128/kubelogin/blob/master/docs/usage.md).
No credential setup, cluster query or workload submission was performed.

## Credential-snapshot HTTPS transport · October 10

`nautilus.HTTPSReader` now implements the production `PageReader` contract with
Go's HTTP client. It receives one explicit cluster/namespace scope, endpoint,
trust configuration and in-memory credentials. Refreshes never read kubeconfig,
execute a plugin, open a browser, prompt or write a credential cache. The future
connection flow must obtain a credential snapshot explicitly, then replace the
reader on reconnect. Known expiration prevents requests; an HTTP 401 asks for
sign-in again. This uses Kubernetes' supported bearer-token/client-certificate
mechanisms, without embedding kubelogin's browser flow in polling.
[Kubernetes authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/)

The transport verifies TLS trust and hostname (TLS 1.2 minimum), using supplied
CA material or system roots. Bearer and PEM client-certificate credentials are
supported separately. It rejects plaintext endpoints, URL credentials/query
parameters, ambiguous path prefixes and invalid header values. An optional
reverse-proxy prefix is preserved. Requests must match both the reader's cluster
identity and namespace; only the two existing list routes are available. Normal
HTTPS proxy environment configuration is respected. There is no insecure-TLS
option or automatic fallback.

Each page has a ten-second deadline within the list's existing twenty-second
budget, 64-KiB response-header bound and 8-MiB body bound. Identity encoding is
requested; unexpected compressed responses fail rather than bypassing those
bounds. Redirects are rejected, even on the same host; credentials are never
forwarded to a login page or alternate endpoint. Error bodies and URL/transport
error strings are discarded, with stable auth/permission/expiry/TLS/timeout/
service classifications. Partial bodies are not returned. Closing a session
cancels its requests and closes idle sockets. Configuration and reader values
redact their material in ordinary formatting and JSON.

Evidence: `internal/nautilus/http_test.go` sends real HTTPS requests to local
synthetic API servers. It covers two-resource pagination through a proxy prefix,
opaque continuation encoding, scope mismatch, bearer and verified client-cert
authentication, untrusted/wrong-host certificates, redirects to a second server,
status/error redaction, partial permission failure, exact/oversize/chunked body
limits, encoding rejection, deadline/cancellation during headers or body reads,
close/reuse rejection and known credential expiry. These are actual socket/TLS
checks, not injected page-reader results; they are **not NRP server acceptance**.

No user kubeconfig, real token, login plugin, API server or workload was accessed.
The missing connection flow must still load a selected context without modifying
the current context, obtain credentials only through explicit user interaction,
and handle unsupported kubeconfig features honestly (including exec/auth-provider
variants, impersonation, token files and custom TLS-server-name/proxy fields).
It must also derive cluster identity consistently rather than using a mutable
context alias. The CLI subset below now exists; the interactive workspace is still open.

## Experimental scoped CLI · October 10

`perch nrp jobs`, `job NAME`, `pods`, and `pod NAME` now provide plain-text and
`--json` output. `--demo` uses embedded synthetic fixtures in namespace `research`
and never invokes kubectl or reads configuration. Demo connection flags are
rejected rather than silently ignored. Live commands require both `--context`
and `--namespace`, accept an optional `--kubeconfig`, and never query all
namespaces. The command family is separate from Hummingbird and does not load
its saved host or appearance preferences.

`kubectl config view --context=NAME --minify --flatten --raw --output=json`
loads only the selected connection. The five-second process is bounded to an
8-MiB captured response; raw output/errors never reach diagnostics or the user.
Its configuration-only implementation does not invoke credential plugins.
[Command reference](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_config/kubectl_config_view/),
[kubectl implementation](https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/config/view.go).
The parser verifies the selected context and its cluster/user references,
rejects duplicate JSON keys and unsupported connection/auth settings, and
constructs the immutable HTTPS reader. Explicit namespace overrides any stored
default without writing it back. Cluster identity hashes the configured endpoint
and trust bytes; alias renaming preserves it, while endpoint/trust changes start
a new identity scope. This is a connection identity, not an independently probed
Kubernetes cluster UID.

By default only static token and client-certificate configurations work.
Exec/OIDC entries require the explicit `--login` path below. Legacy auth-provider
entries, token files, impersonation, custom TLS-server-name, explicit kubeconfig
proxy settings and insecure TLS are rejected rather than silently changing their
meaning. Real NRP OIDC/MFA acceptance remains open.

JSON schema version 1 records source (`synthetic-demo` or `kubernetes-api`),
provider, explicit scope, resource version and observation time. Job outcomes
come from conditions, with Pod counts and indexed completion ranges separated.
Container current/previous state and missing values remain distinct. Pod list
status details expose reported scheduling/image-pull/current exit problems;
a previous failure is not presented as a current failure. Missing names return
failure with no success payload; disappearance never establishes completion.
No history or log durability is promised.

Evidence: unit/race tests plus `tools/test_nrp.py` run the **built executable**
against a local TLS API through both a fake config command and the installed
kubectl. Checks prove explicit namespace routing, Job/Pod inspection, permission
error redaction, demo isolation, unchanged configuration bytes and refusal to
execute a configured credential-plugin sentinel. The sentinel test makes no API
request. Temporary certificates, keys, config and homes are removed afterward.
Logs: `output/validation/nrp-cli-{unit,process,check}.txt`. This did not read the
user's real kubeconfig, run a real login plugin or contact NRP.

## Explicit exec-plugin sign-in · October 10

`perch nrp jobs --context NAME --namespace NAME --login` now permits one
foreground invocation of the selected kubeconfig's credential plugin. This flag
requires terminal stdin and is incompatible with `--demo`. Sign-in defaults to a
2m30s total command budget (overridable with `--timeout`); the plugin itself has a
two-minute ceiling. Configuration is still read without plugin execution first,
and endpoint/trust validation happens before launching sign-in. Static contexts
do not invoke a plugin, even with the flag.

The implementation follows Kubernetes' ExecCredential v1/v1beta1 protocol,
including matching response API version, stdin `Never`/`IfAvailable`/`Always`,
`KUBERNETES_EXEC_INFO`, configured args/env and optional cluster information plus
the reserved cluster exec extension. Arguments are passed directly, with no
shell. Relative plugin paths require an explicit kubeconfig when their origin
cannot otherwise be known. Unknown versions/modes, duplicate/reserved environment
variables and mixed credential methods are rejected. No plugin is installed by
Perch. [Exec credential protocol](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#input-and-output-formats)

Credential stdout is captured privately, with an 8-MiB limit that cancels the
process immediately if exceeded. Foreground instructions on stderr are streamed
to the requesting terminal, capped at 32 KiB; they may contain login URLs/codes
and are never sent through JSON/MCP or diagnostics. Returned tokens or client
certificates populate the in-memory HTTPS snapshot; known expiry is enforced.
Malformed, failed, expired or cancelled sign-in cannot start an API query. The
plugin can manage its own normal credential cache; Perch does not save returned
credentials or modify kubeconfig.

API reads never rerun the plugin, including after 401, timeout or expiration.
The caller must explicitly request another login. This separates the user's
foreground sign-in intent from later polling. It does not assert that
`interactiveMode: Never` alone prevents a plugin from opening a browser: the
explicit `--login` request is the authorization boundary for plugin execution.

Evidence: real helper processes test protocol fields, stdin modes, argument/env
handling, secret-output separation, malformed/version-mismatched responses,
known expiry, cancellation, oversized output and a single-login-then-401 flow.
The built-binary acceptance harness adds a PTY for login and verifies stdout JSON
is clean, terminal modes are preserved, noninteractive login is refused, no API
request follows expired credentials, and a normal query never launches the
plugin. All helpers, kubeconfigs, certificates and API data are synthetic.
The CLI saves terminal modes before invoking the plugin and restores them before
any API read, including on error or cancellation. The PTY helper deliberately
disables echo and canonical input; success, expired credentials, process failure
and timeout all restore the prior modes. The comparison excludes only Darwin's
transient PENDIN input-retype flag, as the existing TUI acceptance tests do.
Logs: `output/validation/nrp-login-{process,check-targeted,check,mcp}.txt`.
Real int128 kubelogin, browser/MFA, namespace access and live NRP behavior still
need acceptance; none was attempted against the user's account.

## Experimental namespace workspace · October 10

`perch nrp ui --demo` presents Jobs, Pods, Namespace and Connection tabs in the
warm Perch palette with rounded panels and a persistent command input. Live use
requires the same explicit context and namespace as queries; add `--login` for
exec credentials. This workspace is read-only and does not use Hummingbird
settings or infer ownership from labels. Quota and Pod metrics are described in their later sections; Chip and workload
actions remain follow-on work. Container log navigation is described below.

Enter opens details; Esc returns. Jobs show controller conditions, indexed-job
ranges and Pods matched by controller UID. Pod inspection separates current
and previous container states, including init, sidecar and ephemeral containers.
Filters and selections survive tab switches. Selection uses object UID, so a
same-name replacement dismisses the old inspector rather than adopting it.
Keyboard, mouse, reverse tabs, detail scrolling and resize recovery are covered
from 48×18 to 120×38. Hover underlines tabs and softly highlights unselected
workload/Pod/container rows without changing selection or reading logs. Supported
terminals receive a hand cursor for those targets and a text cursor over input.
Keyboard input, focus loss and resize clear hover; exit and foreground login
restore the pointer. Unknown terminals and tmux/screen retain visual feedback
without pointer escapes. `--no-mouse` disables capture and hover. `/help` lists
commands. Native pointer appearance remains an acceptance check.

Footer keyboard hints are also clickable: inspect an object, open its Pod and
container choices, switch current/previous logs, read again, return and quit.
Connection exposes the explicit `c connect` action. Command-input focus changes
the footer to Enter/Escape/quit instead of advertising object actions. Returning
from inspection or the log choices preserves the original workload filter and
UID selection. Only complete visible hints are targets; hover never initiates
a read or sign-in. These controls share their shortcut geometry with Hummingbird.

One refresh coordinator reads both resources every 15 seconds, backing off to
120 seconds after failures. Each successful collection replaces only its own
snapshot; failed reads retain data with a stale label and Connection details.
A successful empty collection clears old rows. Sequence and scope checks reject
late replies, including requests cancelled for reconnect. Duplicate completed
replies are ignored. Manual refresh has a two-second cooldown.

An authentication failure stops scheduled reads. `c` or `/connect` suspends the
UI for an explicit foreground credential acquisition; failure retains the old
snapshot and offers retry. Successful reconnect closes the old transport and
refreshes the selected namespace. A changed cluster identity clears old data.
Credentials remain in the connection's private transport, not in UI models.

Validation: `make check`, `tools/test_nrp_ui.py`, and `tools/test_nrp.py --ui`.
The last harness uses a temporary local TLS server and synthetic config/plugin
with a real PTY. It verifies independent permission failures, retained rows,
401 handling with no implicit plugin invocation, explicit reconnect, a plugin
that changes terminal modes and fails, a subsequent successful retry, resumed
navigation and terminal restoration. No real kubeconfig is changed; no NRP
endpoint or account is used. Screenshots under `output/terminal/nrp-*` are
reviewed emulator renders and do not prove native Terminal.app rendering.

## Bounded container logs in the CLI · October 10

`perch nrp logs POD --container NAME` reads the current container's recent output;
`--previous` explicitly chooses the previous terminated instance. Both accept
the existing context/namespace/kubeconfig/login flags, `--json`, and `--demo`.
A container is always explicit, including app, init, sidecar or ephemeral roles.
The demo has synthetic current/previous output for started fixture containers;
missing or not-yet-started output returns an error rather than invented text.

The adapter follows the documented [Pod log API](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/#get-read-log):
`follow=false`, `tailLines=2000` and `limitBytes=262144`, with the named container
and explicit previous flag. The client also enforces the byte limit, including
chunked responses, and a 25-second log-operation budget. The server's byte limit
is approximate; a response exceeding Perch's hard cap is rejected. Standard
output/error remain interleaved; no version-dependent stream selector is used.
There is no promise of complete logs, rotated-log durability or live streaming.

Typed requests allow only namespace Pod GET and Pod log GET, alongside existing
Job/Pod lists. Endpoint/trust/authentication, cancellation, response encoding and
redirect restrictions share the verified HTTPS transport. Pod GET is capped at
1 MiB and reuses the existing identity/container parser. No credential plugin
runs during log reads, no script executes, and logs are not persisted by Perch.

The selected reference includes Pod UID. The client checks that UID before and
after the name-addressed log request; replacement/deletion, a missing selected
container, or a changed observed restart count discards the log payload. These
are observational checks, not an atomic Kubernetes transaction or proof that an
unreported restart could not occur. Errors expose neither raw API bodies nor
partial log text. A successful empty response remains distinct from failure.
Terminal control sequences are stripped before output. JSON schema 1 includes
the Pod reference, container, previous flag, time and requested bounds; logs
have no collection resource version, so that envelope field is omitted.

Validation: adapter/race tests cover current/previous options, wrong scope/UID,
replacement/restart during reading, missing/denied resources, successful empty
logs, invalid Pod responses, sanitization and declared/chunked oversized bodies.
`tools/test_nrp.py` verifies the rebuilt CLI through a local TLS server, including
plain/JSON output, explicit previous selection, no stdout on permission errors,
and network-free demo behavior. Evidence is in `output/validation/nrp-logs-*.txt`.
Real NRP access remains open; no NRP server was contacted. The subsequent TUI
log implementation is described below.

Live log reads require `get` permission on both `pods` (identity verification)
and `pods/log` (the log subresource). Namespace list permission alone does not
establish those capabilities. Perch does not request or change RBAC grants.


## Container logs in the workspace · October 10

`l` or `/logs` opens an explicit chooser: a Job offers Pods matched by controller
UID; a Pod starts at its container list. App, sidecar, init and ephemeral roles
are labeled. Enter reads the selected container. `p` switches current/previous
output, `r` or `/refresh` reads another bounded snapshot, and Esc walks back to
the original workload view and filter. Keyboard, mouse, scrolling, help and
resize share the workspace controls. The header identifies the mode and last
successful read time; it does not imply a live stream.

Log reads use the adapter's existing UID/restart checks and limits. They have
one active request per open view, cancellation and generation checks. Leaving
or changing the selection discards late responses. Mode changes clear old text
before requesting new output. A failed refresh of the same selection preserves
its snapshot and reading position, marks it stale and shows the error; initial
failure shows no invented output. No automatic log polling or disk cache is
added. Logs keep their own viewport, so backing out restores workload details.

A log-subresource authentication failure stops background namespace reads as
well; an unrelated namespace success cannot clear that sign-in requirement.
Explicit `c` reconnects and rereads an open log after obtaining credentials.
Changing cluster scope closes the old log view. Permission denial remains a
read failure, distinct from an authentication failure.

Validation includes race-tested model behavior, real-PTY demo traversal, and
the built workspace against a local TLS server. The latter verifies denied
log access with retained text, current/previous isolation, 401 handling without
implicit login, explicit reconnect and a fresh read of the same selection.
Reviewed wide/compact and stale/reconnected screenshots are emulator renders;
native Terminal and real NRP credentials, permissions and logs remain unverified.
Evidence: `output/validation/nrp-log-ui-{check,pty,https,regression}.txt`.

## Namespace quotas · October 10

`perch nrp quotas` reads the explicitly selected namespace's `v1`
ResourceQuota objects. `--demo` includes synthetic examples; `--json` preserves
object identity, scope selectors, configured limits, reported hard limits,
reported usage and exact remaining quantities. The Namespace tab (`3` or
`/quotas`) displays these observations with compact layouts and keyboard/mouse
scrolling. It returns to the same filtered workload list with Escape.

Each quota is independent. Priority-class and other selectors are shown, and
limits across overlapping quotas are never added together. Quota usage is not
live utilization or free hardware. An empty list means no ResourceQuota objects
were reported, not that all admission policies are absent. These semantics follow
[Kubernetes resource quotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/).

The adapter uses the official `k8s.io/apimachinery/pkg/api/resource` quantity
parser and exact comparisons/subtraction. Raw API quantities remain in JSON;
the UI renders known CPU quantities in cores and exact binary-unit multiples
for memory/storage. Missing usage stays unknown. Exhausted, exceeded and
incomplete rows are distinct. If configured and reported limits disagree,
remaining headroom is withheld and the row is marked updating. Untrusted
quantity strings are bounded to 128 characters and decimal exponents ±30
before arithmetic; out-of-envelope or malformed pages fail rather than appear
as a zero-usage success.

Jobs, Pods and quotas have independent bounded list results, versions and
observation times. Failed quota reads retain a visibly stale prior snapshot;
a successful empty read removes it. Missing quota RBAC does not erase Jobs or
Pods. Any authentication failure suspends background polling until explicit
sign-in. The connection view exposes quota errors and observation metadata.
No permission, kubeconfig or remote policy is changed.

Evidence: `internal/nautilus/quota_test.go`, HTTPS/client tests, CLI JSON/text
checks, UI stale/auth/scroll tests and `tools/test_nrp_quota.py`. The synthetic
HTTPS process suite also exercises quota permission loss and reconnect with the
third independent list. Real NRP namespace access, RBAC, quotas, metrics and
workload actions remain unverified; these fixtures are not live cluster data.


## Offline workload preparation

`perch nrp prepare` builds a local `batch/v1` Job review and export. It makes no
API request, runs no supplied command, reads no kubeconfig and does not submit.
It can therefore be used before onboarding. The live submission gate above
remains open; a schema-valid draft is not evidence of admission or execution.

```sh
perch nrp prepare --name analysis --namespace research \
  --image python:3.13-slim --cpu 1 --memory 512Mi \
  --ephemeral-storage 1Gi --time 5m \
  -- python -c 'print(sum(i*i for i in range(1000000)))'
```

All flags go before `--`. Everything after it is container argv, including flags
named `--json` or `--namespace`. Commands are not implicitly run in a shell.
For shell syntax, explicitly supply `bash -lc '...'` or another shell available
in the image. Dollar signs are escaped in the manifest to prevent Kubernetes
from expanding `$(NAME)` before the process receives its original argv. The
review and full-draft JSON retain the original arguments. Terminal control
characters in command arguments are quoted rather than executed by the review.
[Kubernetes command/argument semantics](https://kubernetes.io/docs/reference/kubernetes-api/core/pod-v1/)

Add `--manifest` before `--` to write only Job JSON to stdout. Redirect it to a
file for review or later cluster validation. `--json` instead includes
`schema_version: 1`, `provider: nautilus`, `source: local-draft`, the original
command, review notes and the manifest. These modes are mutually exclusive.
Human output wraps at the attached terminal's width; JSON is unaffected.

### Resources, scheduling and lifetime

The required flags are `--name`, `--namespace`, `--image`, `--cpu`, `--memory`,
`--ephemeral-storage` and `--time`. CPU accepts cores to millicore precision
(for example `0.5` or `500m`); memory and local disk require explicit units such
as `512Mi` or `2Gi`. Time accepts whole-second Go durations such as `30m` or
`2h`. The deadline includes Pod startup and scheduling delay, not just active
computation.

Requests equal limits, following NRP's allocation policy. The output requests
one completion and one Pod at a time, `restartPolicy: Never`, and zero failure
retries. Kubernetes can still create replacement Pods and does not guarantee
exactly-once execution; workloads that write results should be safe to repeat.
The manifest sets no automatic cleanup TTL. The container does not receive an
automatically mounted Kubernetes service-account token.
[NRP policies](https://nrp.ai/documentation/userdocs/start/policies/),
[Kubernetes Job semantics](https://kubernetes.io/docs/concepts/workloads/controllers/job/)

CPU-only drafts exclude nodes labeled with NVIDIA hardware, following NRP's
CPU guidance. `--opportunistic` explicitly sets the preemptible priority class;
it is not enabled silently. GPU drafts require `--gpus N --gpu-resource KEY`,
for example `--gpus 1 --gpu-resource nvidia.com/gpu`. The key is validated as
an NVIDIA qualified resource name, not as an available or authorized device.
Type-specific keys, quota, drivers, image architecture and any needed placement
or tolerations require cluster-specific review. This builder does not select
those automatically. Current NRP policy prohibits idle batch reservations;
supply a finite computation rather than a sleeping placeholder. Perch does not
claim to infer termination or resource efficiency from arbitrary code.
[CPU guidance](https://nrp.ai/documentation/userdocs/running/cpu-only/),
[GPU resource guide](https://nrp.ai/documentation/userdocs/running/gpu-pods/)

### Files and results

`--pvc research-data --mount-path /data` attaches an existing PVC in the selected
namespace. Perch does not create it or verify its capacity, access mode or
permissions offline. `--workdir /data/project` is a container directory, not a
path to upload from your Mac. Code must already be in the image or mounted
storage. Without a PVC, the review calls out that container files are disposable.
Local disk limits cover ephemeral usage and do not make it persistent.

This first preparer covers a single-container batch Job. Private image pull
secrets, environment/secret injection, volume creation, init containers, indexed
Jobs, distributed workloads and live reviewed submission remain future work.
The generated JSON can be independently inspected or extended outside Perch.

### Verification

`internal/nautilus/prepare_test.go` covers resources, placement, identity bounds,
PVC consistency, lifetime, argument preservation and invalid drafts.
`cmd/perch/nrp_prepare_test.go` checks CLI boundaries, safe review and export.
`tools/test_nrp_prepare.py` tests the built binary with trap executables and a
missing kubeconfig; optional `--schema` validates CPU, PVC and GPU manifests
against a separately downloaded official Kubernetes Swagger schema and tests
that a deliberately wrong deadline type is rejected.

The local schema check uses Kubernetes v1.33.9's published API schema; it is not
a claim about NRP's current server version. Evidence is recorded in
`output/validation/nrp-prepare-schema.json`. No Job was submitted.

## Server Job validation

`perch nrp validate job.json --context NAME --namespace NAME` reads a regular
JSON file of at most 256 KiB. It requires one named `batch/v1` Job whose namespace
matches the explicit destination; lists, generated names, duplicate keys,
trailing documents and excessive nesting are rejected before configuration or
credentials are read. Export from `prepare --manifest`; the full `prepare --json`
review envelope is not a Job manifest. YAML is not accepted by this command.

The exact manifest bytes, including fields Perch's preparer does not generate,
are sent once to the bound namespace Job endpoint with `dryRun=All` and
`fieldValidation=Strict`. There is no fallback to real creation, no automatic
retry, and no redirect following. A successful response must identify the same
Job and namespace. The report records the input SHA-256 and observation time;
it is not a submission receipt. Credential expiry, TLS verification, cancellation,
bounded output and explicit foreground login use the existing connection path.

Kubernetes dry-run performs authorization, defaulting and admission without
persisting the object; side-effecting admission hooks must reject dry-run rather
than run their side effects. [Kubernetes API dry-run](https://kubernetes.io/docs/reference/using-api/api-concepts/#dry-run)

**Job admission is not Pod admission.** Creating the Job would cause its controller
to create Pods later, with their own policy and quota checks. Perch reads the
namespace ResourceQuotas independently and preserves each quota's scope and
unknown values. It does not reserve resources, add overlapping quotas together,
check image pulls or mounts, or promise scheduling. A quota-read failure does not
turn a successful Job dry-run into a rejection; the partial result remains
explicit. No workload is submitted by this command.

`--json` prints a versioned validation report even when admission is rejected or
unverified; those cases exit nonzero. HTTP 409 reports a name conflict, 400/422
report validation/admission rejection, and permission/authentication failures
remain distinct. Arbitrary API error bodies are excluded from diagnostics.
The report's `status` is `accepted`, `rejected` or `unverified`, with a
`next_step` for recovery. Explicit permission, name-conflict and admission
denials are rejected. Expired authentication, connection errors, cancellation,
missing endpoints and malformed replies leave admission unverified. The
compatibility field `job_dry_run_admitted: false` alone does not mean rejection.
Text reports use the same distinction and suggest signing in, checking scope,
renaming or correcting the manifest, or retrying the dry-run as appropriate.
The server may default or mutate the dry-run object; this command does not export
that returned object or authorize its eventual submission.

Go/TLS tests cover exact POST scope and bytes, independent denied quota reads,
cross-namespace refusal, expired/closed sessions, redirects without following,
failed requests without retry and invalid/oversized responses. Built-binary
acceptance extends `tools/test_nrp.py` with local synthetic HTTPS validation,
denial/conflict/admission failures and manifest hashes. Existing list, log,
metrics and credential checks still run. No live NRP validation, namespace access,
Pod admission, workload execution or submission is established by these tests.
