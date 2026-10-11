# Getting started on Hummingbird

Guidance reviewed October 10, 2026. Verify changing configuration with live tools.

UCSC's getting-started guide uses `ssh YOUR_CRUZID@hb.ucsc.edu`, campus CruzID Gold credentials, and campus VPN for off-campus access. Enter passwords in your own SSH terminal, never in an agent message or tool argument. Perch uses the system SSH client and its config, keys and known-host checks. Do not disable host-key verification to solve an authentication problem.

For a first workload:

1. Open `perch --host YOUR_CRUZID@hb.ucsc.edu`, or use `perch_jobs` to check access. Use `perch_doctor` when the adapter or capabilities fail.
2. Inspect `perch_partitions` for currently visible resources. Availability changes; a public hardware list is not a promise of account access or immediate scheduling.
3. Prepare a small Slurm batch script with explicit CPUs, memory, time limit, working directory and separate output/error paths. Create required output directories before submission. Check the software environment and input paths.
4. Review the script and resource request. Hummingbird's documented job submission path is `sbatch`; Perch's current MCP tools do not submit jobs. Obtain the user's authorization for workload changes.
5. Record the returned job ID. Follow it with `perch_job`, inspect output, and check terminal accounting before calling it successful.

Use compute allocations for workloads; avoid launching heavy work on the login node. An empty queue is not evidence that an earlier job succeeded.

Source: [UCSC Hummingbird getting started](https://hummingbird.ucsc.edu/getting-started/).
