package commandexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultTimeout = 2 * time.Minute

var fallbackBinaryDirs = []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/usr/local/bin"}

type Executor struct {
	root    string
	timeout time.Duration
}

func NewExecutor(root string) (*Executor, error) {
	if runtime.GOOS != "darwin" {
		return nil, commandError(CodeNotAvailable, "Sandboxed shell execution is supported on macOS only.")
	}
	for _, path := range []string{"/usr/bin/sandbox-exec", "/bin/bash"} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			return nil, commandError(CodeNotAvailable, "Required shell sandbox is unavailable: "+path)
		}
	}
	return &Executor{root: root, timeout: defaultTimeout}, nil
}

// Execute receives an isolated copy. No command is ever launched against the live project.
func (e *Executor) Execute(parent context.Context, d Decision, protected []string) (Result, error) {
	scratch, err := os.MkdirTemp("", "ppt-agent-shell-state-*")
	if err != nil {
		return Result{}, err
	}
	defer RemoveWorkspace(scratch)
	scratch, err = filepath.EvalSymlinks(scratch)
	if err != nil {
		return Result{}, err
	}
	profile := sandboxProfile(e.root, scratch, d, protected)
	ctx, cancel := context.WithTimeout(parent, e.timeout)
	defer cancel()
	var stdout, stderr outputBuffer
	cmd := exec.CommandContext(ctx, "/usr/bin/sandbox-exec", "-p", profile, "/bin/bash", "--noprofile", "--norc", "-c", d.Source)
	cmd.Dir = e.root
	home, _ := os.UserHomeDir()
	cmd.Env = []string{
		"PATH=" + strings.Join(executableSearchDirs(), ":"), "HOME=" + home, "TMPDIR=" + scratch, "XDG_CONFIG_HOME=" + scratch, "XDG_CACHE_HOME=" + scratch,
		"npm_config_cache=" + scratch + "/npm", "PIP_CACHE_DIR=" + scratch + "/pip", "RIPGREP_CONFIG_PATH=", "LC_ALL=en_US.UTF-8", "LANG=en_US.UTF-8", "TERM=dumb", "NO_COLOR=1",
		"GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "PAGER=cat", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_COUNT=3", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0=false", "GIT_CONFIG_KEY_1=diff.external", "GIT_CONFIG_VALUE_1=", "GIT_CONFIG_KEY_2=core.hooksPath", "GIT_CONFIG_VALUE_2=/dev/null",
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = time.Second
	started := time.Now()
	err = cmd.Start()
	if err != nil {
		return Result{}, commandError(CodeNotAvailable, err.Error())
	}
	// Kill background members on every exit, including successful shells.
	defer syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	err = cmd.Wait()
	result := Result{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started)}
	if parent.Err() != nil {
		result.ExitCode = -1
		return result, parent.Err()
	}
	if ctx.Err() != nil {
		result.ExitCode = -1
		return result, commandError(CodeTimeout, "Command exceeded its execution deadline.")
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			result.ExitCode = exit.ExitCode()
		} else {
			result.ExitCode = -1
		}
		code := CodeExitNonzero
		if result.ExitCode == 126 || result.ExitCode == 127 {
			code = CodeNotAvailable
		}
		if strings.Contains(result.Stderr, "Operation not permitted") || strings.Contains(result.Stderr, "Permission denied") {
			code = CodeNotAllowed
		}
		if strings.HasPrefix(strings.TrimSpace(result.Stderr), "sandbox-exec:") {
			code = CodeNotAvailable
		}
		return result, commandError(code, fmt.Sprintf("Command exited with code %d.", result.ExitCode))
	}
	return result, nil
}

func sandboxProfile(root, scratch string, d Decision, protected []string) string {
	var p strings.Builder
	p.WriteString(`(version 1)
(deny default)
(deny file-link)
(allow process-fork)
(allow process-exec)
(allow signal (target same-sandbox))
(allow process-info* (target same-sandbox))
(allow sysctl-read)
(allow file-read-metadata)
(allow mach-lookup (global-name "com.apple.system.opendirectoryd.libinfo") (global-name "com.apple.system.logger") (global-name "com.apple.bsd.dirhelper"))
`)
	rule := func(operation, filter, path string) {
		fmt.Fprintf(&p, "(%s (%s %s))\n", operation, filter, strconv.Quote(path))
	}
	p.WriteString("(allow file-read*)\n")
	rule("allow file-write-data", "literal", "/dev/null")
	rule("allow file-read*", "subpath", root)
	rule("allow file-read* file-write*", "subpath", scratch)
	rule("allow file-write*", "subpath", root)
	if d.Network {
		p.WriteString(`(allow network-outbound (remote tcp) (remote udp))
(deny network-outbound (remote ip "localhost:*"))
(allow network-outbound (remote unix-socket "/private/var/run/mDNSResponder"))
(allow mach-lookup (global-name "com.apple.TrustEvaluationAgent") (global-name "com.apple.networkd") (global-name "com.apple.SystemConfiguration.configd"))
`)
	}
	for _, path := range append([]string{".git", ".runtime", ".run", ".commit-tmp", "versions", ".manifest.json", ".design.json", ".outline.json"}, protected...) {
		rule("deny file-write*", "subpath", filepath.Join(root, path))
	}
	rule("deny file-write-unlink file-write-mode", "literal", root)
	// Extended attributes are not part of the project transaction format.
	rule("deny file-write-xattr", "subpath", root)
	if d.Network {
		for _, dir := range executableSearchDirs() {
			candidate := filepath.Join(dir, "git")
			for _, path := range []string{candidate, resolveExecutable(candidate)} {
				if path != "" {
					rule("deny process-exec", "literal", path)
				}
			}
		}
		for _, dir := range []string{"/usr/libexec/git-core", "/Library/Developer/CommandLineTools/usr/libexec/git-core", "/Applications/Xcode.app/Contents/Developer/usr/libexec/git-core", "/opt/homebrew/opt/git", "/usr/local/opt/git"} {
			rule("deny process-exec", "subpath", dir)
			if resolved := resolveExecutable(dir); resolved != "" {
				rule("deny process-exec", "subpath", resolved)
			}
		}
	}
	return p.String()
}

func resolveExecutable(path string) string {
	resolved, _ := filepath.EvalSymlinks(path)
	return resolved
}

type outputBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *outputBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(value)
}
func (b *outputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.ToValidUTF8(b.data.String(), "\uFFFD")
}

func executableSearchDirs() []string {
	dirs := append(filepath.SplitList(os.Getenv("PATH")), fallbackBinaryDirs...)
	seen := map[string]bool{}
	out := []string{}
	for _, dir := range dirs {
		if dir != "" && !seen[dir] {
			seen[dir] = true
			out = append(out, dir)
		}
	}
	return out
}
