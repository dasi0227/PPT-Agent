package commandexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	stdoutLimit    = 64 << 10
	stderrLimit    = 16 << 10
)

var approvedBinaryDirs = []string{
	"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin",
}

type Executor struct {
	root      string
	inventory map[string]string
	timeout   time.Duration
}

func NewExecutor(projectRoot string) (*Executor, error) {
	guard, err := NewPathGuard(projectRoot)
	if err != nil {
		return nil, err
	}
	inventory := map[string]string{}
	for _, name := range []string{"ls", "cat", "tail", "head", "find", "grep", "jq", "rg", "pwd", "stat", "sed", "wc", "git"} {
		for _, dir := range approvedBinaryDirs {
			path := filepath.Join(dir, name)
			if info, statErr := os.Stat(path); statErr == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
				inventory[name] = path
				break
			}
		}
	}
	return &Executor{root: guard.Root(), inventory: inventory, timeout: defaultTimeout}, nil
}

func (e *Executor) Available() []string {
	out := make([]string, 0, len(e.inventory))
	for _, name := range []string{"ls", "cat", "tail", "head", "find", "grep", "jq", "rg", "pwd", "stat", "sed", "wc", "git"} {
		if e.inventory[name] != "" {
			out = append(out, name)
		}
	}
	return out
}

func (e *Executor) Execute(ctx context.Context, graph Graph) (Result, error) {
	started := time.Now()
	var combinedOut, combinedErr strings.Builder
	exitCode := 0
	truncated := false
	for _, pipeline := range graph.Groups {
		result, err := e.executePipeline(ctx, pipeline)
		result.Stdout = e.sanitizeOutput(result.Stdout)
		result.Stderr = e.sanitizeOutput(result.Stderr)
		combinedOut.WriteString(result.Stdout)
		combinedErr.WriteString(result.Stderr)
		exitCode = result.ExitCode
		truncated = truncated || result.OutputTruncated
		if err != nil {
			result.Stdout = combinedOut.String()
			result.Stderr = combinedErr.String()
			result.Duration = time.Since(started)
			result.OutputTruncated = truncated
			return result, err
		}
	}
	return Result{
		Stdout: combinedOut.String(), Stderr: combinedErr.String(), ExitCode: exitCode,
		Duration: time.Since(started), OutputTruncated: truncated,
	}, nil
}

func (e *Executor) ExecuteSedBytes(ctx context.Context, command Command, content []byte) ([]byte, Result, error) {
	if len(command.Args) != 5 || command.Args[0] != "sed" || command.Args[1] != "-i" {
		return nil, Result{}, commandError(CodeInvariantViolation, "invalid staged sed command")
	}
	tempDir, err := os.MkdirTemp("", "ppt-agent-command-*")
	if err != nil {
		return nil, Result{}, commandError(CodeExecFailed, err.Error())
	}
	defer os.RemoveAll(tempDir)
	target := filepath.Join(tempDir, "target")
	if err := os.WriteFile(target, content, 0o600); err != nil {
		return nil, Result{}, commandError(CodeExecFailed, err.Error())
	}
	isolated := Command{Args: append([]string(nil), command.Args...)}
	isolated.Args[4] = target
	result, err := e.Execute(ctx, Graph{Groups: []Pipeline{{Commands: []Command{isolated}}}})
	if err != nil {
		return nil, result, err
	}
	updated, err := os.ReadFile(target)
	if err != nil {
		return nil, result, commandError(CodeExecFailed, err.Error())
	}
	return updated, result, nil
}

