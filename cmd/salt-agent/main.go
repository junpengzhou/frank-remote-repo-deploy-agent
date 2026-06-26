package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"frank-remote-repo-deploy-agent/internal/cache"
	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/constants"
	"frank-remote-repo-deploy-agent/internal/deploy"
	"frank-remote-repo-deploy-agent/internal/output"
	registerx "frank-remote-repo-deploy-agent/internal/register"
	"frank-remote-repo-deploy-agent/internal/runner"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		output.Error("salt-agent: %v", err)
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
	case "register":
		return runRegister(args[1:])
	case "-h", "--help", "help":
		return usage()
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type registerCLIOptions struct {
	registerx.Options
	DryRun        bool
	Debug         bool
	RegenerateKey bool
}

func parseRegisterOptions(args []string) (registerCLIOptions, error) {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	sshDir := fs.String("ssh-dir", "", "local SSH directory")
	host := fs.String("host", "", "remote server host")
	port := fs.Int("port", 22, "remote SSH port")
	user := fs.String("user", "", "remote SSH user")
	password := fs.String("password", "", "remote SSH password")
	remotePath := fs.String("remote-path", "", "remote salt-agent path")
	scriptsDir := fs.String("scripts-dir", "scripts", "local scripts directory to sync")
	regenerateKey := fs.Bool("regenerate-key", false, "regenerate SSH key pair when id_rsa already exists")
	dryRun := fs.Bool("dry-run", false, "print register actions without connecting")
	debug := fs.Bool("debug", false, "print verbose register details")
	if err := fs.Parse(args); err != nil {
		return registerCLIOptions{}, err
	}
	scriptsDirProvided := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "scripts-dir" {
			scriptsDirProvided = true
		}
	})
	opts := registerCLIOptions{
		Options: registerx.Options{
			SSHDir:             *sshDir,
			Host:               *host,
			Port:               *port,
			User:               *user,
			Password:           *password,
			RemotePath:         *remotePath,
			ScriptsDir:         *scriptsDir,
			ScriptsDirProvided: scriptsDirProvided,
		},
		DryRun:        *dryRun,
		Debug:         *debug,
		RegenerateKey: *regenerateKey,
	}
	if opts.SSHDir == "" {
		return registerCLIOptions{}, fmt.Errorf("--ssh-dir is required")
	}
	if opts.Host == "" {
		return registerCLIOptions{}, fmt.Errorf("--host is required")
	}
	if opts.User == "" {
		return registerCLIOptions{}, fmt.Errorf("--user is required")
	}
	if opts.Password == "" {
		return registerCLIOptions{}, fmt.Errorf("--password is required")
	}
	if opts.RemotePath == "" {
		return registerCLIOptions{}, fmt.Errorf("--remote-path is required")
	}
	return opts, nil
}

func runRegister(args []string) error {
	opts, err := parseRegisterOptions(args)
	if err != nil {
		return err
	}
	output.SetDebug(opts.Debug)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	keyPair, err := registerx.EnsureKeyPair(opts.SSHDir, opts.RegenerateKey)
	if err != nil {
		return err
	}
	opts.PublicKeyPath = keyPair.PublicKeyPath

	var client registerx.Client
	if opts.DryRun {
		client = registerx.NewDryRunClient(os.Stdout)
	} else {
		client, err = registerx.NewNativeClient(ctx, registerx.NativeOptions{
			SSHDir:   opts.SSHDir,
			Host:     opts.Host,
			Port:     opts.Port,
			User:     opts.User,
			Password: opts.Password,
		})
		if err != nil {
			return err
		}
	}
	output.Debug("register host=%s port=%d user=%s remotePath=%s scriptsDir=%s dryRun=%v",
		opts.Host, opts.Port, opts.User, opts.RemotePath, opts.ScriptsDir, opts.DryRun)
	if err := (registerx.Registrar{Client: client}).Run(ctx, opts.Options); err != nil {
		return err
	}
	if opts.DryRun {
		return nil
	}
	keyClient, err := registerx.NewNativeClient(ctx, registerx.NativeOptions{
		SSHDir:  opts.SSHDir,
		Host:    opts.Host,
		Port:    opts.Port,
		User:    opts.User,
		KeyFile: keyPair.PrivateKeyPath,
	})
	if err != nil {
		return fmt.Errorf("verify passwordless ssh connection: %w", err)
	}
	defer func() {
		_ = keyClient.Close()
	}()
	if err := keyClient.Run(ctx, "echo 'Passwordless login successful'"); err != nil {
		return fmt.Errorf("verify passwordless ssh connection: %w", err)
	}
	output.Success(registerSuccessMessage(opts.Host, opts.RemotePath))
	return nil
}

func registerSuccessMessage(host, remotePath string) string {
	return fmt.Sprintf("register completed successfully, host: %s, remote path: %s", host, remotePath)
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
	tailLines := fs.Int("tail-lines", constants.DefaultTailLines, "number of remote log lines to print")
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
	output.SetDebug(*debug)
	exec := runner.ExecRunner{Stdout: os.Stdout, Stderr: os.Stderr, DryRun: *dryRun, Debug: *debug}
	deployer := deploy.New(cfg, exec, exec, store)
	output.Debug("deploy env=%s modules=%v concurrency=%d dryRun=%v debug=%v", *env, modules, *concurrency, *dryRun, *debug)
	return deployer.Run(ctx, deploy.Options{
		Env:         *env,
		Modules:     modules,
		Concurrency: *concurrency,
		TailLines:   *tailLines,
		Operator:    *operator,
	})
}

func usage() error {
	_, _ = fmt.Fprintf(os.Stderr, "Usage:\n"+
		"  salt-agent deploy --config configs/agent.yaml --env test --modules demo1[,demo2]\n"+
		"  salt-agent register --ssh-dir /data/salt-agent/.ssh --host 10.0.0.1 --port 22 --user root --password secret --remote-path /data/salt-agent\n"+
		"\n"+
		"Options:\n"+
		"  --concurrency N   Deploy multiple main modules concurrently.\n"+
		"  --dry-run         Print external commands without running them.\n"+
		"  --regenerate-key  Regenerate register SSH key pair when id_rsa already exists.\n"+
		"  --user NAME       Append --user NAME to module remoteScript.\n"+
		"  --tail-lines N    Number of log lines to print, default %d.\n", constants.DefaultTailLines)
	return nil
}
