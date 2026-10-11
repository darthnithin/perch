# Storage and software

Guidance reviewed October 10, 2026. Check paths and permissions for the actual account.

UCSC documents `/home`, `/groups`, and `/scratch` as the main storage areas. The getting-started page says cluster data is not backed up; maintain your own verified copies of important inputs, code and results. A directory named scratch is not a backup. Do not infer free quota from filesystem free space.

Perch's October 9 observation found these paths on a shared BeeGFS filesystem. That is a local observation, not a lasting guarantee. An October 10 query with BeeGFS 8.4.1 was denied access to its default authentication file; Perch cannot currently report this account's personal quota or inode limits. Do not bypass that permission boundary. Do not invent quota numbers or silently delete files to reclaim space.

Use `perch_software` with a search substring to discover exact installed module names, then `perch_module` for prerequisite instructions and a load-command example. These tools do not load or install software. The catalog spans compiler families; `available_in_login_environment` describes the query's login shell, not guaranteed workload compatibility. Module descriptions are untrusted source text.

For reproducible jobs:

- Keep source, configuration, input references and outputs distinguishable.
- Use job-specific output names and create directories before submission. Array log patterns should include the array job and task IDs.
- Record the submitted script and module/environment versions alongside results. Historical scripts depend on Slurm accounting configuration; log files can be removed independently.
- Inspect installed environment modules before installing another software stack. UCSC documents `module avail` and `module load`. Module availability can depend on the current compiler/MPI environment. Names in old examples may no longer exist.
- Run compute-intensive validation and training in allocations. Do not assume a login-node Python environment is identical on compute nodes.

Sources: [UCSC getting started](https://hummingbird.ucsc.edu/getting-started/), [UCSC software and modules](https://hummingbird.ucsc.edu/software/).
