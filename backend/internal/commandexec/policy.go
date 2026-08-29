package commandexec

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
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
	case "pwd":
		if len(command.Args) != 1 {
			return nil, false, false, "", commandError(CodeFlagDenied, "pwd does not accept arguments")
		}
		return nil, false, false, "", nil
	case "ls":
		return p.validateSimplePaths(command.Args[1:], false, map[string]bool{
			"-1": true, "-a": true, "-A": true, "-d": true, "-l": true, "-la": true, "-al": true, "--": true,
		})
	case "cat":
		return p.validateSimplePaths(command.Args[1:], true, map[string]bool{"--": true})
	case "head":
		return p.validateHeadTail(command.Args[1:], false)
	case "tail":
		return p.validateHeadTail(command.Args[1:], true)
	case "find":
		return p.validateFind(command)
	case "grep":
		return p.validateGrep(command)
	case "rg":
		return p.validateRG(command)
	case "jq":
		return p.validateJQ(command)
	case "stat":
		return p.validateSimplePaths(command.Args[1:], true, map[string]bool{"-f": true, "-L": false, "--": true})
	case "sed":
		return p.validateSed(command, executeMode)
	case "wc":
		return p.validateSimplePaths(command.Args[1:], false, map[string]bool{"-c": true, "-l": true, "-m": true, "-w": true, "--": true})
	case "git":
		return p.validateGit(command)
	default:
		return nil, false, false, "", commandError(CodeNotAllowed, "command is not in the project read allowlist")
	}
}

func (p *Policy) validateSimplePaths(args []string, requireRegular bool, flags map[string]bool) ([]string, bool, bool, string, error) {
	paths := []string{}
	sensitive := false
	afterSeparator := false
	for _, arg := range args {
		if arg == "--" {
			afterSeparator = true
			continue
		}
		if strings.HasPrefix(arg, "-") && !afterSeparator {
			allowed, known := flags[arg]
			if !known || !allowed {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported command flag "+arg)
			}
			continue
		}
		path, err := p.guard.Validate(arg, requireRegular)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		sensitive = sensitive || IsSensitivePath(path)
	}
	if requireRegular && len(paths) == 0 {
		return nil, false, false, "", commandError(CodePathInvalid, "at least one file operand is required")
	}
	return paths, sensitive, false, "", nil
}

func (p *Policy) validateHeadTail(args []string, tail bool) ([]string, bool, bool, string, error) {
	paths := []string{}
	sensitive := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if tail && (arg == "-f" || arg == "-F" || arg == "--follow" || arg == "--retry" || strings.HasPrefix(arg, "--pid")) {
			return nil, false, false, "", commandError(CodeFlagDenied, "stream-following options are not supported")
		}
		if arg == "-n" || arg == "-c" {
			if index+1 >= len(args) {
				return nil, false, false, "", commandError(CodeFlagDenied, arg+" requires a count")
			}
			index++
			count, err := strconv.Atoi(strings.TrimPrefix(args[index], "+"))
			limit := 1000
			if arg == "-c" {
				limit = 65536
			}
			if err != nil || count < 0 || count > limit {
				return nil, false, false, "", commandError(CodeFlagDenied, "requested output count exceeds the command limit")
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return nil, false, false, "", commandError(CodeFlagDenied, "unsupported command flag "+arg)
		}
		path, err := p.guard.Validate(arg, true)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		sensitive = sensitive || IsSensitivePath(path)
	}
	return paths, sensitive, false, "", nil
}

