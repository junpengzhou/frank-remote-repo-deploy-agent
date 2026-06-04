package deploy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"frank-remote-repo-deploy-agent/internal/cache"
	"frank-remote-repo-deploy-agent/internal/config"
	"frank-remote-repo-deploy-agent/internal/gitops"
	"frank-remote-repo-deploy-agent/internal/lock"
	"frank-remote-repo-deploy-agent/internal/maven"
	"frank-remote-repo-deploy-agent/internal/packagex"
	"frank-remote-repo-deploy-agent/internal/pomxml"
	"frank-remote-repo-deploy-agent/internal/remote"
	"frank-remote-repo-deploy-agent/internal/rsync"
	"frank-remote-repo-deploy-agent/internal/runner"
)

type Options struct {
	Env         string
	Modules     []string
	Concurrency int
	TailLines   int
	Operator    string
	Debug       bool
}

type Deployer struct {
	Config *config.Config
	// Runner 负责有实时输出的命令；Output 用于 git rev-parse 这类需要拿返回值的命令。
	Runner runner.Runner
	Output gitops.OutputRunner
	Cache  *cache.Store
	Locks  *lock.Manager
}

func New(cfg *config.Config, run runner.Runner, out gitops.OutputRunner, store *cache.Store) *Deployer {
	if out == nil {
		if outputRunner, ok := run.(gitops.OutputRunner); ok {
			out = outputRunner
		}
	}
	return &Deployer{
		Config: cfg,
		Runner: run,
		Output: out,
		Cache:  store,
		Locks:  lock.NewManager(cfg.LockDir),
	}
}

