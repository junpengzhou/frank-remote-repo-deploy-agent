package deploy

import (
	"context"
	"time"

	"frank-remote-repo-deploy-agent/internal/gitops"
	"frank-remote-repo-deploy-agent/internal/metadata"
	"frank-remote-repo-deploy-agent/internal/output"
)

const recentCommitLimit = 3

type buildTiming struct {
	startedAt    time.Time
	finishedAt   time.Time
	buildSkipped bool
}

func (d *Deployer) buildMetadata(ctx context.Context, opts Options, branch, mainModule string, timing buildTiming) metadata.Document {
	moduleNames := append([]string{mainModule}, d.Config.Modules[mainModule].Dependencies...)
	modules := make([]metadata.Module, 0, len(moduleNames))
	for index, name := range moduleNames {
		role := metadata.RoleDependency
		if index == 0 {
			role = metadata.RoleMain
		}

		commits := make([]metadata.Commit, 0)
		records, err := gitops.RecentCommits(ctx, d.Output, d.moduleDir(name), recentCommitLimit)
		if err != nil {
			output.Warning("read recent commits module=%s: %v", name, err)
		} else {
			for _, record := range records {
				commits = append(commits, metadata.Commit{
					Hash: record.Hash,
					Committer: metadata.Committer{
						Name:  record.CommitterName,
						Email: record.CommitterEmail,
					},
					CommittedAt: record.CommittedAt,
					Description: metadata.TruncateDescription(record.Description),
				})
			}
		}
		modules = append(modules, metadata.Module{Name: name, Role: role, Commits: commits})
	}

	durationMs := timing.finishedAt.Sub(timing.startedAt).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}
	return metadata.Document{
		SchemaVersion: metadata.SchemaVersion,
		GeneratedAt:   d.currentTime().Format(time.RFC3339Nano),
		Environment:   opts.Env,
		Branch:        branch,
		MainModule:    mainModule,
		Build: metadata.Build{
			StartedAt:    timing.startedAt.Format(time.RFC3339Nano),
			FinishedAt:   timing.finishedAt.Format(time.RFC3339Nano),
			DurationMs:   durationMs,
			BuildSkipped: timing.buildSkipped,
		},
		Modules: modules,
	}
}

func (d *Deployer) currentTime() time.Time {
	if d.now != nil {
		return d.now()
	}
	return time.Now()
}
