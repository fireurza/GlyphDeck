//go:build !windows

package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// pipeSession is the non-Windows fallback: connects the shell via OS pipes.
// Resize is a no-op because pipes carry no terminal geometry signal.
type pipeSession struct {
	cmd     *exec.Cmd
	stdinW  io.WriteCloser
	stdoutR io.ReadCloser
}

// newTermSession launches the shell and returns a pipe-backed session.
func newTermSession(shellPath string, shellArgs []string, cwd string) (termSession, error) {
	cmd := exec.Command(shellPath, shellArgs...)
	cmd.Dir = cwd
	cmd.Env = os.Environ()

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	// Merge stderr into the stdout pipe so the caller sees a single stream.
	// cmd.Stdout is the pipe write end set by StdoutPipe(); assigning cmd.Stderr
	// to the same value routes both streams through one pipe reader.
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start shell: %w", err)
	}
	return &pipeSession{cmd: cmd, stdinW: stdinPipe, stdoutR: stdoutPipe}, nil
}

func (s *pipeSession) stdin() io.WriteCloser    { return s.stdinW }
func (s *pipeSession) stdout() io.ReadCloser    { return s.stdoutR }
func (s *pipeSession) resize(_, _ uint16) error { return nil }
func (s *pipeSession) process() *os.Process     { return s.cmd.Process }

func (s *pipeSession) wait() error {
	return s.cmd.Wait()
}

func (s *pipeSession) close() error {
	_ = s.stdinW.Close()
	return nil
}
