package commandexec

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxEditableFileBytes = 1 << 20

type Policy struct {
	guard *PathGuard
}

func NewPolicy(projectRoot string) (*Policy, error) {
	guard, err := NewPathGuard(projectRoot)
	if err != nil {
		return nil, err
	}
	return &Policy{guard: guard}, nil
}

func (p *Policy) Evaluate(source string, executeMode bool) Decision {
	graph, err := Parse(source)
	if err != nil {
		return deniedDecision(source, err)
	}
	decision := Decision{Outcome: Allow, Graph: graph, TargetPaths: []string{}}
	for groupIndex := range decision.Graph.Groups {
		for commandIndex := range decision.Graph.Groups[groupIndex].Commands {
			command := &decision.Graph.Groups[groupIndex].Commands[commandIndex]
			paths, sensitive, mutates, reason, validationErr := p.validateCommand(command, executeMode)
			if validationErr != nil {
				return deniedDecision(source, validationErr)
			}
			decision.TargetPaths = appendUnique(decision.TargetPaths, paths...)
			if sensitive {
				decision.Outcome = Confirm
				decision.ReasonCode = "SENSITIVE_PROJECT_READ"
				decision.PublicReason = "该命令将读取敏感项目文件，需要你的批准。"
			}
			if mutates {
				if len(decision.Graph.Groups) != 1 || len(decision.Graph.Groups[0].Commands) != 1 {
					return deniedDecision(source, commandError(CodeSyntaxDenied, "sed -i must be the only command in the call"))
				}
				decision.Outcome = Confirm
				decision.Mutates = true
				decision.ReasonCode = reason
				decision.PublicReason = "该命令将修改项目文件，需要你的批准。"
			}
		}
	}
	decision.Display = displayGraph(decision.Graph)
	decision.CommandHash = hashCommand(decision.Display)
	if decision.Mutates && len(decision.TargetPaths) == 1 {
		raw, readErr := os.ReadFile(filepath.Join(p.guard.Root(), filepath.FromSlash(decision.TargetPaths[0])))
		if readErr != nil {
			return deniedDecision(source, commandError(CodePathInvalid, readErr.Error()))
		}
		decision.PreimageHash = hashContent(raw)
	}
	return decision
}

func deniedDecision(source string, err error) Decision {
	code := ErrorCode(err)
	message := err.Error()
	if typed, ok := err.(*Error); ok {
		message = typed.Message
	}
	display := strings.TrimSpace(source)
	return Decision{
		Outcome: Deny, Display: display, CommandHash: hashCommand(display),
		ReasonCode: code, PublicReason: message, TargetPaths: []string{},
	}
}

func (p *Policy) validateCommand(command *Command, executeMode bool) ([]string, bool, bool, string, error) {
	if len(command.Args) == 0 {
		return nil, false, false, "", commandError(CodeParseInvalid, "empty command")
	}
	if strings.Contains(command.Args[0], "/") {
		return nil, false, false, "", commandError(CodeNotAllowed, "executable paths are not accepted")
	}
	switch command.Args[0] {
	case "pwd", "ls", "cat", "head", "tail", "stat", "wc":
		return p.validatePathCommand(command)
	case "find":
		return p.validateFind(command)
	case "grep", "rg":
		return p.validateSearch(command)
	case "jq":
		return p.validateJQ(command)
	case "sed":
		return p.validateSed(command, executeMode)
	case "git":
		return p.validateGit(command)
	default:
		return nil, false, false, "", commandError(CodeNotAllowed, "command is not in the project read allowlist")
	}
}

// Search operands may be directories or symlink aliases. Inspect their entire
// readable tree with the same guard and sensitive-path rules as direct reads.
func (p *Policy) searchReadsSensitive(paths []string) (bool, error) {
	sensitive := false
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(relative string) error {
		canonical, err := p.guard.Validate(relative, false)
		if err != nil {
			return err
		}
		sensitive = sensitive || IsSensitivePath(relative) || IsSensitivePath(canonical)
		if visited[canonical] {
			return nil
		}
		visited[canonical] = true
		return filepath.WalkDir(filepath.Join(p.guard.Root(), filepath.FromSlash(canonical)), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return commandError(CodePathInvalid, "search path cannot be inspected")
			}
			rel, err := filepath.Rel(p.guard.Root(), path)
			if err != nil {
				return err
			}
			sensitive = sensitive || IsSensitivePath(rel)
			if entry.Type()&os.ModeSymlink != 0 {
				return walk(filepath.ToSlash(rel))
			}
			if !entry.IsDir() && !entry.Type().IsRegular() {
				return commandError(CodePathInvalid, "special files are not supported")
			}
			return nil
		})
	}
	for _, path := range paths {
		if err := walk(path); err != nil {
			return false, err
		}
	}
	return sensitive, nil
}

