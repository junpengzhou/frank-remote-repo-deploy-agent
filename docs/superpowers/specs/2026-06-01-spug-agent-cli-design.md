# Spug Agent CLI Design

## Goal

Build a standard, extensible Go CLI that Spug can call once per release to deploy one or more Java modules from source to Docker-backed remote hosts.

## Scope

The first version is a one-shot command:

```bash
spug-agent deploy --config configs/agent.yaml --env test --modules ifintech-frank,ifintech-user
```

It handles source checkout, Maven build cache decisions, packaging, rsync deployment, remote restart, and log tail output. It does not expose a long-running HTTP API.

## Architecture

The CLI is split into small internal packages:

- `internal/config` loads and validates YAML configuration.
- `internal/runner` runs external commands with context cancellation and live output.
- `internal/gitops` handles clone, fetch, checkout, pull, and commit hash discovery.
- `internal/maven` plans and runs Maven builds with dependency cache checks.
- `internal/cache` stores module branch commit hashes in a local JSON file.
- `internal/packagex` resolves Maven artifacts and prepares staging directories.
- `internal/rsync` builds and runs rsync commands.
- `internal/remote` builds and runs SSH commands for scripts, Docker restart, and tail logs.
- `internal/lock` handles same-module cancellation locks and a global Maven install lock.
- `internal/deploy` orchestrates the full pipeline.

External systems remain command-line tools: `git`, `mvn`, `rsync`, `ssh`, `unzip`, and optionally `docker` through SSH.

## Configuration

All operational parameters live in YAML:

- workspace paths
- cache file path
- Maven executable, settings, local repository, extra arguments
- JDK `JAVA_HOME`
- SSH user, host, port, key file
- rsync executable and options
- environment-to-branch mapping
- module repository, dependencies, packaging, remote path, container, log file, and optional remote script

CLI flags choose config, environment, modules, concurrency, dry-run mode, and whether to follow remote logs.

## Data Flow

For each requested module:

1. Acquire same-module deployment lock and cancel any older task marker.
2. Resolve module and dependency definitions from config.
3. Clone or update source repositories.
4. Checkout the environment branch.
5. Compare dependency commit hashes with the JSON cache.
6. Build changed dependencies under the Maven install lock.
7. Build the requested main module.
8. Prepare the artifact:
   - WAR: extract into a staging directory.
   - JAR: copy to a staging directory.
9. Sync staging to the remote target with `rsync --delete`.
10. Run the configured remote deploy script or `docker restart`.
11. Print remote tail output.

## Concurrency

Multiple main modules can deploy concurrently. Shared Maven local repository writes are protected by a global Maven lock. Same-module deployments are preemptive: a newer run writes a new generation value, and older runs check that value between external steps and exit quickly when superseded.

## Error Handling

Each stage returns contextual errors that include the module, stage name, command, exit code, and stderr summary. Command output is streamed to stdout/stderr so Spug can display progress in real time.

## Testing

Tests focus on deterministic behavior:

- config loading and validation
- cache change detection
- lock generation and supersede checks
- command argument construction for Maven, rsync, and SSH
- artifact staging decisions

External process execution is abstracted behind interfaces so tests do not need real git, Maven, SSH, or rsync.