func (p *Policy) validateFind(command *Command) ([]string, bool, bool, string, error) {
	if len(command.Args) < 2 {
		command.Args = append(command.Args, ".", "-maxdepth", "8", "-print")
	}
	paths := []string{}
	index := 1
	for index < len(command.Args) && !strings.HasPrefix(command.Args[index], "-") {
		path, err := p.guard.Validate(command.Args[index], false)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		index++
	}
	if len(paths) == 0 {
		return nil, false, false, "", commandError(CodePathInvalid, "find requires an explicit project-local root")
	}
	hasDepth := false
	for index < len(command.Args) {
		arg := command.Args[index]
		switch arg {
		case "-maxdepth", "-mindepth":
			if index+1 >= len(command.Args) {
				return nil, false, false, "", commandError(CodeFlagDenied, arg+" requires a value")
			}
			value, err := strconv.Atoi(command.Args[index+1])
			if err != nil || value < 0 || value > 8 {
				return nil, false, false, "", commandError(CodeFlagDenied, "find depth must be between 0 and 8")
			}
			hasDepth = hasDepth || arg == "-maxdepth"
			index += 2
		case "-name", "-iname", "-path", "-ipath":
			if index+1 >= len(command.Args) {
				return nil, false, false, "", commandError(CodeFlagDenied, arg+" requires a pattern")
			}
			index += 2
		case "-type":
			if index+1 >= len(command.Args) || !oneOf(command.Args[index+1], "f", "d", "l") {
				return nil, false, false, "", commandError(CodeFlagDenied, "find -type accepts only f, d, or l")
			}
			index += 2
		case "-print", "-print0":
			index++
		default:
			return nil, false, false, "", commandError(CodeFlagDenied, "unsupported find predicate "+arg)
		}
	}
	if !hasDepth {
		insertAt := 1 + len(paths)
		command.Args = append(command.Args[:insertAt], append([]string{"-maxdepth", "8"}, command.Args[insertAt:]...)...)
	}
	return paths, false, false, "", nil
}

func (p *Policy) validateGrep(command *Command) ([]string, bool, bool, string, error) {
	paths := []string{}
	sensitive := false
	patternSeen := false
	for index := 1; index < len(command.Args); index++ {
		arg := command.Args[index]
		if arg == "-f" || arg == "--file" || strings.HasPrefix(arg, "--file=") ||
			arg == "--include-from" || arg == "--exclude-from" {
			return nil, false, false, "", commandError(CodeFlagDenied, "grep file-loading flags are not supported")
		}
		if strings.HasPrefix(arg, "-") && !patternSeen {
			if !grepFlagAllowed(arg) {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported grep flag "+arg)
			}
			continue
		}
		if !patternSeen {
			patternSeen = true
			continue
		}
		path, err := p.guard.Validate(arg, false)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		sensitive = sensitive || IsSensitivePath(path)
	}
	if !patternSeen {
		return nil, false, false, "", commandError(CodeFlagDenied, "grep requires a pattern")
	}
	if !sensitive {
		command.Args = append(command.Args[:1], append([]string{"--exclude=.env*", "--exclude=*.key", "--exclude=*.pem"}, command.Args[1:]...)...)
	}
	return paths, sensitive, false, "", nil
}

func grepFlagAllowed(arg string) bool {
	return oneOf(arg, "-n", "-H", "-h", "-i", "-v", "-w", "-x", "-E", "-F", "-r", "-R", "-l", "-L", "-c", "-q", "-s", "--line-number", "--ignore-case", "--recursive", "--fixed-strings") ||
		strings.HasPrefix(arg, "--include=") || strings.HasPrefix(arg, "--exclude=") ||
		strings.HasPrefix(arg, "--exclude-dir=") || strings.HasPrefix(arg, "-m")
}

func (p *Policy) validateRG(command *Command) ([]string, bool, bool, string, error) {
	paths := []string{}
	sensitive := false
	patternSeen := false
	for index := 1; index < len(command.Args); index++ {
		arg := command.Args[index]
		if oneOf(arg, "--pre", "--pre-glob", "--follow", "-L", "--no-ignore", "--no-ignore-vcs", "--hidden") ||
			strings.HasPrefix(arg, "--pre=") || strings.HasPrefix(arg, "--config") {
			return nil, false, false, "", commandError(CodeFlagDenied, "ripgrep option may execute helpers or broaden filesystem access")
		}
		if strings.HasPrefix(arg, "-") && !patternSeen {
			if !rgFlagAllowed(arg) {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported ripgrep flag "+arg)
			}
			continue
		}
		if !patternSeen {
			patternSeen = true
			continue
		}
		path, err := p.guard.Validate(arg, false)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		sensitive = sensitive || IsSensitivePath(path)
	}
	if !patternSeen {
		return nil, false, false, "", commandError(CodeFlagDenied, "rg requires a pattern")
	}
	if !sensitive {
		command.Args = append(command.Args[:1], append([]string{
			"--glob=!**/.env*", "--glob=!**/*.key", "--glob=!**/*.pem",
		}, command.Args[1:]...)...)
	}
	return paths, sensitive, false, "", nil
}

