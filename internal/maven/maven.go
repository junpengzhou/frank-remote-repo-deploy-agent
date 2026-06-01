package maven

import "frank-remote-repo-deploy-agent/internal/runner"

type Options struct {
	Executable string
	Settings   string
	LocalRepo  string
	ExtraArgs  []string
	JavaHome   string
}

func BuildInstallCommand(workDir, module string, alsoMake bool, opts Options) runner.Command {
	name := opts.Executable
	if name == "" {
		name = "mvn"
	}
	args := []string{"clean", "install", "-Dmaven.test.skip=true", "-pl", module}
	if alsoMake {
		args = append(args, "-am")
	}
	if opts.Settings != "" {
		args = append(args, "-s", opts.Settings)
	}
	if opts.LocalRepo != "" {
		args = append(args, "-Dmaven.repo.local="+opts.LocalRepo)
	}
	args = append(args, opts.ExtraArgs...)
	env := map[string]string{}
	if opts.JavaHome != "" {
		env["JAVA_HOME"] = opts.JavaHome
	}
	return runner.Command{Name: name, Args: args, Dir: workDir, Env: env}
}