func (p *Policy) validateGit(command *Command) ([]string, bool, bool, string, error) {
	if len(command.Args) < 2 || !oneOf(command.Args[1], "status", "diff", "log") {
		return nil, false, false, "", commandError(CodeNotAllowed, "only git status, git diff, and git log are supported")
	}
	gitDir := filepath.Join(p.guard.Root(), ".git")
	gitInfo, err := os.Lstat(gitDir)
	if err != nil || !gitInfo.IsDir() || gitInfo.Mode()&os.ModeSymlink != 0 {
		return nil, false, false, "", commandError(CodePathInvalid, "Git commands require a repository rooted inside the project")
	}
	for _, arg := range command.Args[2:] {
		if arg == "-c" || strings.HasPrefix(arg, "-c=") || strings.HasPrefix(arg, "--config-env") ||
			strings.HasPrefix(arg, "--exec-path") || strings.HasPrefix(arg, "--git-dir") ||
			strings.HasPrefix(arg, "--work-tree") || strings.Contains(arg, "ext-diff") ||
			strings.Contains(arg, "textconv") || strings.Contains(arg, "submodule") ||
			strings.Contains(arg, "output") || strings.Contains(arg, "pager") {
			return nil, false, false, "", commandError(CodeFlagDenied, "unsafe Git configuration and helper options are not supported")
		}
	}
	switch command.Args[1] {
	case "status":
		for _, arg := range command.Args[2:] {
			if !oneOf(arg, "--short", "-s", "--branch", "-b", "--porcelain", "--porcelain=v1", "--untracked-files=no", "--untracked-files=normal", "--untracked-files=all") {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported git status option "+arg)
			}
		}
		if !contains(command.Args, "--porcelain=v1") && !contains(command.Args, "--porcelain") {
			command.Args = append(command.Args, "--porcelain=v1")
		}
	case "diff":
		paths := []string{}
		sensitive := false
		afterSeparator := false
		for _, arg := range command.Args[2:] {
			if arg == "--" {
				if afterSeparator {
					return nil, false, false, "", commandError(CodeFlagDenied, "git diff accepts one path separator")
				}
				afterSeparator = true
				continue
			}
			if !afterSeparator {
				if !oneOf(arg,
					"--cached", "--staged", "--stat", "--shortstat", "--numstat",
					"--name-only", "--name-status", "--check", "--color=never",
				) {
					return nil, false, false, "", commandError(CodeFlagDenied, "unsupported git diff option "+arg)
				}
				continue
			}
			path, pathErr := p.guard.Validate(arg, false)
			if pathErr != nil {
				return nil, false, false, "", pathErr
			}
			paths = append(paths, path)
			sensitive = sensitive || IsSensitivePath(arg) || IsSensitivePath(path)
		}
		command.Args = append(command.Args[:2], append([]string{"--no-ext-diff", "--no-textconv"}, command.Args[2:]...)...)
		if !afterSeparator {
			command.Args = append(command.Args, "--", ".")
		}
		if !sensitive {
			command.Args = append(command.Args, sensitiveGitExclusions()...)
		}
		return paths, sensitive, false, "", nil
	case "log":
		for _, arg := range command.Args[2:] {
			if strings.HasPrefix(arg, "--format") || strings.HasPrefix(arg, "--pretty") || strings.HasPrefix(arg, "--max-count") || arg == "-n" {
				return nil, false, false, "", commandError(CodeFlagDenied, "git log output and count are Runtime-controlled")
			}
			if !oneOf(arg, "--all", "--branches", "--tags", "--first-parent", "--merges", "--no-merges", "--reverse") &&
				!strings.HasPrefix(arg, "--since=") && !strings.HasPrefix(arg, "--until=") {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported git log option "+arg)
			}
		}
		command.Args = append(command.Args[:2], append([]string{"--max-count=50", "--format=%h %s"}, command.Args[2:]...)...)
	}
	return nil, false, false, "", nil
}

func displayGraph(graph Graph) string {
	groups := make([]string, 0, len(graph.Groups))
	for _, group := range graph.Groups {
		commands := make([]string, 0, len(group.Commands))
		for _, command := range group.Commands {
			args := make([]string, 0, len(command.Args))
			for _, arg := range command.Args {
				args = append(args, shellQuote(arg))
			}
			commands = append(commands, strings.Join(args, " "))
		}
		groups = append(groups, strings.Join(commands, " | "))
	}
	return strings.Join(groups, " && ")
}

func shellQuote(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\n'\"\\$;&|()<>*?[") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func hashCommand(command string) string {
	sum := sha256.Sum256([]byte(PolicyVersion + "\x00" + command))
	return fmt.Sprintf("sha256:%x", sum)
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("sha256:%x", sum)
}

func ContentHash(content []byte) string {
	return hashContent(content)
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
