package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Command struct {
	Name           string
	Args           []string
	Dir            string
	Env            map[string]string
	SuppressStdout bool
}

func (c Command) String() string {
	return strings.TrimSpace(c.Name + " " + strings.Join(c.Args, " "))
}

type Runner interface {
	Run(ctx context.Context, cmd Command) error
}

type ExecRunner struct {
	Stdout io.Writer
	Stderr io.Writer
	DryRun bool
	Debug  bool
}

func (r ExecRunner) Run(ctx context.Context, spec Command) error {
	if spec.Name == "" {
		return errors.New("command name is required")
	}
	out := r.Stdout
	if out == nil {
		out = os.Stdout
	}
	errOut := r.Stderr
	if errOut == nil {
		errOut = os.Stderr
	}
	if r.Debug || r.DryRun {
		_, _ = fmt.Fprintf(out, "[cmd] %s\n", commandLog(spec))
	}
	if r.DryRun {
		// dry-run 用于在部署机上先确认 git/mvn/rsync/ssh 命令是否符合预期。
		return nil
	}
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	// 如果配置抑制输出则进行标准输出的丢弃动作
	if spec.SuppressStdout {
		cmd.Stdout = io.Discard
	} else {
		cmd.Stdout = out
	}
	cmd.Stderr = errOut
	cmd.Env = os.Environ()
	for key, value := range spec.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s: %w", encryptedCommandLabel(spec), err)
	}
	return nil
}

func commandLog(spec Command) string {
	if spec.Dir == "" {
		return spec.String()
	}
	return fmt.Sprintf("(dir=%s) %s", spec.Dir, spec.String())
}

func (r ExecRunner) Output(ctx context.Context, spec Command) (string, error) {
	if spec.Name == "" {
		return "", errors.New("command name is required")
	}
	if r.DryRun {
		// dry-run 下没有真实 HEAD，调用方会按空输出继续走命令预览。
		return "", nil
	}
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	for key, value := range spec.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command failed: %s: %w: %s", encryptedCommandLabel(spec), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
