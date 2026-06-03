package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"frank-remote-repo-deploy-agent/internal/cache"
	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/deploy"
	"frank-remote-repo-deploy-agent/internal/runner"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "salt-agent: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "deploy":
		return runDeploy(args[1:])
	case "-h", "--help", "help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDeploy(args []string) error {
	// CLI 只负责把 Salt 传入的参数翻译成部署选项；真正的流程编排在 internal/deploy。
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to YAML config")
	env := fs.String("env", "", "deployment environment name")
	modulesCSV := fs.String("modules", "", "comma-separated module names")
	concurrency := fs.Int("concurrency", 1, "number of main modules to deploy concurrently")
	dryRun := fs.Bool("dry-run", false, "print commands without executing them")
	debug := fs.Bool("debug", false, "print verbose deployment details")
	followLogs := fs.Bool("tail", false, "follow remote logs after deployment")
	tailLines := fs.Int("tail-lines", 300, "number of remote log lines to print")
	operator := fs.String("user", "", "operator name passed to module remoteScript as --user")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *env == "" {
		return fmt.Errorf("--env is required")
	}
	if *modulesCSV == "" {
		return fmt.Errorf("--modules is required")
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}

	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	modules, err := cfg.ModuleNames(*modulesCSV)
	if err != nil {
		return err
	}
	if err := deploy.EnsureDirs(cfg); err != nil {
		return err
	}
	// 构建缓存是本地 JSON 文件，用于判断基础模块当前分支 HEAD 是否发生变化。
	store, err := cache.Load(cfg.CacheFile)
	if err != nil {
		return err
	}
	// Salt 或用户中断进程时，context 会传递给 git/mvn/rsync/ssh 等外部命令。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	exec := runner.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr, DryRun: *dryRun, Debug: *debug}
	deployer := deploy.New(cfg, exec, exec, store)
	if *debug {
		fmt.Printf("[deploy] env=%s modules=%s concurrency=%d dryRun=%v debug=%v\n", *env, strings.Join(modules, ","), *concurrency, *dryRun, *debug)
	}
	return deployer.Run(ctx, deploy.Options{
		Env:         *env,
		Modules:     modules,
		Concurrency: *concurrency,
		FollowLogs:  *followLogs,
		TailLines:   *tailLines,
		Operator:    *operator,
		Debug:       *debug,
	})
}

func usage() error {
	_, _ = fmt.Fprintln(os.Stderr, `Usage:
  salt-agent deploy --config configs/agent.yaml --env test --modules demo1[,demo2]

Options:
  --concurrency N   Deploy multiple main modules concurrently.
  --debug           Print verbose deployment details.
  --dry-run         Print external commands without running them.
  --user NAME       Append --user NAME to module remoteScript.
  --tail            Follow remote logs after restart.
  --tail-lines N    Number of log lines to print, default 3000.`)
	return nil
}
