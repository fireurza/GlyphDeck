package sandboxes

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// realSSHRunner executes commands over SSH using the `ssh` executable.
type realSSHRunner struct{}

func (r *realSSHRunner) Run(ctx context.Context, sshAlias, command string, timeout time.Duration) SSHResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", sshAlias, command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := SSHResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if ctx.Err() != nil {
			result.Err = fmt.Errorf("ssh command timed out after %v", timeout)
			return result
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Err = err
	}

	return result
}

// fakeSSHRunner is used in tests to simulate SSH command execution.
type fakeSSHRunner struct {
	results map[string]SSHResult // command -> result
}

func newFakeSSHRunner() *fakeSSHRunner {
	return &fakeSSHRunner{results: make(map[string]SSHResult)}
}

func (f *fakeSSHRunner) setResult(command string, result SSHResult) {
	f.results[command] = result
}

func (f *fakeSSHRunner) Run(_ context.Context, sshAlias, command string, _ time.Duration) SSHResult {
	if r, ok := f.results[command]; ok {
		return r
	}
	// Default: success with echo of command.
	return SSHResult{Stdout: command + "\n"}
}