func rgFlagAllowed(arg string) bool {
	return oneOf(arg, "-n", "-i", "-v", "-w", "-x", "-F", "-l", "--files", "--json", "--count", "--stats", "--no-heading", "--hidden=false") ||
		strings.HasPrefix(arg, "--glob=") || strings.HasPrefix(arg, "-g") ||
		strings.HasPrefix(arg, "--type=") || strings.HasPrefix(arg, "-t") ||
		strings.HasPrefix(arg, "--max-count=") || strings.HasPrefix(arg, "-m")
}

func (p *Policy) validateJQ(command *Command) ([]string, bool, bool, string, error) {
	paths := []string{}
	sensitive := false
	filterSeen := false
	for index := 1; index < len(command.Args); index++ {
		arg := command.Args[index]
		if oneOf(arg, "-L", "--from-file", "-f", "--slurpfile", "--rawfile", "--argfile") ||
			strings.HasPrefix(arg, "--slurpfile=") || strings.HasPrefix(arg, "--rawfile=") || strings.HasPrefix(arg, "--argfile=") {
			return nil, false, false, "", commandError(CodeFlagDenied, "jq module and file-loading options are not supported")
		}
		if strings.HasPrefix(arg, "-") && !filterSeen {
			if !oneOf(arg, "-c", "-r", "-M", "-S", "-s", "--compact-output", "--raw-output", "--monochrome-output", "--sort-keys", "--slurp") {
				return nil, false, false, "", commandError(CodeFlagDenied, "unsupported jq flag "+arg)
			}
			continue
		}
		if !filterSeen {
			filterSeen = true
			continue
		}
		path, err := p.guard.Validate(arg, true)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = append(paths, path)
		sensitive = sensitive || IsSensitivePath(path)
	}
	if !filterSeen {
		return nil, false, false, "", commandError(CodeFlagDenied, "jq requires a filter")
	}
	return paths, sensitive, false, "", nil
}

var (
	readSedScript  = regexp.MustCompile(`^[0-9]+(?:,[0-9]+)?[pq]$`)
	writeSedScript = regexp.MustCompile(`^(?:[0-9]+)?s/([^/\\$&\n]+)/([^/\\$&\n]*)/(g?)$`)
)

func (p *Policy) validateSed(command *Command, executeMode bool) ([]string, bool, bool, string, error) {
	args := command.Args
	if len(args) == 4 && args[1] == "-n" && readSedScript.MatchString(args[2]) {
		path, err := p.guard.Validate(args[3], true)
		if err != nil {
			return nil, false, false, "", err
		}
		return []string{path}, IsSensitivePath(path), false, "", nil
	}
	if len(args) != 5 || args[1] != "-i" || args[2] != "" || !writeSedScript.MatchString(args[3]) {
		return nil, false, false, "", commandError(CodeFlagDenied, "sed supports only numeric -n selection or the approved single-file -i substitution")
	}
	if !executeMode {
		return nil, false, false, "", commandError(CodeNotAllowed, "sed -i is available only in execute mode")
	}
	path, err := p.guard.Validate(args[4], true)
	if err != nil {
		return nil, false, false, "", err
	}
	raw, err := os.ReadFile(filepath.Join(p.guard.Root(), filepath.FromSlash(path)))
	if err != nil {
		return nil, false, false, "", commandError(CodePathInvalid, err.Error())
	}
	if len(raw) > maxEditableFileBytes || !utf8.Valid(raw) {
		return nil, false, false, "", commandError(CodePathInvalid, "sed -i target must be UTF-8 text no larger than 1 MiB")
	}
	return []string{path}, false, true, "PROJECT_FILE_EDIT", nil
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
			sensitive = sensitive || IsSensitivePath(path)
		}
		command.Args = append(command.Args[:2], append([]string{"--no-ext-diff", "--no-textconv"}, command.Args[2:]...)...)
		if !afterSeparator {
			command.Args = append(command.Args,
				"--", ".",
				":(exclude)**/.env*", ":(exclude)**/*.key", ":(exclude)**/*.pem",
			)
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
