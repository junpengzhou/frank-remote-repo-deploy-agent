# Git Log Compatibility Patch Design

## Purpose

Fix two production issues in `salt-agent-metadata.json` commit histories:

1. merge commits currently occupy the three-record history and obscure the
   application changes operators want to confirm;
2. production Git 1.8.3.1 does not recognize the `%cI` pretty-format token and
   emits the literal text `%cI`, which is copied into `committedAt`.

The production host `trial-1` has verified that Git 1.8.3.1 supports both
`%ci` and `--no-merges` with this output shape:

```text
3d69fba4d5b2a5e4cc26588d0916f465fc6f5bec^@RayLin^@RayLin@ifintech.ltd^@2026-07-22 14:53:24 +0800^@feature/sprint53/OPER-18075【官网】替换logo图增加多图
```

## Git Query

Change the recent-commit query to the Git 1.8.3.1-compatible form:

```text
git log -n 3 --format=%H%x00%cn%x00%ce%x00%ci%x00%s --no-merges
```

Rules:

- `--no-merges` filters merge commits before the three-record limit is
  satisfied, so each module contributes its latest zero to three non-merge
  commits.
- `%ci` replaces unsupported `%cI` and preserves the committer's recorded
  numeric timezone offset.
- All other fields and the NUL-separated record format remain unchanged.

## Timestamp Conversion

`gitops` parses the `%ci` field using Go's fixed layout:

```text
2006-01-02 15:04:05 -0700
```

It then serializes the parsed value with `time.RFC3339`. For example:

```text
2026-07-22 14:53:24 +0800
```

becomes:

```text
2026-07-22T14:53:24+08:00
```

This preserves the original instant and timezone offset while satisfying the
existing JSON Schema `date-time` contract.

If the Git time cannot be parsed, `RecentCommits` returns an error instead of
publishing invalid text. The existing deploy degradation behavior then logs a
warning and publishes `"commits": []` for that module without blocking the
deployment.

## Compatibility and Protocol

- Minimum verified production Git: 1.8.3.1.
- JSON field names and types do not change.
- `schemaVersion` remains `1.0` because this is a producer bug fix and selection
  refinement, not a wire-format change.
- The downstream protocol document will state that `commits` contains the
  latest non-merge commits.

## Testing

Follow TDD:

1. update the command-construction test to require `%ci` and `--no-merges`;
2. feed the parser a Git 1.8.3.1 `%ci` value and require RFC 3339 output;
3. verify invalid or literal placeholder time text returns a parse error;
4. run `go test ./internal/gitops`, then `go test ./...`;
5. build `./cmd/salt-agent` and validate the updated documentation.

## Non-Goals

- No Git version probe or new configuration option.
- No changes to hashes, committer identity, descriptions, module ordering,
  cache behavior, staging, rsync, or restart behavior.
- No schema version bump.
