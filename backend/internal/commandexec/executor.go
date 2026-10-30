package commandexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

// Runner is a transport/lifecycle adapter. OS isolation and permission mapping
// belong to command-worker and its unmodified sandbox-runtime dependency.
type Runner struct {
	settings                       config.SandboxConfig
	node, worker, home, searchPath string
	protectedReads                 []string
	electron                       bool
	fingerprint                    string
	initErr                        error
}

func NewRunner(settings config.SandboxConfig, protectedReads ...string) *Runner {
	settings.ReadRoots = slices.Clone(settings.ReadRoots)
	settings.Network.AllowedDomains = slices.Clone(settings.Network.AllowedDomains)
	r := &Runner{settings: settings, electron: os.Getenv("PPT_COMMAND_ELECTRON") == "1"}
	r.initErr = config.ValidateSandboxConfig(settings)
	for i, path := range settings.ReadRoots {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			r.initErr = err
			continue
		}
		settings.ReadRoots[i] = resolved
	}
	if err := config.ValidateSandboxConfig(settings); err != nil {
		r.initErr = err
	}
	r.settings = settings
	var homeErr error
	r.home, homeErr = os.UserHomeDir()
	if homeErr != nil || !filepath.IsAbs(r.home) {
		r.initErr = errors.New("cannot resolve host home directory")
	}
	r.node = os.Getenv("PPT_COMMAND_NODE")
	if r.node == "" {
		r.node = "node"
	}
	if resolved, err := exec.LookPath(r.node); err == nil {
		r.node, _ = filepath.Abs(resolved)
	} else if r.initErr == nil {
		r.initErr = err
	}
	r.worker = os.Getenv("PPT_COMMAND_WORKER")
	if r.worker == "" {
		// Test packages start below backend; server starts in backend or app data.
		_, source, _, _ := runtime.Caller(0)
		for _, path := range []string{"command-worker/worker.mjs", "backend/command-worker/worker.mjs", filepath.Join(filepath.Dir(source), "../../command-worker/worker.mjs")} {
			if _, err := os.Stat(path); err == nil {
				r.worker = path
				break
			}
		}
	}
	r.worker, _ = filepath.Abs(r.worker)
	for _, path := range protectedReads {
		if path == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			r.initErr = err
			continue
		}
		r.protectedReads = append(r.protectedReads, absolute)
		if resolved, err := filepath.EvalSymlinks(absolute); err == nil && resolved != absolute {
			r.protectedReads = append(r.protectedReads, resolved)
		}
	}
	dirs := []string{}
	for _, path := range append(filepath.SplitList(os.Getenv("PATH")), "/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin") {
		if filepath.IsAbs(path) && !slices.Contains(dirs, path) {
			dirs = append(dirs, path)
		}
	}
	r.searchPath = strings.Join(dirs, ":")
	policy, _ := json.Marshal(struct {
		Settings                           config.SandboxConfig
		Protected                          []string
		Node, Worker, Home, Path, Platform string
		Electron                           bool
	}{settings, r.protectedReads, r.node, r.worker, r.home, r.searchPath, runtime.GOOS, r.electron})
	// Upgrading the worker or lockfile also invalidates pending approvals.
	for _, name := range []string{"worker.mjs", "policy.mjs", "package.json", "package-lock.json"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(r.worker), name))
		if err != nil {
			r.initErr = err
		}
		policy = append(policy, []byte("\x00"+name+"\x00"+ContentHash(raw))...)
	}
	r.fingerprint = ContentHash(policy)
	return r
}

func (r *Runner) effective() *Runner {
	if r == nil {
		return NewRunner(config.DefaultSandboxConfig())
	}
	return r
}

func (r *Runner) PolicyHash(originalRoot string, writable bool) string {
	r = r.effective()
	raw, _ := json.Marshal([]any{PolicyVersion, r.fingerprint, filepath.Clean(originalRoot), writable})
	return ContentHash(raw)
}

