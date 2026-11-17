package commandexec

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

func ContentHash(content []byte) string { return fmt.Sprintf("%x", sha256.Sum256(content)) }

func Evaluate(source string) Decision {
	d := Decision{Outcome: Allow, Source: source, Display: source, CommandHash: ContentHash([]byte(PolicyVersion + "\x00" + source))}
	deny := func(code, message string) Decision {
		d.Outcome = Deny
		d.ReasonCode = code
		d.PublicReason = message
		return d
	}
	if strings.TrimSpace(source) == "" || strings.ContainsRune(source, 0) {
		return deny(CodeInvalid, "Command must be nonempty shell source without NUL bytes.")
	}
	tree, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(source), "command")
	if err != nil {
		return deny(CodeInvalid, err.Error())
	}
	// Syntax is parsed only to reject explicit Git commit/push requests, never
	// to maintain a whitelist of reading commands or options.
	forbidden := ""
	syntax.Walk(tree, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		args, literal := literalArgs(call.Args)
		if !literal || len(args) == 0 {
			return true
		}
		if filepath.Base(args[0]) == "git" {
			for i := 1; i < len(args); i++ {
				arg := args[i]
				if arg == "-C" || arg == "-c" || arg == "--git-dir" || arg == "--work-tree" || arg == "--namespace" || arg == "--config-env" {
					i++
					continue
				}
				if strings.HasPrefix(arg, "-") {
					continue
				}
				if arg == "commit" || arg == "push" {
					forbidden = "Git commit and push are reserved for manual actions."
				}
				break
			}
		}
		return true
	})
	if forbidden != "" {
		return deny(CodeNotAllowed, forbidden)
	}
	if len(tree.Stmts) == 1 {
		stmt := tree.Stmts[0]
		if call, ok := stmt.Cmd.(*syntax.CallExpr); ok && len(stmt.Redirs) == 0 && len(call.Assigns) == 0 {
			args, ok := literalArgs(call.Args)
			if ok && len(args) == 2 && args[0] == "cat" && !strings.HasPrefix(args[1], "-") {
				d.FullReadPath = filepath.ToSlash(filepath.Clean(args[1]))
			}
		}
	}
	return d
}

func literalArgs(words []*syntax.Word) ([]string, bool) {
	out := make([]string, 0, len(words))
	var literal func([]syntax.WordPart) (string, bool)
	literal = func(parts []syntax.WordPart) (string, bool) {
		var b strings.Builder
		for _, part := range parts {
			switch p := part.(type) {
			case *syntax.Lit:
				// Expansion and escaping must never make an unsafe command look literal.
				if strings.ContainsAny(p.Value, "\\*?[]{}~") {
					return "", false
				}
				b.WriteString(p.Value)
			case *syntax.SglQuoted:
				b.WriteString(p.Value)
			case *syntax.DblQuoted:
				value, ok := literal(p.Parts)
				if !ok {
					return "", false
				}
				b.WriteString(value)
			default:
				return "", false
			}
		}
		return b.String(), true
	}
	for _, word := range words {
		value, ok := literal(word.Parts)
		if !ok {
			return nil, false
		}
		out = append(out, value)
	}
	return out, true
}
