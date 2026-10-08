package commandexec

import "strings"

func (p *Policy) validateFind(command *Command) ([]string, bool, bool, string, error) {
	prefix := []string{"find"}
	roots := []string{}
	index := 1
	for index < len(command.Args) {
		arg := command.Args[index]
		if oneOf(arg, "-H", "-L", "-P", "-E", "-X", "-s", "-x", "-d") {
			prefix = append(prefix, arg)
			index++
			continue
		}
		if arg == "-f" {
			if index+1 >= len(command.Args) {
				return nil, false, false, "", commandError(CodeFlagDenied, "find -f requires a root")
			}
			roots = append(roots, command.Args[index+1])
			index += 2
			continue
		}
		if strings.HasPrefix(arg, "-") || oneOf(arg, "!", "(", ")") {
			break
		}
		roots = append(roots, arg)
		index++
	}
	if len(roots) == 0 {
		roots = []string{"."}
	}
	paths, _, err := p.operandPaths(roots, false, false)
	if err != nil {
		return nil, false, false, "", err
	}
	// Follow every reachable alias during inspection, even if find itself uses
	// -P. This also checks aliases reached by -L/-follow and predicate file reads.
	if _, err := p.searchReadsSensitive(paths); err != nil {
		return nil, false, false, "", err
	}
	set := options("-print -print0 -ls -empty -prune -quit -true -false -depth -mount -xdev -follow -ignore_readdir_race -noignore_readdir_race -readable -writable -executable -nouser -nogroup -a -and -o -or -not ! ( )",
		"-maxdepth -mindepth -name -iname -path -ipath -wholename -iwholename -lname -ilname -regex -iregex -regextype -type -xtype -size -mtime -mmin -atime -amin -ctime -cmin -Btime -Bmin -user -uid -group -gid -perm -links -inum -used -fstype -printf")
	for _, name := range []string{"-newer", "-anewer", "-cnewer", "-samefile"} {
		set.fileOption(name, 1, 0)
	}
	predicates := []string{}
	files := []string{}
	sensitive := false
	for index < len(command.Args) {
		arg := command.Args[index]
		spec, ok := set[arg]
		// -newerXY has either a project-file operand or a timestamp operand.
		if !ok && len(arg) == 8 && strings.HasPrefix(arg, "-newer") && strings.ContainsRune("acmB", rune(arg[6])) && strings.ContainsRune("acmBt", rune(arg[7])) {
			spec = optionSpec{name: arg, arity: 1}
			if arg[7] != 't' {
				spec.files = []int{0}
			}
			ok = true
		}
		if !ok {
			return nil, false, false, "", commandError(CodeFlagDenied, "unsupported or unsafe find predicate "+arg)
		}
		option, err := consumeOption(command.Args, &index, spec, "", false)
		if err != nil {
			return nil, false, false, "", err
		}
		loaded, readSensitive, err := p.optionFiles(parsedArgs{options: []parsedOption{option}})
		if err != nil {
			return nil, false, false, "", err
		}
		files = appendUnique(files, loaded...)
		sensitive = sensitive || readSensitive
		predicates = append(predicates, arg)
		predicates = append(predicates, option.values...)
		index++
	}
	for index, root := range roots {
		if strings.HasPrefix(root, "-") || oneOf(root, "!", "(", ")") {
			roots[index] = "./" + root
		}
	}
	command.Args = append(prefix, roots...)
	command.Args = append(command.Args, predicates...)
	return appendUnique(paths, files...), sensitive, false, "", nil
}
