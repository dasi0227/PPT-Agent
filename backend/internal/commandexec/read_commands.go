package commandexec

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func (p *Policy) validatePathCommand(command *Command) ([]string, bool, bool, string, error) {
	name := command.Args[0]
	var set optionSet
	regular, stdin := false, false
	switch name {
	case "pwd":
		set = options("-L -P --logical --physical --help --version", "")
	case "ls":
		set = options("-1 -a -A -b -B -c -C -d -D -f -F -g -G -h -H -i -I -k -l -L -m -n -N -o -O -p -q -Q -r -R -s -S -t -T -u -U -v -w -x --all --almost-all --directory --human-readable --inode --numeric-uid-gid --recursive --reverse --size --dereference --dereference-command-line --literal --help --version",
			"--block-size --color --format --hide --ignore --indicator-style --quoting-style --sort --time --time-style --width --tabsize")
		if runtime.GOOS == "linux" {
			for _, name := range []string{"-I", "-T", "-w"} {
				set[name] = optionSpec{name: name, arity: 1}
			}
		} else {
			set["-D"] = optionSpec{name: "-D", arity: 1}
		}
	case "cat":
		regular, stdin = true, true
		set = options("-A -b -e -E -n -s -t -T -u -v --show-all --number-nonblank --show-ends --number --squeeze-blank --show-tabs --show-nonprinting --help --version", "")
	case "head", "tail":
		regular, stdin = true, true
		set = options("-q -v --quiet --silent --verbose --zero-terminated -z --help --version", "-n -c --lines --bytes")
	case "stat":
		set = options("-L -l -n -q -r -s -x --dereference --file-system --terse --help --version", "-c --format --printf --cached")
		if runtime.GOOS == "linux" {
			set["-f"] = optionSpec{name: "-f"}
			set["-t"] = optionSpec{name: "-t"}
		} else {
			set["-f"] = optionSpec{name: "-f", arity: 1}
			set["-t"] = optionSpec{name: "-t", arity: 1}
		}
	case "wc":
		regular, stdin = true, true
		set = options("-c -l -L -m -w --bytes --lines --max-line-length --chars --words --help --version", "")
	}
	parsed, err := parseOptions(command.Args[1:], set)
	if err != nil {
		return nil, false, false, "", err
	}
	if name == "pwd" && len(parsed.operands) > 0 {
		return nil, false, false, "", commandError(CodeFlagDenied, "pwd does not accept file operands")
	}
	paths, sensitive, err := p.operandPaths(parsed.operands, regular, stdin)
	if err != nil {
		return nil, false, false, "", err
	}
	if name == "ls" {
		roots := paths
		if len(roots) == 0 {
			roots = []string{"."}
		}
		if parsed.has("-R", "--recursive", "-L", "--dereference") {
			if _, err := p.searchReadsSensitive(roots); err != nil {
				return nil, false, false, "", err
			}
		} else if err := p.validateListedLinks(roots, !parsed.has("-d", "--directory")); err != nil {
			return nil, false, false, "", err
		}
	}
	command.Args = parsed.command(name)
	return paths, sensitive, false, "", nil
}

