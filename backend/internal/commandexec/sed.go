package commandexec

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var writeSedScript = regexp.MustCompile(`^(?:[0-9]+)?s/([^/\\$&\n]+)/([^/\\$&\n]*)/(g?)$`)

func (p *Policy) validateSed(command *Command, executeMode bool) ([]string, bool, bool, string, error) {
	args := command.Args
	if len(args) == 5 && args[1] == "-i" && args[2] == "" && writeSedScript.MatchString(args[3]) {
		if !executeMode {
			return nil, false, false, "", commandError(CodeNotAllowed, "sed -i is available only in execute mode")
		}
		path, err := p.guard.Validate(args[4], true)
		if err != nil {
			return nil, false, false, "", err
		}
		raw, err := os.ReadFile(filepath.Join(p.guard.Root(), filepath.FromSlash(path)))
		if err != nil || len(raw) > maxEditableFileBytes || !utf8.Valid(raw) {
			return nil, false, false, "", commandError(CodePathInvalid, "sed -i target must be UTF-8 text no larger than 1 MiB")
		}
		return []string{path}, false, true, "PROJECT_FILE_EDIT", nil
	}
	set := options("-n -E -r -u -z --quiet --silent --regexp-extended --unbuffered --null-data --sandbox --help --version", "-e --expression")
	set.fileOption("-f", 1, 0)
	set.fileOption("--file", 1, 0)
	parsed, err := parseOptions(args[1:], set)
	if err != nil {
		return nil, false, false, "", err
	}
	paths, sensitive, err := p.optionFiles(parsed)
	if err != nil {
		return nil, false, false, "", err
	}
	for _, option := range parsed.options {
		if oneOf(option.spec.name, "-e", "--expression", "-f", "--file") {
			program := option.values[0]
			if oneOf(option.spec.name, "-f", "--file") {
				program, err = p.readProgram(program)
				if err != nil {
					return nil, false, false, "", err
				}
			}
			if !readOnlySedProgram(program) {
				return nil, false, false, "", commandError(CodeFlagDenied, "sed scripts may not execute programs, read extra files, or write files")
			}
		}
	}
	operands := parsed.operands
	if !parsed.has("-e", "--expression", "-f", "--file", "--help", "--version") {
		if len(operands) == 0 || !readOnlySedProgram(operands[0]) {
			return nil, false, false, "", commandError(CodeFlagDenied, "sed requires a supported read-only script")
		}
		operands = operands[1:]
	}
	files, readSensitive, err := p.operandPaths(operands, true, true)
	if err != nil {
		return nil, false, false, "", err
	}
	command.Args = parsed.command("sed")
	return appendUnique(paths, files...), sensitive || readSensitive, false, "", nil
}

// Parse the read-only subset instead of scanning for letters that can occur in
// patterns or replacements. r/R/w/W/e and substitution e/w flags are excluded.
func readOnlySedProgram(program string) bool {
	index, depth := 0, 0
	skipSpace := func() {
		for index < len(program) && strings.ContainsRune(" \t\r", rune(program[index])) {
			index++
		}
	}
	for index < len(program) {
		skipSpace()
		if index == len(program) {
			break
		}
		if strings.ContainsRune(";\n", rune(program[index])) {
			index++
			continue
		}
		if program[index] == '#' {
			for index < len(program) && program[index] != '\n' {
				index++
			}
			continue
		}
		if program[index] == '}' {
			depth--
			index++
			if depth < 0 {
				return false
			}
			continue
		}
		for address := 0; address < 2; address++ {
			skipSpace()
			if index >= len(program) {
				return false
			}
			if program[index] >= '0' && program[index] <= '9' {
				for index < len(program) && program[index] >= '0' && program[index] <= '9' {
					index++
				}
			} else if program[index] == '$' {
				index++
			} else if program[index] == '/' {
				index++
				if !sedDelimited(program, &index, '/') {
					return false
				}
			} else {
				break
			}
			skipSpace()
			if index < len(program) && program[index] == ',' && address == 0 {
				index++
				continue
			}
			break
		}
		skipSpace()
		if index < len(program) && program[index] == '!' {
			index++
			skipSpace()
		}
		if index >= len(program) {
			return false
		}
		cmd := program[index]
		index++
		switch cmd {
		case '{':
			depth++
			continue
		case 'p', 'P', 'd', 'D', 'n', 'N', 'h', 'H', 'g', 'G', 'x', '=':
		case 'q', 'Q', 'l':
			skipSpace()
			for index < len(program) && program[index] >= '0' && program[index] <= '9' {
				index++
			}
		case ':', 'b', 't', 'T':
			for index < len(program) && !strings.ContainsRune(";\n}", rune(program[index])) {
				index++
			}
		case 's', 'y':
			if index >= len(program) || strings.ContainsRune("\\\n\r", rune(program[index])) {
				return false
			}
			delimiter := program[index]
			index++
			if !sedDelimited(program, &index, delimiter) || !sedDelimited(program, &index, delimiter) {
				return false
			}
			if cmd == 's' {
				for index < len(program) && (strings.ContainsRune("gpIiMm", rune(program[index])) || program[index] >= '0' && program[index] <= '9') {
					index++
				}
			}
		default:
			return false
		}
		skipSpace()
		if index < len(program) && !strings.ContainsRune(";\n}", rune(program[index])) {
			return false
		}
	}
	return depth == 0
}

func sedDelimited(program string, index *int, delimiter byte) bool {
	for *index < len(program) {
		char := program[*index]
		(*index)++
		if char == '\\' {
			if *index >= len(program) {
				return false
			}
			(*index)++
		} else if char == delimiter {
			return true
		} else if char == '\n' || char == '\r' {
			return false
		}
	}
	return false
}
