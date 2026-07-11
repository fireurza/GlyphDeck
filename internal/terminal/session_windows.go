//go:build windows

package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// defaultPTYCols and defaultPTYRows set the initial ConPTY window size.
const (
	defaultPTYCols = 220
	defaultPTYRows = 50
)

// pipeWriter wraps a raw Win32 HANDLE for blocking write via WriteFile.
type pipeWriter struct{ h windows.Handle }

func (w *pipeWriter) Write(b []byte) (int, error) {
	var n uint32
	err := windows.WriteFile(w.h, b, &n, nil)
	return int(n), err
}
func (w *pipeWriter) Close() error { return windows.CloseHandle(w.h) }

// pipeReader wraps a raw Win32 HANDLE for blocking read via ReadFile.
type pipeReader struct{ h windows.Handle }

func (r *pipeReader) Read(b []byte) (int, error) {
	var n uint32
	err := windows.ReadFile(r.h, b, &n, nil)
	if err == windows.ERROR_BROKEN_PIPE || err == windows.ERROR_HANDLE_EOF {
		return int(n), io.EOF
	}
	if err != nil {
		return int(n), err
	}
	return int(n), nil
}
func (r *pipeReader) Close() error { return windows.CloseHandle(r.h) }

// conPTYSession backs a terminal with a Windows Pseudo Console (ConPTY).
type conPTYSession struct {
	hpc       windows.Handle
	attrList  *windows.ProcThreadAttributeListContainer
	hproc     windows.Handle
	proc      *os.Process
	inWrite   *pipeWriter
	outRead   *pipeReader
	waitOnce  sync.Once
	waitDone  bool
}

func newTermSession(shellPath string, shellArgs []string, cwd string) (termSession, error) {
	var inReadPTY, inWrite windows.Handle
	if err := windows.CreatePipe(&inReadPTY, &inWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("create pty input pipe: %w", err)
	}
	var outRead, outWritePTY windows.Handle
	if err := windows.CreatePipe(&outRead, &outWritePTY, nil, 0); err != nil {
		windows.CloseHandle(inReadPTY)
		windows.CloseHandle(inWrite)
		return nil, fmt.Errorf("create pty output pipe: %w", err)
	}
	var hpc windows.Handle
	if err := windows.CreatePseudoConsole(
		windows.Coord{X: defaultPTYCols, Y: defaultPTYRows},
		inReadPTY, outWritePTY, 0, &hpc,
	); err != nil {
		windows.CloseHandle(inReadPTY)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		windows.CloseHandle(outWritePTY)
		return nil, fmt.Errorf("create pseudo console: %w", err)
	}
	windows.CloseHandle(inReadPTY)
	windows.CloseHandle(outWritePTY)

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		return nil, fmt.Errorf("build proc thread attribute list: %w", err)
	}
	if err := attrList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		unsafe.Pointer(&hpc), unsafe.Sizeof(hpc)); err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		return nil, fmt.Errorf("set pseudoconsole attribute: %w", err)
	}

	si := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
		},
		ProcThreadAttributeList: attrList.List(),
	}
	cmdLine16, err := windows.UTF16PtrFromString(buildWindowsCmdLine(shellPath, shellArgs))
	if err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		return nil, fmt.Errorf("encode command line: %w", err)
	}
	cwd16, err := windows.UTF16PtrFromString(cwd)
	if err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		return nil, fmt.Errorf("encode working directory: %w", err)
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(
		nil, cmdLine16, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.EXTENDED_STARTUPINFO_PRESENT,
		nil, cwd16, &si.StartupInfo, &pi,
	); err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(hpc)
		windows.CloseHandle(inWrite)
		windows.CloseHandle(outRead)
		return nil, fmt.Errorf("create process: %w", err)
	}
	windows.CloseHandle(pi.Thread)

	inWritePipe := &pipeWriter{h: inWrite}
	outReadPipe := &pipeReader{h: outRead}
	proc, err := os.FindProcess(int(pi.ProcessId))
	if err != nil {
		windows.TerminateProcess(pi.Process, 1)
		windows.CloseHandle(pi.Process)
		inWritePipe.Close()
		outReadPipe.Close()
		attrList.Delete()
		windows.ClosePseudoConsole(hpc)
		return nil, fmt.Errorf("find process %d: %w", pi.ProcessId, err)
	}
	return &conPTYSession{
		hpc: hpc, attrList: attrList, hproc: pi.Process,
		proc: proc, inWrite: inWritePipe, outRead: outReadPipe,
	}, nil
}

func (s *conPTYSession) stdin() io.WriteCloser { return s.inWrite }
func (s *conPTYSession) stdout() io.ReadCloser { return s.outRead }
func (s *conPTYSession) process() *os.Process  { return s.proc }

func (s *conPTYSession) resize(rows, cols uint16) error {
	if err := windows.ResizePseudoConsole(s.hpc, windows.Coord{X: int16(cols), Y: int16(rows)}); err != nil {
		return fmt.Errorf("resize pseudo console: %w", err)
	}
	return nil
}

func (s *conPTYSession) close() error { return s.inWrite.Close() }

func (s *conPTYSession) wait() error {
	s.waitOnce.Do(func() {
		_, werr := windows.WaitForSingleObject(s.hproc, windows.INFINITE)
		windows.ClosePseudoConsole(s.hpc)
		_ = s.outRead.Close()
		s.attrList.Delete()
		windows.CloseHandle(s.hproc)
		s.waitDone = true
		if werr != nil {
			// Store error but don't return from Do.
		}
	})
	if s.waitDone {
		return nil
	}
	return fmt.Errorf("wait already in progress")
}

func buildWindowsCmdLine(path string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, windowsQuoteArg(path))
	for _, a := range args {
		parts = append(parts, windowsQuoteArg(a))
	}
	return strings.Join(parts, " ")
}

func windowsQuoteArg(s string) string {
	if s == "" {
		return `""`
	}
	if !strings.ContainsAny(s, " \t\n\"") {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range s {
		switch c {
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteString(`\"`)
		default:
			slashes = 0
			b.WriteRune(c)
		}
	}
	for ; slashes > 0; slashes-- {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return b.String()
}