func (p *Policy) validateListedLinks(paths []string, listContents bool) error {
	if !listContents {
		return nil
	}
	for _, relative := range paths {
		entries, err := os.ReadDir(filepath.Join(p.guard.Root(), filepath.FromSlash(relative)))
		if err != nil {
			// Regular operands were already checked; ls itself reports missing paths.
			continue
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				if _, err := p.guard.Validate(filepath.ToSlash(filepath.Join(relative, entry.Name())), false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func grepOptions() optionSet {
	set := options("-a -b -c -E -F -G -H -h -i -I -l -L -n -o -P -q -r -R -s -v -w -x -z -Z -U --text --byte-offset --count --extended-regexp --fixed-strings --basic-regexp --with-filename --no-filename --ignore-case --files-with-matches --files-without-match --line-number --only-matching --perl-regexp --quiet --silent --recursive --dereference-recursive --no-messages --invert-match --word-regexp --line-regexp --null-data --null --line-buffered --help --version",
		"-e -m -A -B -C -d -D --regexp --max-count --after-context --before-context --context --directories --devices --include --exclude --exclude-dir --binary-files --color --colour --label --group-separator")
	set.fileOption("-f", 1, 0)
	set.fileOption("--file", 1, 0)
	set.fileOption("--exclude-from", 1, 0)
	return set
}

func rgOptions() optionSet {
	set := options("-a -b -c -F -H -h -i -I -l -L -n -N -o -P -q -s -S -U -v -w -x -z -0 --text --byte-offset --count --count-matches --fixed-strings --with-filename --no-filename --ignore-case --files-with-matches --files-without-match --line-number --no-line-number --only-matching --pcre2 --quiet --case-sensitive --smart-case --multiline --invert-match --word-regexp --line-regexp --null --files --json --stats --heading --no-heading --hidden --follow --no-ignore --no-ignore-vcs --no-ignore-parent --no-ignore-global --no-ignore-dot --no-config --line-buffered --block-buffered --crlf --passthru --trim --type-list --help --version --no-messages --no-unicode --unicode --no-pcre2-unicode --pcre2-unicode --no-require-git",
		"-e -g -t -T -m -A -B -C -j -r --regexp --glob --iglob --type --type-not --type-add --type-clear --max-count --after-context --before-context --context --threads --replace --max-depth --max-filesize --encoding --engine --sort --sortr --color --colors --field-match-separator --field-context-separator --context-separator --path-separator")
	// Compressed searches can launch subprocesses.
	delete(set, "-z")
	set.fileOption("-f", 1, 0)
	set.fileOption("--file", 1, 0)
	set.fileOption("--ignore-file", 1, 0)
	return set
}

func (p *Policy) validateSearch(command *Command) ([]string, bool, bool, string, error) {
	name := command.Args[0]
	set := grepOptions()
	if name == "rg" {
		set = rgOptions()
	}
	parsed, err := parseOptions(command.Args[1:], set)
	if err != nil {
		return nil, false, false, "", err
	}
	for _, option := range parsed.options {
		if name == "grep" && oneOf(option.spec.name, "-D", "--devices") && option.values[0] != "skip" {
			return nil, false, false, "", commandError(CodeFlagDenied, "grep device reads are not supported")
		}
	}
	filePaths, sensitive, err := p.optionFiles(parsed)
	if err != nil {
		return nil, false, false, "", err
	}
	operands := parsed.operands
	noPattern := parsed.has("--files", "--type-list", "--help", "--version")
	if !noPattern && !parsed.has("-e", "--regexp", "-f", "--file") {
		if len(operands) == 0 {
			return nil, false, false, "", commandError(CodeFlagDenied, name+" requires a pattern")
		}
		operands = operands[1:]
	}
	paths, directSensitive, err := p.operandPaths(operands, false, true)
	if err != nil {
		return nil, false, false, "", err
	}
	recursive := name == "rg" && !parsed.has("--type-list", "--help", "--version") || parsed.has("-r", "-R", "--recursive", "--dereference-recursive")
	if name == "grep" {
		for _, option := range parsed.options {
			recursive = recursive || oneOf(option.spec.name, "-d", "--directories") && option.values[0] == "recurse"
		}
	}
	if recursive {
		if len(paths) == 0 && !contains(operands, "-") {
			paths = []string{"."}
		}
		readSensitive, err := p.searchReadsSensitive(paths)
		if err != nil {
			return nil, false, false, "", err
		}
		if !parsed.has("--files") {
			sensitive = sensitive || readSensitive
		}
	}
	if name == "rg" {
		// Ignore files from parent directories and global configuration may live
		// outside the project, even when every explicit operand is local.
		for _, option := range []string{"--no-ignore-parent", "--no-ignore-global"} {
			if !parsed.has(option) {
				parsed.options = append(parsed.options, parsedOption{spec: set[option]})
			}
		}
	}
	command.Args = parsed.command(name)
	return appendUnique(filePaths, paths...), sensitive || directSensitive, false, "", nil
}

func jqOptions() optionSet {
	set := options("-c -r -R -s -n -e -j -a -S -M -C --compact-output --raw-output --raw-input --slurp --null-input --exit-status --join-output --ascii-output --sort-keys --monochrome-output --color-output --unbuffered --stream --stream-errors --seq --raw-output0 --args --jsonargs --help --version", "--indent")
	for _, name := range []string{"--arg", "--argjson"} {
		set[name] = optionSpec{name: name, arity: 2}
	}
	for _, name := range []string{"--rawfile", "--slurpfile", "--argfile"} {
		set.fileOption(name, 2, 1)
	}
	set.fileOption("-f", 1, 0)
	set.fileOption("--from-file", 1, 0)
	return set
}

func (p *Policy) validateJQ(command *Command) ([]string, bool, bool, string, error) {
	parsed, err := parseOptions(command.Args[1:], jqOptions())
	if err != nil {
		return nil, false, false, "", err
	}
	paths, sensitive, err := p.optionFiles(parsed)
	if err != nil {
		return nil, false, false, "", err
	}
	for _, option := range parsed.options {
		if oneOf(option.spec.name, "-f", "--from-file") {
			program, err := p.readProgram(option.values[0])
			if err != nil {
				return nil, false, false, "", err
			}
			if jqLoadsModules(program) {
				return nil, false, false, "", commandError(CodeFlagDenied, "jq module imports are not supported")
			}
		}
	}
	operands := parsed.operands
	if !parsed.has("-f", "--from-file", "--help", "--version") {
		if len(operands) == 0 {
			return nil, false, false, "", commandError(CodeFlagDenied, "jq requires a filter")
		}
		if jqLoadsModules(operands[0]) {
			return nil, false, false, "", commandError(CodeFlagDenied, "jq module imports are not supported")
		}
		operands = operands[1:]
	}
	if !parsed.has("--args", "--jsonargs") {
		filePaths, readSensitive, err := p.operandPaths(operands, true, true)
		if err != nil {
			return nil, false, false, "", err
		}
		paths = appendUnique(paths, filePaths...)
		sensitive = sensitive || readSensitive
	}
	command.Args = parsed.command("jq")
	return paths, sensitive, false, "", nil
}

func (p *Policy) readProgram(relative string) (string, error) {
	path, err := p.guard.Validate(relative, true)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(p.guard.Root(), filepath.FromSlash(path)))
	if err != nil || info.Size() > maxEditableFileBytes {
		return "", commandError(CodeFlagDenied, "command program must be no larger than 1 MiB")
	}
	raw, err := os.ReadFile(filepath.Join(p.guard.Root(), filepath.FromSlash(path)))
	if err != nil {
		return "", commandError(CodePathInvalid, "command program cannot be inspected")
	}
	return string(raw), nil
}

// jq imports are language statements. Do not mistake quoted string values or
// comments for imports; module paths cannot be confined through argv checking.
func jqLoadsModules(program string) bool {
	for index := 0; index < len(program); {
		switch program[index] {
		case '#':
			for index < len(program) && program[index] != '\n' {
				index++
			}
		case '"':
			index++
			for index < len(program) {
				if program[index] == '\\' {
					index += 2
				} else if program[index] == '"' {
					index++
					break
				} else {
					index++
				}
			}
		default:
			start := index
			for index < len(program) && (program[index] >= 'a' && program[index] <= 'z' || program[index] == '_') {
				index++
			}
			if start == index {
				index++
				continue
			}
			if oneOf(program[start:index], "import", "include") {
				next := index
				for next < len(program) {
					if strings.ContainsRune(" \t\r\n", rune(program[next])) {
						next++
					} else if program[next] == '#' {
						for next < len(program) && program[next] != '\n' {
							next++
						}
					} else {
						break
					}
				}
				if next < len(program) && program[next] == '"' {
					return true
				}
			}
		}
	}
	return false
}
