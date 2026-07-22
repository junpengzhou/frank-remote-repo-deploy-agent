# Git Log Compatibility Patch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Exclude merge commits from build metadata and emit valid RFC 3339 committer times on production Git 1.8.3.1.

**Architecture:** Keep commit discovery inside `internal/gitops`. Query Git with the verified legacy-compatible `%ci` token and `--no-merges`, then normalize `%ci` to RFC 3339 before returning `Commit` records; the deploy and JSON layers remain unchanged.

**Tech Stack:** Go 1.25 standard library, Git CLI 1.8.3.1+, existing `OutputRunner`, Markdown.

---

## File Map

- Modify `internal/gitops/gitops_test.go`: specify the compatible Git command,
  timestamp normalization, and invalid-time behavior.
- Modify `internal/gitops/gitops.go`: query non-merge commits with `%ci` and
  normalize the legacy timestamp.
- Modify `docs/salt-agent-metadata.md`: define histories as non-merge commits.
- Modify `README.md` and `README.zh-CN.md`: describe recent non-merge commits.

### Task 1: Make Recent Commit Discovery Compatible with Git 1.8.3.1

**Files:**
- Modify: `internal/gitops/gitops_test.go`
- Modify: `internal/gitops/gitops.go`

- [ ] **Step 1: Change the existing test to require `%ci`, RFC 3339, and `--no-merges`**

In `TestRecentCommitsQueriesAndParsesNewestRecords`, replace the runner output,
expected command, and timestamp assertion with:

```go
run := &recordingRunner{outputs: []string{
	"0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x002026-07-17 13:20:30 +0800\x00发布元数据\n" +
		"89abcdef0123456789abcdef0123456789abcdef\x00Developer\x00dev@example.com\x002026-07-16 18:10:00 +0800\x00Update dependency\n",
}}

want := runner.Command{
	Name: "git",
	Args: []string{
		"log",
		"-n",
		"3",
		"--format=%H%x00%cn%x00%ce%x00%ci%x00%s",
		"--no-merges",
	},
	Dir: "/workspace/example",
}

if commits[0].CommittedAt != "2026-07-17T13:20:30+08:00" {
	t.Fatalf("unexpected commit time: %q", commits[0].CommittedAt)
}
```

Keep the existing hash, committer, description, record count, and command
assertions around this replacement.

- [ ] **Step 2: Add a failing regression test for the production `%cI` symptom**

Add to `internal/gitops/gitops_test.go`:

```go
func TestRecentCommitsRejectsInvalidCommitTime(t *testing.T) {
	run := &recordingRunner{outputs: []string{
		"0123456789abcdef0123456789abcdef01234567\x00Frank Zhou\x00frank@example.com\x00%cI\x00发布元数据\n",
	}}

	_, err := RecentCommits(context.Background(), run, "/workspace/example", 3)
	if err == nil || !strings.Contains(err.Error(), "commit time") {
		t.Fatalf("expected commit time parse error, got %v", err)
	}
}
```

- [ ] **Step 3: Run the focused tests and verify RED**

Run:

```powershell
go test ./internal/gitops -run 'TestRecentCommits(QueriesAndParsesNewestRecords|RejectsInvalidCommitTime)' -v
```

Expected: the command assertion still contains `%cI` without `--no-merges`, the
valid `%ci` time is not normalized, and literal `%cI` is not rejected.

- [ ] **Step 4: Implement the minimal compatible query and timestamp parser**

Add `time` to `internal/gitops/gitops.go` imports and define:

```go
const gitCommitTimeLayout = "2006-01-02 15:04:05 -0700"
```

Change the Git command to:

```go
value, err := out.Output(ctx, runner.Command{
	Name: "git",
	Args: []string{
		"log",
		"-n",
		strconv.Itoa(limit),
		"--format=%H%x00%cn%x00%ce%x00%ci%x00%s",
		"--no-merges",
	},
	Dir: dir,
})
```

In `parseCommitLog`, parse field 4 before appending the record:

```go
committedAt, err := time.Parse(gitCommitTimeLayout, fields[3])
if err != nil {
	return nil, fmt.Errorf("parse git log record %d commit time %q: %w", index+1, fields[3], err)
}
```

Then assign:

```go
CommittedAt: committedAt.Format(time.RFC3339),
```

- [ ] **Step 5: Run GREEN and the full package tests**

Run:

```powershell
gofmt -w internal/gitops/gitops.go internal/gitops/gitops_test.go
go test ./internal/gitops -run 'TestRecentCommits' -v
go test ./internal/gitops
```

Expected: every recent-commit test and the complete `gitops` package pass.

- [ ] **Step 6: Commit the compatibility fix**

```powershell
git add internal/gitops/gitops.go internal/gitops/gitops_test.go
git commit -m "fix(gitops): support legacy commit timestamps"
```

### Task 2: Document Non-Merge Commit Histories

**Files:**
- Modify: `docs/salt-agent-metadata.md`
- Modify: `README.md`
- Modify: `README.zh-CN.md`

- [ ] **Step 1: Update the normative commit selection text**

Make these exact semantic changes in `docs/salt-agent-metadata.md`:

```markdown
| `commits` | array | 是 | 从该模块本地 `HEAD` 开始的最近零至三条非 merge 提交。 |
```

```markdown
提交已过滤 merge commit，并按从新到旧排列。仓库只有一条或两条非 merge 提交时，数组只包含实际存在的记录，不会补齐到三条。
```

In the generation flow, change the collection step to:

```markdown
3. Maven 阶段成功后读取每个模块最近最多三条本地非 merge 提交。
```

Add this compatibility statement after that flow:

```markdown
提交查询使用 Git 1.8.3.1 已验证支持的 `%ci` 和 `--no-merges`；`%ci` 时间由 `salt-agent` 转换为 RFC 3339 后再写入 JSON。
```

- [ ] **Step 2: Update both README summaries**

Use these bullets:

```markdown
- Build metadata: each staged deployment includes [`salt-agent-metadata.json`](docs/salt-agent-metadata.md) with build timing and up to three recent non-merge commits for the main module and every configured dependency.
```

```markdown
- 构建元数据：每次 staging 都会包含 [`salt-agent-metadata.json`](docs/salt-agent-metadata.md)，记录构建时间以及主模块和各依赖模块最近最多三条非 merge 提交。
```

- [ ] **Step 3: Verify and commit documentation**

Run:

```powershell
rg -n '%ci|--no-merges|非 merge|non-merge' docs/salt-agent-metadata.md README.md README.zh-CN.md
git diff --check
git add docs/salt-agent-metadata.md README.md README.zh-CN.md
git commit -m "docs: clarify non-merge commit metadata"
```

Expected: the search finds the compatibility rule and all three user-facing
non-merge descriptions; `git diff --check` exits 0.

### Task 3: Full Verification

**Files:**
- Verify all modified files

- [ ] **Step 1: Run the complete test suite without cached results**

```powershell
go test -count=1 ./...
```

Expected: every package reports `ok` or `[no test files]` with no failures.

- [ ] **Step 2: Build the CLI**

```powershell
go build -o salt-agent ./cmd/salt-agent
```

Expected: exit 0. Remove the generated local binary after confirming the build;
do not stage it.

- [ ] **Step 3: Audit the final contract and repository state**

```powershell
rg -n '%ci|--no-merges|time.RFC3339|gitCommitTimeLayout' internal/gitops docs/salt-agent-metadata.md
git diff --check
git status --short --branch
```

Confirm that the exact production symptom `%cI` exists only in the regression
test and compatibility explanation, never in the generated command.
