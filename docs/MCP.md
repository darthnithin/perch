# Perch for agents

Perch includes a local stdio MCP server built with the [official Go SDK](https://github.com/modelcontextprotocol/go-sdk). It connects to Hummingbird using the same system SSH transport and normalized queries as the CLI. Nothing is installed on the cluster.

```sh
make build
./bin/perch mcp --demo
./bin/perch mcp --host your-cruzid@hb.ucsc.edu
```

The command waits for an MCP client on stdin; a quiet terminal is expected. Do not use it as an interactive shell. Stdout carries protocol messages only. On EOF the server exits and closes its private SSH master. `--timeout 45s` sets each live tool's deadline, including time spent waiting behind another query. `--local` is available where local cluster commands are permitted.

## Connect a client

Use an absolute path to the executable. Clients that use an `mcpServers` JSON configuration can adapt [the example configuration](../examples/mcp-config.json):

```json
{
  "mcpServers": {
    "perch": {
      "command": "/absolute/path/to/perch/bin/perch",
      "args": ["mcp", "--host", "your-cruzid@hb.ucsc.edu"]
    }
  }
}
```

For an offline walkthrough, use `"args": ["mcp", "--demo"]`. Setup syntax differs between clients; this file is a configuration example, not an automatic installation. Perch does not modify client settings.

Live tools require working noninteractive SSH credentials. Guides and discovery work even when the cluster is unavailable. Authentication problems appear as tool errors; Perch does not request or accept passwords through MCP. The TUI's interactive SSH sign-in is local to that TUI process, so it does not establish credentials for a separately running MCP process. Configure your system SSH agent/key access for repeated CLI or MCP use.

## Tools

| Tool | Purpose |
| --- | --- |
| `perch_jobs` | Personal current queue and pending reasons |
| `perch_history` | Seven-day accounting records |
| `perch_nodes` | Visible per-node CPU/memory/GPU assignments, system reservation, state flags and raw GRES; on demand |
| `perch_priority` | Personal pending allocation/partition scores and weighted factors; on demand, not queue positions |
| `perch_waits` | Personal partition wait statistics and sample counts; no arguments, no predictions |
| `perch_job` | One allocation or array task with reconciled state |
| `perch_logs` | Bounded stdout or stderr tail |
| `perch_script` | Submitted script as text |
| `perch_resources` | Recorded measurements and separate running-step samples (`live_steps`) |
| `perch_arrays` | Observed task totals and confirmed failed-task selection |
| `perch_partitions` | Visible partitions and allocation counts |
| `perch_software` | Search the installed Lmod catalog across compiler families |
| `perch_module` | Prerequisites and a displayed load command for an exact module |
| `perch_doctor` | Live capability and schema checks |
| `perch_guide` | Offline Hummingbird/Perch guidance |

Job, array, partition, node, priority and software list tools accept `offset` and `limit` (default 50, maximum 200) and return pagination metadata. Every page is a fresh query; changes between pages can shift records. Job tools accept a `job_id`, such as `9100_11`. Collapsed ranges are not single-job identifiers.

`perch_software` also accepts a case-insensitive `search` substring, filtered before pagination. `perch_module` accepts `name` and an optional `max_chars` (default 12,000; maximum 32,768). Module instructions and load commands are returned as data, never executed. Availability is specific to the query login environment; inspect prerequisite instructions before using a module.

Log output defaults to the last 12,000 characters of a sanitized, at-most-256-KiB tail. Scripts default to their first 16,000 characters. `max_chars` can raise either limit to 32,768. `text_truncated` means additional MCP character trimming occurred; `tail_limit_bytes` describes the separate backend log limit. Logs are not promised to contain complete lines or an entire file.

Query responses provide structured content and a JSON text representation for clients that do not consume structured content. They identify source (`demo` or `hummingbird`), observation timestamp, schema version, and accounting window where applicable. Durations use seconds, memory uses bytes, and missing measurements are null. Doctor uses the existing capability-report shape; guide results contain topic and Markdown text. Invalid inputs, unavailable SSH, missing logs and query errors are failures, never empty successful results.

All current tools are read-only. They do not submit, cancel, run returned scripts, execute arbitrary shell commands, or install software. Logs, names and script content are untrusted data even after terminal escape sanitization. Agents should distinguish scheduler facts from diagnostic hypotheses and obtain authorization before making workload changes through other tools.

The guide also describes the separate `perch shell JOB_ID` CLI. Its default
preview is read-only; `--open` starts a shared step and requires a real terminal.
No MCP tool opens that shell. See [allocation shells](ALLOCATION-SHELL.md) for
resource scope, environment and lifecycle behavior.

## Queued-job timing

Job results separate actual execution (`started_at`) from a queued job's tentative
scheduler prediction (`estimated_start_at`). Either may be null. For a pending
job, `started_at` is null even when a prediction is available. A prediction can
move or expire while the job remains queued; it must not be interpreted as a
reservation, countdown to guaranteed execution or state transition.

Priority reports preserve Slurm's total separately from potentially rounded
weighted factors. Nice is a separate adjustment. Zero contribution is not proof
that a factor is disabled, and held/new jobs can be absent. `allocation_id` is a
numeric scheduler allocation ID, including array allocations; it is not a task
count or a guaranteed logical array-task ID. Scores are partition-specific.
`perch_priority` is read on demand and must not be used for tight polling.

Node results use bytes for all memory fields and preserve null for missing
measurements. `unallocated_memory_bytes` is configured minus allocated minus
system-reserved memory; negative values remain negative. It is not OS free memory
or a scheduling promise. States/partitions are lists; shared partition membership
does not duplicate a node. GRES allocation does not measure GPU utilization.
Node queries are on demand and limited to the local cluster visible to the account.
Optional `gpus` entries contain `type`, `configured`, `allocated` and
`unallocated`. Counts are explicit GPU GRES units, including MIG instances where
configured; they are not necessarily physical cards. Unknown or inconsistent
allocation counts are null with `gpu_note` explaining the limitation. Unsupported
descriptions retain raw `gres` / `gres_used` rather than inventing capacity.
The parser uses [Slurm's named GRES counts](https://slurm.schedmd.com/gres.html);
device-index lists are metadata, not a second set of resources to sum.

## Guides and prompt

Each guide is available through `perch_guide` and as a Markdown resource:

- `perch://guide/getting-started`: access and first-workload workflow.
- `perch://guide/jobs-and-arrays`: scheduling, accounting, task counts and evidence.
- `perch://guide/storage-and-software`: storage, backups, modules and reproducibility.
- `perch://guide/perch`: TUI, CLI, MCP and troubleshooting.

The `investigate_job` prompt accepts a job ID and outlines an evidence-based diagnostic workflow. It fetches nothing until the client acts on it. Guides link to official sources and label dated local observations; live configuration always needs live verification.

## Verification

```sh
make check
make mcp-test
PERCH_MCP_LIVE_HOST=your-cruzid@hb.ucsc.edu go test -count=1 ./cmd/perch -run '^TestMCPLive$' -v
# Check one live capability without repeating the other cluster queries:
PERCH_MCP_LIVE_HOST=your-cruzid@hb.ucsc.edu go test -count=1 ./cmd/perch -run '^TestMCPLive$/^perch_priority$' -v
```

Tests cover discovery, typed schemas, every demo query, all resources, the prompt, pagination, text bounds, malformed arguments, offline guidance, request cancellation, queued deadlines and recovery. An official SDK client launches the actual command entrypoint over stdio and verifies graceful EOF shutdown. A separate standard-library Python wire check exercises the built executable using newline-delimited JSON-RPC. Live testing is opt-in and reads existing state only. Integration with a particular desktop client's UI still requires that client's acceptance check.

Diagnostics: `perch mcp --debug` records bounded operation metadata locally and
announces its path on stderr. Protocol stdout stays clean. Optional
`--debug-raw` captures scheduler JSON separately and may include private metadata;
review captures before sharing. See the README for retention and exclusions.
