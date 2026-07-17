# Salt Agent Build Metadata Design

## Purpose

After a successful Maven phase, every deployment should publish a versioned
`salt-agent-metadata.json` file alongside the staged application files. The file
allows downstream restart plugins and application code to identify the exact
main-module and dependency commits that were synchronized to the remote host.

The metadata is informational. It proves which build output was synchronized,
but it does not prove that the later remote restart succeeded or that the
application became healthy.

## Scope

This change applies only to the `deploy` command. It does not change
configuration, the `register` flow, Maven cache decisions, artifact selection,
remote restart behavior, or the `rsync --delete` contract.

For each requested main module, the deployment produces one independent
metadata file in that module's staging root. The file covers the main module and
its configured direct dependencies.

## Metadata Contract

The file name is always `salt-agent-metadata.json`. It is UTF-8 JSON with a
trailing newline and is written at the staging root:

- WAR deployment: the file is synchronized to the application root next to the
  extracted WAR contents.
- JAR deployment: the file is synchronized next to the JAR file.

The initial schema version is `1.0`:

```json
{
  "schemaVersion": "1.0",
  "generatedAt": "2026-07-17T14:35:12+08:00",
  "environment": "test",
  "branch": "test",
  "mainModule": "example-app",
  "build": {
    "startedAt": "2026-07-17T14:34:01+08:00",
    "finishedAt": "2026-07-17T14:35:12+08:00",
    "durationMs": 71000,
    "buildSkipped": false
  },
  "modules": [
    {
      "name": "example-app",
      "role": "main",
      "commits": [
        {
          "hash": "0123456789abcdef0123456789abcdef01234567",
          "committer": {
            "name": "Frank Zhou",
            "email": "frank@example.com"
          },
          "committedAt": "2026-07-17T13:20:30+08:00",
          "description": "Add build metadata"
        }
      ]
    },
    {
      "name": "example-common",
      "role": "dependency",
      "commits": []
    }
  ]
}
```

All fields shown above are required, including `commits`. An unavailable or
empty commit history is represented by an empty array, never by a missing field
or `null`.

### Top-Level Fields

- `schemaVersion`: Contract version. Version `1.0` is the initial format.
- `generatedAt`: Time at which the metadata model is finalized.
- `environment`: Environment name passed to `deploy --env`.
- `branch`: Branch resolved from the selected environment.
- `mainModule`: Requested main module for this deployment unit.
- `build`: Timing and cache-skip status for the Maven phase.
- `modules`: Main module first, followed by dependencies in configured order.

### Build Fields

- `startedAt`: Start of the Maven decision/build phase.
- `finishedAt`: Successful completion of the Maven decision/build phase.
- `durationMs`: Non-negative elapsed milliseconds between the two timestamps.
- `buildSkipped`: `true` only when no Maven command was executed because every
  dependency and the main module satisfied the existing cache rules. If any
  module executes Maven, the value is `false`.

Build and generation timestamps use RFC 3339 and retain their timezone offset.
The duration is calculated with Go's monotonic elapsed-time support where
available and serialized as an integer number of milliseconds.

### Module and Commit Fields

- `name`: Configured module name.
- `role`: `main` or `dependency`.
- `commits`: Up to three commits from local `HEAD`, newest first.
- `hash`: Full Git object ID; it is not abbreviated.
- `committer.name` and `committer.email`: Git committer identity.
- `committedAt`: Git committer date in strict ISO/RFC 3339 form, retaining its
  recorded offset.
- `description`: Commit subject only. The commit body is not included.

Descriptions are counted as Unicode code points, not UTF-8 bytes. A description
of 100 characters or fewer is unchanged. A longer description is reduced to its
first 100 characters and then receives the three ASCII characters `...`; the
ellipsis is not included in the 100-character limit.

A repository with fewer than three commits contributes only its available
commits. Failure to retrieve or parse a module's history produces
`"commits": []` for that module and a warning; it does not stop deployment.

## Architecture

### `internal/gitops`

Add a focused recent-commit query that uses the existing `OutputRunner`
abstraction. It runs one local `git log -n 3` command per module after the Maven
phase. A machine-oriented format carries the full hash, committer name,
committer email, strict committer timestamp, and subject. Parsing remains in
`gitops`, so callers do not depend on Git's output encoding details.

### `internal/metadata`

