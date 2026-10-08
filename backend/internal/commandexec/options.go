package commandexec

import (
	"strconv"
	"strings"
)

// Options declare operand semantics, not spelling variants. The scanner handles
// clusters, attached values, separate values, --name=value, and -- centrally.
type optionSpec struct {
	name  string
	arity int
	files []int
}

type parsedOption struct {
	spec   optionSpec
	values []string
}

type parsedArgs struct {
	options  []parsedOption
	operands []string
}

type optionSet map[string]optionSpec

func options(flags, values string) optionSet {
	set := optionSet{}
	for _, name := range strings.Fields(flags) {
		set[name] = optionSpec{name: name}
	}
	for _, name := range strings.Fields(values) {
		set[name] = optionSpec{name: name, arity: 1}
	}
	return set
}

func (s optionSet) fileOption(name string, arity int, positions ...int) {
	s[name] = optionSpec{name: name, arity: arity, files: positions}
}

func parseOptions(args []string, set optionSet) (parsedArgs, error) {
	result := parsedArgs{}
	afterSeparator := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" && !afterSeparator {
			afterSeparator = true
			continue
		}
		if afterSeparator || arg == "-" || !strings.HasPrefix(arg, "-") {
			result.operands = append(result.operands, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			name, attached, hasAttached := strings.Cut(arg, "=")
			spec, ok := set[name]
			if !ok || (hasAttached && spec.arity == 0) {
				return result, commandError(CodeFlagDenied, "unsupported command option "+name)
			}
			option, err := consumeOption(args, &index, spec, attached, hasAttached)
			if err != nil {
				return result, err
			}
			result.options = append(result.options, option)
			continue
		}
		if spec, ok := set["-n"]; ok && spec.arity == 1 {
			if _, err := strconv.ParseUint(arg[1:], 10, 64); err == nil {
				result.options = append(result.options, parsedOption{spec: spec, values: []string{arg[1:]}})
				continue
			}
		}
		// Prefer a declared multi-character short option (e.g. stat -x).
		if spec, ok := set[arg]; ok {
			option, err := consumeOption(args, &index, spec, "", false)
			if err != nil {
				return result, err
			}
			result.options = append(result.options, option)
			continue
		}
		for offset := 1; offset < len(arg); offset++ {
			spec, ok := set["-"+string(arg[offset])]
			if !ok {
				return result, commandError(CodeFlagDenied, "unsupported command option -"+string(arg[offset]))
			}
			hasAttached := spec.arity > 0 && offset+1 < len(arg)
			attached := ""
			if hasAttached {
				attached = arg[offset+1:]
			}
			option, err := consumeOption(args, &index, spec, attached, hasAttached)
			if err != nil {
				return result, err
			}
			result.options = append(result.options, option)
			if spec.arity > 0 {
				break
			}
		}
	}
	return result, nil
}

func consumeOption(args []string, index *int, spec optionSpec, attached string, hasAttached bool) (parsedOption, error) {
	option := parsedOption{spec: spec}
	if hasAttached {
		option.values = append(option.values, attached)
	}
	for len(option.values) < spec.arity {
		(*index)++
		if *index >= len(args) {
			return option, commandError(CodeFlagDenied, spec.name+" requires a value")
		}
		option.values = append(option.values, args[*index])
	}
	return option, nil
}

func (a parsedArgs) has(names ...string) bool {
	for _, option := range a.options {
		if oneOf(option.spec.name, names...) {
			return true
		}
	}
	return false
}

// Reordering recognized options before operands avoids native getopt differences
// accidentally interpreting an unchecked pattern or filename as another option.
func (a parsedArgs) command(name string) []string {
	args := []string{name}
	for _, option := range a.options {
		if oneOf(option.spec.name, "--color", "--colour") && len(option.values) == 1 {
			args = append(args, option.spec.name+"="+option.values[0])
		} else {
			args = append(args, option.spec.name)
			args = append(args, option.values...)
		}
	}
	if len(a.operands) > 0 {
		args = append(args, "--")
		args = append(args, a.operands...)
	}
	return args
}

func (p *Policy) optionFiles(args parsedArgs) ([]string, bool, error) {
	paths := []string{}
	sensitive := false
	for _, option := range args.options {
		for _, index := range option.spec.files {
			raw := option.values[index]
			if raw == "-" {
				return nil, false, commandError(CodeFlagDenied, "option-loaded files must be explicit project files")
			}
			path, err := p.guard.Validate(raw, true)
			if err != nil {
				return nil, false, err
			}
			paths = appendUnique(paths, path)
			sensitive = sensitive || IsSensitivePath(raw) || IsSensitivePath(path)
		}
	}
	return paths, sensitive, nil
}

func (p *Policy) operandPaths(operands []string, regular, stdin bool) ([]string, bool, error) {
	paths := []string{}
	sensitive := false
	for _, raw := range operands {
		if raw == "-" && stdin {
			continue
		}
		path, err := p.guard.Validate(raw, regular)
		if err != nil {
			return nil, false, err
		}
		paths = appendUnique(paths, path)
		sensitive = sensitive || IsSensitivePath(raw) || IsSensitivePath(path)
	}
	return paths, sensitive, nil
}