func (d *Deployer) Run(ctx context.Context, opts Options) error {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if len(opts.Modules) == 0 {
		return fmt.Errorf("no modules requested")
	}
	sem := make(chan struct{}, opts.Concurrency)
	errs := make(chan error, len(opts.Modules))
	var wg sync.WaitGroup
	// 多个主模块可以并发部署；共享资源通过内部锁保护。
	for _, module := range opts.Modules {
		module := module
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := d.deployOne(ctx, opts, module); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Deployer) deployOne(ctx context.Context, opts Options, moduleName string) error {
	// 获取同模块 lease 后，如果后续又有人部署同一模块，本次任务会在阶段边界快速退出。
	lease, err := d.Locks.AcquireModule(moduleName)
	if err != nil {
		return stageErr(moduleName, "acquire module lock", err)
	}
	branch, err := d.Config.BranchForEnv(opts.Env)
	if err != nil {
		return stageErr(moduleName, "resolve environment branch", err)
	}
	module := d.Config.Modules[moduleName]
	if module.RemotePath == "" {
		return stageErr(moduleName, "validate deploy target", fmt.Errorf("remotePath is required for requested module"))
	}
	if module.Container == "" && module.RemoteScript == "" {
		return stageErr(moduleName, "validate deploy target", fmt.Errorf("container or remoteScript is required for requested module"))
	}
	allModules := append([]string{}, module.Dependencies...)
	allModules = append(allModules, moduleName)
	git := gitops.Client{Runner: d.Runner}

	// 先把依赖模块和主模块都拉到本地，并确保聚合 pom.xml 里有对应 module。
	for _, name := range allModules {
		if err := lease.Check(); err != nil {
			return stageErr(moduleName, "superseded", err)
		}
		repoModule := d.Config.Modules[name]
		dir := d.moduleDir(name)
		if err := git.EnsureRepo(ctx, repoModule.Repo, dir); err != nil {
			return stageErr(name, "ensure git repository", err)
		}
		if err := git.Checkout(ctx, dir, branch); err != nil {
			return stageErr(name, "checkout branch", err)
		}
		if err := d.ensurePomModule(ctx, name, opts.Debug); err != nil {
			return stageErr(name, "ensure aggregator pom module", err)
		}
	}

	// 基础模块用 HEAD 缓存判断是否需要重新 install，减少无意义构建。
	for _, dep := range module.Dependencies {
		if err := lease.Check(); err != nil {
			return stageErr(moduleName, "superseded", err)
		}
		commit, err := gitops.HeadCommit(ctx, d.Output, d.moduleDir(dep))
		if err != nil {
			return stageErr(dep, "read git HEAD", err)
		}
		if !d.Cache.Changed(dep, branch, commit) {
			if opts.Debug {
				fmt.Printf("[cache] %s@%s unchanged (%s), skip install\n", dep, branch, commit)
			}
			continue
		}
		if err := d.withMavenLock(ctx, func() error {
			cmd := maven.BuildInstallCommand(d.Config.BuildRoot, dep, true, d.mavenOptions(opts.Env))
			return d.Runner.Run(ctx, cmd)
		}); err != nil {
			return stageErr(dep, "maven install dependency", err)
		}
		d.Cache.Update(dep, branch, commit)
		if err := d.Cache.Save(); err != nil {
			return stageErr(dep, "save cache", err)
		}
	}

	if err := lease.Check(); err != nil {
		return stageErr(moduleName, "superseded", err)
	}
	if err := d.withMavenLock(ctx, func() error {
		// 主模块始终构建，确保本次发布产物来自当前分支最新代码。
		cmd := maven.BuildInstallCommand(d.Config.BuildRoot, moduleName, false, d.mavenOptions(opts.Env))
		return d.Runner.Run(ctx, cmd)
	}); err != nil {
		return stageErr(moduleName, "maven install module", err)
	}

	artifact, err := packagex.FindArtifact(d.moduleDir(moduleName), module.Packaging)
	if err != nil {
		return stageErr(moduleName, "find artifact", err)
	}
	staging, err := packagex.PrepareStaging(artifact, d.Config.StagingDir, moduleName)
	if err != nil {
		return stageErr(moduleName, "prepare staging", err)
	}

	if err := lease.Check(); err != nil {
		return stageErr(moduleName, "superseded", err)
	}
	syncCmd := rsync.BuildCommand(staging, module.RemotePath, rsync.Options{
		Executable: d.Config.Rsync.Executable,
		Options:    d.Config.Rsync.Options,
		SSH:        d.Config.SSH,
	})
	if err := d.Runner.Run(ctx, syncCmd); err != nil {
		return stageErr(moduleName, "rsync to remote", err)
	}

	if module.RemoteScript != "" {
		// 如果配置了自定义远端脚本，优先交给脚本处理重启、通知等特殊动作。
		if err := d.Runner.Run(ctx, remote.ScriptCommandWithUser(d.Config.SSH, module.RemoteScript, opts.Operator)); err != nil {
			return stageErr(moduleName, "run remote script", err)
		}
	} else if module.Container != "" {
		if err := d.Runner.Run(ctx, remote.DockerRestartCommand(d.Config.SSH, module.Container)); err != nil {
			return stageErr(moduleName, "restart container", err)
		}
	}

	if err := d.monitorStartup(ctx, module, opts); err != nil {
		return stageErr(moduleName, "monitor startup", err)
	}
	return nil
}

func (d *Deployer) withMavenLock(ctx context.Context, fn func() error) error {
	// Maven install 会写本地仓库；并发写同一个 .m2 容易互相影响，所以统一串行化。
	lease, err := d.Locks.AcquireExclusive(ctx, "maven-install", 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer func(lease *lock.FileLock) {
		_ = lease.Release()
	}(lease)
	return fn()
}

func (d *Deployer) ensurePomModule(ctx context.Context, module string, debug bool) error {
	// 多个部署进程可能同时发现新模块，POM 更新必须加锁避免互相覆盖。
	lease, err := d.Locks.AcquireExclusive(ctx, "pom-modules", 200*time.Millisecond)
	if err != nil {
		return err
	}
	defer func(lease *lock.FileLock) {
		_ = lease.Release()
	}(lease)
	changed, err := pomxml.EnsureModule(filepath.Join(d.Config.BuildRoot, "pom.xml"), module)
	if err != nil {
		return err
	}
	if changed && debug {
		fmt.Printf("[pom] appended missing module %s to %s\n", module, filepath.Join(d.Config.BuildRoot, "pom.xml"))
	}
	return nil
}

func (d *Deployer) moduleDir(module string) string {
	return filepath.Join(d.Config.BuildRoot, module)
}

func (d *Deployer) mavenOptions(env string) maven.Options {
	envConfig := d.Config.Environments[env]
	return maven.Options{
		Executable: d.Config.Maven.Executable,
		Settings:   d.Config.Maven.Settings,
		LocalRepo:  d.Config.Maven.LocalRepo,
		Profile:    envConfig.MavenProfile,
		ExtraArgs:  d.Config.Maven.ExtraArgs,
		JavaHome:   d.Config.JDK.JavaHome,
	}
}

func (d *Deployer) monitorStartup(ctx context.Context, module config.Module, opts Options) error {
	if module.HealthURL == "" {
		if module.LogFile == "" {
			return nil
		}
		return d.Runner.Run(ctx, remote.TailCommand(d.Config.SSH, module.LogFile, opts.TailLines))
	}

	fmt.Printf("Waiting for application startup. healthUrl: %q, healthTimeout: %v\n", module.HealthURL, module.HealthTimeout)
	healthErr := waitForHealth(ctx, module.HealthURL, module.HealthTimeout)

	if module.LogFile == "" {
		if healthErr != nil {
			fmt.Printf("Application status is unknown. Please check startup logs or verify the health check URL configuration. healthUrl: %q, healthTimeout: %v\n",
				module.HealthURL, module.HealthTimeout)
			return nil
		}
		fmt.Println("Application started successfully, but logFile is not configured. Please check the server manually if startup logs are needed.")
		return nil
	}

	if healthErr != nil {
		fmt.Printf("Application status is unknown. Please check startup logs or verify the health check URL configuration. healthUrl: %q, healthTimeout: %v\n",
			module.HealthURL, module.HealthTimeout)
	} else {
		fmt.Println("Application started successfully.")
	}
	return d.Runner.Run(ctx, remote.TailCommand(d.Config.SSH, module.LogFile, opts.TailLines))
}

func waitForHealth(ctx context.Context, healthURL string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if healthOK(ctx, healthURL) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check timed out after %s", timeout)
		case <-ticker.C:
		}
	}
}

func healthOK(ctx context.Context, healthURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	return resp.StatusCode == http.StatusOK
}

func stageErr(module, stage string, err error) error {
	return fmt.Errorf("[%s] %s failed: %w", module, stage, err)
}

func EnsureDirs(cfg *config.Config) error {
	// 启动时先创建本地目录，后续步骤失败时就能更聚焦在 git/mvn/ssh 等真实问题上。
	for _, dir := range []string{cfg.Workspace, cfg.BuildRoot, cfg.StagingDir, cfg.LockDir, filepath.Dir(cfg.CacheFile)} {
		if dir == "" || dir == "." {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