Add a package that owns:

- the versioned JSON data model;
- Unicode-safe description truncation;
- deterministic module and commit representation;
- indented UTF-8 JSON serialization with a trailing newline; and
- writing `salt-agent-metadata.json` to a supplied staging directory.

The writer uses a temporary file and rename within the staging directory so a
failed write does not leave a partially valid metadata file.

### `internal/deploy`

The deploy orchestrator owns lifecycle data that other packages cannot know:

- environment, branch, and main-module identity;
- Maven phase start and finish times;
- whether any Maven command was executed;
- configured dependency order; and
- the final staging directory.

Time acquisition and metadata writing will be replaceable inside deploy tests,
without adding user-facing configuration.

## Deployment Data Flow

For each requested main module:

1. Acquire the existing same-module lease.
2. Resolve the environment branch.
3. Ensure and checkout the main and dependency repositories.
4. Record the Maven phase start.
5. Apply the existing dependency and main-module cache/build rules.
6. If the Maven phase succeeds, record its finish and skip status.
7. Read up to three local commits for the main module and each dependency.
8. Find the application artifact and rebuild the module staging directory.
9. Create `salt-agent-metadata.json` in the staging root.
10. Synchronize the whole staging root with the existing rsync command.
11. Run the existing remote script or container restart.

Commit histories are queried after a successful Maven phase as requested. They
come from the already fetched and checked-out local repositories, so metadata
collection adds no second network fetch.

## Failure and Degradation Behavior

Metadata is best-effort and must never prevent application deployment:

- A Git query or parse failure logs a warning and emits an empty commit array
  for that module.
- JSON serialization, temporary-file creation, writing, or rename failure logs
  a warning and skips the metadata step. Rsync and restart continue.
- When metadata writing is skipped, the staging root contains no metadata file.
  The unchanged `rsync --delete` behavior therefore removes any stale remote
  `salt-agent-metadata.json`, preventing consumers from mistaking an old file
  for the current deployment.
- A Maven failure retains the existing fail-fast behavior. No staging sync or
  metadata publication occurs for that deployment.
- An rsync or restart failure retains the existing deployment error behavior.
  If rsync succeeded but restart failed, the new metadata may already exist on
  the remote host; consumers must not treat file presence as restart success.

## Concurrency and Safety

Different main modules already use different staging subdirectories. Each
deployment therefore creates an independent metadata file without shared
mutable state. The same-module lease continues to prevent two generations from
reaching later stage boundaries concurrently.

The metadata is added only after `PrepareStaging` has rebuilt the staging
directory, so it cannot be erased by that cleanup step. Existing rsync source
and destination semantics remain unchanged.

## Downstream Documentation

Create `docs/salt-agent-metadata.md` as the consumer-facing protocol reference
and link it from both READMEs. It will contain:

- file location and publication lifecycle;
- a normative field table;
- full, cache-hit, short-history, and unavailable-history examples;
- timestamp, ordering, hash, identity, and truncation rules;
- best-effort generation and stale-file deletion behavior;
- restart-status caveats;
- compatibility guidance; and
- parsing recommendations for downstream plugins and applications.

Also publish `docs/schemas/salt-agent-metadata.schema.json` as a machine-readable
JSON Schema. Consumers should reject unsupported schema major versions and
ignore unknown fields within a supported major version. Compatible additive
fields increment the minor version; removals or semantic changes require a new
major version.

## Testing Strategy

Follow test-driven development for each behavior:

- `internal/gitops`: assert the exact Git command and parse zero through three
  commits, Unicode subjects, identities, strict timestamps, and malformed data.
- `internal/metadata`: verify the contract model, `commits: []`, indented JSON,
  trailing newline, full hashes, and Unicode-safe 100-character truncation.
- `internal/deploy`: verify build timing, all-cache-hit skip status, stable module
  order, staging-root placement, Git failure degradation, and continued rsync
  and restart after metadata writer failure.
- Run targeted package tests first, followed by `go test ./...` and
  `go build -o salt-agent ./cmd/salt-agent`.

## Non-Goals

- No new YAML settings or CLI flags.
- No remote read-back or metadata verification after rsync.
- No service health assertion.
- No commit bodies, diffs, tags, or changed-file lists.
- No historical metadata archive; each deployment publishes only the current
  file, containing at most three commits per module.