func (e *Executor) executePipeline(parent context.Context, pipeline Pipeline) (Result, error) {
	ctx, cancel := context.WithTimeout(parent, e.timeout)
	defer cancel()
	home, err := os.MkdirTemp("", "ppt-agent-command-home-*")
	if err != nil {
		return Result{}, commandError(CodeExecFailed, err.Error())
	}
	defer os.RemoveAll(home)

	var stdout, stderr cappedBuffer
	stdout.limit = stdoutLimit
	stderr.limit = stderrLimit
	outputLimitReached := make(chan struct{})
	var outputLimitOnce sync.Once
	onOutputLimit := func() {
		outputLimitOnce.Do(func() { close(outputLimitReached) })
	}
	stdout.onLimit = onOutputLimit
	stderr.onLimit = onOutputLimit
	commands := make([]*exec.Cmd, len(pipeline.Commands))
	pipeEnds := make([]*os.File, 0, (len(pipeline.Commands)-1)*2)
	var previousReader *os.File
	for index, command := range pipeline.Commands {
		binary := e.inventory[command.Args[0]]
		if binary == "" {
			closePipeEnds(pipeEnds)
			return Result{}, commandError(CodeNotAvailable, command.Args[0]+" is not available on this host")
		}
		cmd := exec.Command(binary, command.Args[1:]...)
		cmd.Dir = e.root
		cmd.Env = e.environment(home, command.Args[0])
		if previousReader != nil {
			cmd.Stdin = previousReader
		}
		if index == len(pipeline.Commands)-1 {
			cmd.Stdout = &stdout
		} else {
			reader, writer, pipeErr := os.Pipe()
			if pipeErr != nil {
				closePipeEnds(pipeEnds)
				return Result{}, commandError(CodeExecFailed, pipeErr.Error())
			}
			cmd.Stdout = writer
			pipeEnds = append(pipeEnds, reader, writer)
			previousReader = reader
		}
		cmd.Stderr = &stderr
		commands[index] = cmd
	}

	started := time.Now()
	startedCommands := []*exec.Cmd{}
	for index, cmd := range commands {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if index > 0 {
			cmd.SysProcAttr.Pgid = commands[0].Process.Pid
		}
		if err := cmd.Start(); err != nil {
			killCommands(startedCommands)
			closePipeEnds(pipeEnds)
			return Result{Duration: time.Since(started)}, commandError(CodeExecFailed, err.Error())
		}
		startedCommands = append(startedCommands, cmd)
	}
	closePipeEnds(pipeEnds)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killCommands(startedCommands)
		case <-outputLimitReached:
			killCommands(startedCommands)
		case <-done:
		}
	}()

	waitErrors := make([]error, len(commands))
	for index, cmd := range commands {
		waitErrors[index] = cmd.Wait()
	}
	close(done)
	result := Result{
		Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started),
		OutputTruncated: stdout.truncated || stderr.truncated,
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.ExitCode = -1
		return result, commandError(CodeTimeout, "command exceeded the 10 second limit")
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(parent.Err(), context.Canceled) {
		result.ExitCode = -1
		return result, context.Canceled
	}
	if result.OutputTruncated {
		result.ExitCode = -1
		return result, commandError(CodeOutputLimit, "command output exceeded the configured limit")
	}
	if waitErr := waitErrors[len(waitErrors)-1]; waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			return result, commandError(CodeExitNonzero, fmt.Sprintf("command exited with code %d", result.ExitCode))
		}
		return result, commandError(CodeExecFailed, waitErr.Error())
	}
	return result, nil
}

func (e *Executor) sanitizeOutput(value string) string {
	return strings.ReplaceAll(value, e.root, ".")
}

func closePipeEnds(pipeEnds []*os.File) {
	for _, pipe := range pipeEnds {
		_ = pipe.Close()
	}
}

func (e *Executor) environment(home, command string) []string {
	env := []string{
		"PATH=" + strings.Join(approvedBinaryDirs, string(os.PathListSeparator)),
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + home,
		"RIPGREP_CONFIG_PATH=",
		"LC_ALL=C",
		"LANG=C",
		"TERM=dumb",
		"NO_COLOR=1",
	}
	if command == "git" {
		emptyConfig := filepath.Join(home, "gitconfig")
		_ = os.WriteFile(emptyConfig, nil, 0o600)
		env = append(env,
			"GIT_OPTIONAL_LOCKS=0",
			"GIT_PAGER=cat",
			"PAGER=cat",
			"GIT_EXTERNAL_DIFF=",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_GLOBAL="+emptyConfig,
		)
	}
	return env
}

func killCommands(commands []*exec.Cmd) {
	for _, command := range commands {
		if command == nil || command.Process == nil {
			continue
		}
		if pgid, err := syscall.Getpgid(command.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = command.Process.Kill()
		}
	}
}

type cappedBuffer struct {
	mu        sync.Mutex
	data      bytes.Buffer
	limit     int
	truncated bool
	onLimit   func()
}

func (b *cappedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.limit - b.data.Len()
	if remaining > 0 {
		if len(value) < remaining {
			remaining = len(value)
		}
		_, _ = b.data.Write(value[:remaining])
	}
	if remaining < len(value) {
		if !b.truncated {
			b.truncated = true
			if b.onLimit != nil {
				b.onLimit()
			}
		}
	}
	return len(value), nil
}

func (b *cappedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.ToValidUTF8(b.data.String(), "\uFFFD")
}

func (r Result) DurationMS() int64 {
	return r.Duration.Milliseconds()
}

func (r Result) ExitCodeText() string {
	return strconv.Itoa(r.ExitCode)
}
