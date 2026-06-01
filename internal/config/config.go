package config

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Workspace 是 agent 的总工作目录；BuildRoot 通常是包含聚合 pom.xml 的 Maven 根目录。
	Workspace    string               `yaml:"workspace"`
	BuildRoot    string               `yaml:"buildRoot"`
	StagingDir   string               `yaml:"stagingDir"`
	CacheFile    string               `yaml:"cacheFile"`
	LockDir      string               `yaml:"lockDir"`
	Maven        MavenConfig          `yaml:"maven"`
	JDK          JDKConfig            `yaml:"jdk"`
	SSH          SSHConfig            `yaml:"ssh"`
	Rsync        RsyncConfig          `yaml:"rsync"`
	Environments map[string]EnvConfig `yaml:"environments"`
	Modules      map[string]Module    `yaml:"modules"`
}

type MavenConfig struct {
	Executable string   `yaml:"executable"`
	Settings   string   `yaml:"settings"`
	LocalRepo  string   `yaml:"localRepo"`
	ExtraArgs  []string `yaml:"extraArgs"`
}

type JDKConfig struct {
	JavaHome string `yaml:"javaHome"`
}

type SSHConfig struct {
	User    string `yaml:"user"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	KeyFile string `yaml:"keyFile"`
}

type RsyncConfig struct {
	Executable string   `yaml:"executable"`
	Options    []string `yaml:"options"`
}

type EnvConfig struct {
	Branch string `yaml:"branch"`
}

type Module struct {
	Repo         string   `yaml:"repo"`
	Packaging    string   `yaml:"packaging"`
	Dependencies []string `yaml:"dependencies"`
	RemotePath   string   `yaml:"remotePath"`
	Container    string   `yaml:"container"`
	LogFile      string   `yaml:"logFile"`
	RemoteScript string   `yaml:"remoteScript"`
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	// 默认值尽量贴近部署机常见命令名，让配置文件只关注差异化参数。
	if cfg.Maven.Executable == "" {
		cfg.Maven.Executable = "mvn"
	}
	if cfg.Rsync.Executable == "" {
		cfg.Rsync.Executable = "rsync"
	}
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}
	if cfg.LockDir == "" && cfg.Workspace != "" {
		cfg.LockDir = cfg.Workspace + "/locks"
	}
	if cfg.BuildRoot == "" {
		cfg.BuildRoot = cfg.Workspace
	}
	if cfg.StagingDir == "" && cfg.Workspace != "" {
		cfg.StagingDir = cfg.Workspace + "/staging"
	}
	for name, module := range cfg.Modules {
		if module.Packaging == "" {
			module.Packaging = "war"
		}
		module.Packaging = strings.ToLower(module.Packaging)
		cfg.Modules[name] = module
	}
}

func (c *Config) Validate() error {
	// 这里一次性收集所有配置问题，方便 Spug 日志里直接看到完整缺项列表。
	var problems []string
	if c.Workspace == "" {
		problems = append(problems, "workspace is required")
	}
	if c.CacheFile == "" {
		problems = append(problems, "cacheFile is required")
	}
	if c.LockDir == "" {
		problems = append(problems, "lockDir is required")
	}
	if c.Maven.Executable == "" {
		problems = append(problems, "maven.executable is required")
	}
	if c.SSH.User == "" || c.SSH.Host == "" {
		problems = append(problems, "ssh.user and ssh.host are required")
	}
	if len(c.Environments) == 0 {
		problems = append(problems, "at least one environment is required")
	}
	for name, env := range c.Environments {
		if env.Branch == "" {
			problems = append(problems, fmt.Sprintf("environments.%s.branch is required", name))
		}
	}
	if len(c.Modules) == 0 {
		problems = append(problems, "at least one module is required")
	}
	for name, module := range c.Modules {
		// 依赖模块可以只参与构建缓存，不一定需要远端部署路径；真正被部署的模块会在 deploy 阶段再校验 remotePath/container。
		if module.Repo == "" {
			problems = append(problems, fmt.Sprintf("modules.%s.repo is required", name))
		}
		if module.Packaging != "war" && module.Packaging != "jar" {
			problems = append(problems, fmt.Sprintf("modules.%s.packaging must be war or jar", name))
		}
		for _, dep := range module.Dependencies {
			if _, ok := c.Modules[dep]; !ok {
				problems = append(problems, fmt.Sprintf("modules.%s.dependencies references unknown module %s", name, dep))
			}
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (c *Config) BranchForEnv(env string) (string, error) {
	item, ok := c.Environments[env]
	if !ok {
		return "", fmt.Errorf("environment %q is not configured", env)
	}
	return item.Branch, nil
}

func (c *Config) ModuleNames(csv string) ([]string, error) {
	var names []string
	for _, raw := range strings.Split(csv, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := c.Modules[name]; !ok {
			return nil, fmt.Errorf("module %q is not configured", name)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, errors.New("at least one module must be provided")
	}
	return names, nil
}

func (c *Config) SortedDependencies(moduleName string) ([]string, error) {
	module, ok := c.Modules[moduleName]
	if !ok {
		return nil, fmt.Errorf("module %q is not configured", moduleName)
	}
	deps := append([]string(nil), module.Dependencies...)
	sort.Strings(deps)
	return deps, nil
}
