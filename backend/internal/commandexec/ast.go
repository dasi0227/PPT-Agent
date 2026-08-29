package commandexec

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

const (
	maxCommandBytes = 4096
	maxGroups       = 4
	maxStages       = 4
	maxArguments    = 128
)

func Parse(source string) (Graph, error) {
	if strings.TrimSpace(source) == "" || len(source) > maxCommandBytes {
		return Graph{}, commandError(CodeParseInvalid, "command must contain between 1 and 4096 bytes")
	}
	parser := syntax.NewParser(syntax.Variant(syntax.LangPOSIX))
	file, err := parser.Parse(strings.NewReader(source), "")
	if err != nil {
		return Graph{}, commandError(CodeParseInvalid, err.Error())
	}
	if len(file.Stmts) != 1 {
		return Graph{}, commandError(CodeSyntaxDenied, "multiline and semicolon-separated command lists are not supported")
	}
	stmt := file.Stmts[0]
	if stmt.Semicolon.IsValid() || stmt.Background || stmt.Negated || stmt.Coprocess || len(stmt.Redirs) > 0 {
		return Graph{}, commandError(CodeSyntaxDenied, "redirection, background execution, negation, and command separators are not supported")
	}
	graph, err := parseAnd(stmt)
	if err != nil {
		return Graph{}, err
	}
	if len(graph.Groups) == 0 || len(graph.Groups) > maxGroups {
		return Graph{}, commandError(CodeSyntaxDenied, "command contains too many && groups")
	}
	arguments := 0
	for _, group := range graph.Groups {
		if len(group.Commands) == 0 || len(group.Commands) > maxStages {
			return Graph{}, commandError(CodeSyntaxDenied, "pipeline contains too many stages")
		}
		for _, command := range group.Commands {
			arguments += len(command.Args)
		}
	}
	if arguments > maxArguments {
		return Graph{}, commandError(CodeSyntaxDenied, "command contains too many arguments")
	}
	return graph, nil
}

func parseAnd(stmt *syntax.Stmt) (Graph, error) {
	if binary, ok := stmt.Cmd.(*syntax.BinaryCmd); ok {
		if binary.Op != syntax.AndStmt {
			if binary.Op == syntax.Pipe {
				pipeline, err := parsePipeline(stmt)
				return Graph{Groups: []Pipeline{pipeline}}, err
			}
			return Graph{}, commandError(CodeSyntaxDenied, "only | and && operators are supported")
		}
		left, err := parseAnd(binary.X)
		if err != nil {
			return Graph{}, err
		}
		right, err := parseAnd(binary.Y)
		if err != nil {
			return Graph{}, err
		}
		left.Groups = append(left.Groups, right.Groups...)
		return left, nil
	}
	pipeline, err := parsePipeline(stmt)
	return Graph{Groups: []Pipeline{pipeline}}, err
}

func parsePipeline(stmt *syntax.Stmt) (Pipeline, error) {
	if stmt == nil || stmt.Semicolon.IsValid() || stmt.Background || stmt.Negated || stmt.Coprocess || len(stmt.Redirs) > 0 {
		return Pipeline{}, commandError(CodeSyntaxDenied, "unsupported statement syntax")
	}
	if binary, ok := stmt.Cmd.(*syntax.BinaryCmd); ok {
		if binary.Op != syntax.Pipe {
			return Pipeline{}, commandError(CodeSyntaxDenied, "only ordinary stdout pipelines are supported")
		}
		left, err := parsePipeline(binary.X)
		if err != nil {
			return Pipeline{}, err
		}
		right, err := parsePipeline(binary.Y)
		if err != nil {
			return Pipeline{}, err
		}
		left.Commands = append(left.Commands, right.Commands...)
		return left, nil
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Assigns) > 0 || len(call.Args) == 0 {
		return Pipeline{}, commandError(CodeSyntaxDenied, "only simple commands are supported")
	}
	args := make([]string, 0, len(call.Args))
	for _, word := range call.Args {
		value, err := staticWord(word)
		if err != nil {
			return Pipeline{}, err
		}
		args = append(args, value)
	}
	return Pipeline{Commands: []Command{{Args: args}}}, nil
}

func staticWord(word *syntax.Word) (string, error) {
	var value strings.Builder
	for _, part := range word.Parts {
		switch current := part.(type) {
		case *syntax.Lit:
			if strings.ContainsAny(current.Value, "*?[") {
				return "", commandError(CodeSyntaxDenied, "shell glob expansion is not supported")
			}
			value.WriteString(current.Value)
		case *syntax.SglQuoted:
			value.WriteString(current.Value)
		case *syntax.DblQuoted:
			for _, quotedPart := range current.Parts {
				literal, ok := quotedPart.(*syntax.Lit)
				if !ok {
					return "", commandError(CodeSyntaxDenied, "variables and substitutions are not supported")
				}
				value.WriteString(literal.Value)
			}
		default:
			return "", commandError(CodeSyntaxDenied, "variables, substitutions, expansions, and process substitutions are not supported")
		}
	}
	return value.String(), nil
}