// Execute receives a disposable copy. It never launches a bare shell or retries
// a failed worker without its sandbox.
func (r *Runner) Execute(parent context.Context, workspace, originalRoot string, writable bool, d Decision) (Result, error) {
	r = r.effective()
	if runtime.GOOS != "darwin" {
		return Result{}, commandError(CodeNotAvailable, "Sandboxed commands currently require macOS.")
	}
	if r.initErr != nil {
		return Result{}, commandError(CodeNotAvailable, "Command worker is unavailable: "+r.initErr.Error())
	}
	scratch, err := os.MkdirTemp("", "ppt-agent-command-state-*")
	if err != nil {
		return Result{}, err
	}
	defer RemoveWorkspace(scratch)
	scratch, err = filepath.EvalSymlinks(scratch)
	if err != nil {
		return Result{}, err
	}
	request := struct {
		Protocol           int                  `json:"protocol"`
		Command            string               `json:"command"`
		Workspace          string               `json:"workspace"`
		OriginalRoot       string               `json:"original_root"`
		Scratch            string               `json:"scratch"`
		HostHome           string               `json:"host_home"`
		ProtectedReadPaths []string             `json:"protected_read_paths"`
		Writable           bool                 `json:"writable"`
		Settings           config.SandboxConfig `json:"settings"`
	}{1, d.Source, workspace, originalRoot, scratch, r.home, r.protectedReads, writable, r.settings}
	raw, err := json.Marshal(request)
	if err != nil {
		return Result{}, err
	}
	// Worker owns the deadline. This outer bound also covers a stuck worker.
	ctx, cancel := context.WithTimeout(parent, time.Duration(r.settings.Execution.TimeoutSeconds)*time.Second+3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.node, r.worker)
	cmd.Dir = workspace
	cmd.Env = []string{"PATH=" + r.searchPath, "HOME=" + scratch, "TMPDIR=" + scratch, "LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8", "TERM=dumb", "NO_COLOR=1"}
	if r.electron {
		cmd.Env = append(cmd.Env, "ELECTRON_RUN_AS_NODE=1")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	killGroup := func(signal syscall.Signal) {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, signal)
		}
	}
	cmd.Cancel = func() error { killGroup(syscall.SIGTERM); return nil }
	cmd.WaitDelay = 2 * time.Second
	// Bound protocol expansion and upstream diagnostics independently.
	stdout := &boundedOutput{limit: r.settings.Execution.MaxOutputBytes*2 + 64*1024, stop: func() { killGroup(syscall.SIGKILL) }}
	stderr := &boundedOutput{limit: 64 * 1024, stop: func() { killGroup(syscall.SIGKILL) }}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Result{}, err
	}
	defer stdin.Close()
	started := time.Now()
	if err := cmd.Start(); err != nil {
		return Result{}, commandError(CodeNotAvailable, err.Error())
	}
	defer killGroup(syscall.SIGKILL)
	// Keep stdin open: EOF tells the worker its server died.
	if _, err := stdin.Write(append(raw, '\n')); err != nil {
		killGroup(syscall.SIGKILL)
		_ = cmd.Wait()
		if parent.Err() != nil {
			return Result{}, parent.Err()
		}
		return Result{}, commandError(CodeNotAvailable, "Cannot send command to worker.")
	}
	waitErr := cmd.Wait()
	result := Result{ExitCode: -1, Duration: time.Since(started)}
	if parent.Err() != nil {
		return result, parent.Err()
	}
	if ctx.Err() != nil {
		return result, commandError(CodeTimeout, "Command worker exceeded its execution deadline.")
	}
	if stdout.exceeded || stderr.exceeded {
		return result, commandError(CodeOutputLimit, "Command worker exceeded its output limit.")
	}
	var response struct {
		Protocol       int `json:"protocol"`
		Stdout, Stderr []byte
		ExitCode       int `json:"exit_code"`
		Code, Message  string
	}
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || response.Protocol != 1 || len(response.Stdout)+len(response.Stderr) > r.settings.Execution.MaxOutputBytes {
		return result, commandError(CodeNotAvailable, "Command worker returned no valid result. "+strings.TrimSpace(stderr.String()))
	}
	// Workers kill their own group after flushing a terminal record.
	var exit *exec.ExitError
	if waitErr != nil && !(errors.As(waitErr, &exit) && exit.ProcessState.Sys().(syscall.WaitStatus).Signal() == syscall.SIGKILL) {
		return result, commandError(CodeNotAvailable, "Command worker terminated unexpectedly.")
	}
	result.Stdout, result.Stderr = strings.ToValidUTF8(string(response.Stdout), "\uFFFD"), strings.ToValidUTF8(string(response.Stderr), "\uFFFD")
	result.ExitCode = response.ExitCode
	if response.Code != "" {
		return result, commandError(response.Code, response.Message)
	}
	if response.ExitCode != 0 {
		return result, commandError(CodeExitNonzero, "Command exited unsuccessfully.")
	}
	return result, nil
}

// Each stream has one os/exec copy goroutine; read only after Wait.
type boundedOutput struct {
	data     bytes.Buffer
	limit    int
	exceeded bool
	stop     func()
}

func (b *boundedOutput) Write(value []byte) (int, error) {
	n := len(value)
	remaining := b.limit - b.data.Len()
	if n > remaining {
		value = value[:remaining]
		b.exceeded = true
		b.stop()
	}
	_, _ = b.data.Write(value)
	return n, nil
}

func (b *boundedOutput) Bytes() []byte  { return b.data.Bytes() }
func (b *boundedOutput) String() string { return b.data.String() }
