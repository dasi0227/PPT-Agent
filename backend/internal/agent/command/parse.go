// Package command 是指令的权威解析层（AGENT-CMD-INDEX）：把用户输入行首的指令
// 解析为 Run 的 scope/mode/page_index/instruction/command 字段，供 harness 动态裁剪工具集。
// scope 维度：/current（默认）、/page x、/overview、/repo。
// 命令/模式维度：/prompt、/recap 是一次性命令（command，mode 仍 normal）；
// /talk、/ask 是真正的模式（写 runs.mode）。映射见 commands/README.md「指令 → Run 字段映射」。
package command

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// Parsed 是一次指令解析结果（尚未落库的 Run 意图）。
type Parsed struct {
	Scope       model.Scope
	Mode        model.Mode
	PageIndex   *int   // /page x 显式页号；其余 scope 为 nil（current 由调用方用当前预览页填充）
	Instruction string // 去除指令前缀后的自然语言指令
	Command     string // 显式命令名：prompt/recap/talk/ask；scope 类指令为空
}

// ErrMultipleScopes 表示一次输入出现多个 scope 主指令（AGENT-CMD-007 不支持组合）。
var ErrMultipleScopes = fmt.Errorf("一次只能使用一个 scope 指令（/current、/page x、/overview、/repo），不支持组合")

// ErrPageNeedsIndex 表示 /page 缺少页号（AGENT-CMD-002）。
var ErrPageNeedsIndex = fmt.Errorf("/page 指令必须携带页号，如 /page 3")

// ErrUnknownCommand 表示未知 /xxx 指令（AGENT-CMD-004）。
type ErrUnknownCommand struct{ Name string }

func (e *ErrUnknownCommand) Error() string {
	return fmt.Sprintf("未知指令 %q；可用 scope：/current、/page x、/overview、/repo；可用命令：/prompt、/recap、/talk、/ask", e.Name)
}

// Parse 解析用户输入。仅当指令出现在**行首**才识别为指令（AGENT-CMD-001）。
// 无 scope 指令时默认 /current（AGENT-CMD-006）。
func Parse(input string) (Parsed, error) {
	trimmed := strings.TrimSpace(input)

	// 非指令：默认 current，整段作为 instruction。
	if !strings.HasPrefix(trimmed, "/") {
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Instruction: trimmed}, nil
	}

	head, rest := splitHead(trimmed)

	switch head {
	case "/current":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Instruction: strings.TrimSpace(rest)}, nil

	case "/page":
		idx, instr, err := parsePageArgs(rest)
		if err != nil {
			return Parsed{}, err
		}
		if startsWithCommand(instr) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopePage, Mode: model.ModeNormal, PageIndex: &idx, Instruction: strings.TrimSpace(instr)}, nil

	case "/overview":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeOverview, Mode: model.ModeNormal, Instruction: strings.TrimSpace(rest)}, nil

	case "/repo":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeRepo, Mode: model.ModeNormal, Instruction: strings.TrimSpace(rest)}, nil

	// /prompt、/recap 是一次性命令（不是 mode）：scope 归 current、mode 归 normal，仅置 Command。
	case "/prompt":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Command: "prompt", Instruction: strings.TrimSpace(rest)}, nil

	case "/recap":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Command: "recap", Instruction: strings.TrimSpace(rest)}, nil

	// /talk、/ask 是真正的 mode：写 runs.mode，同时记 Command 以便审计。
	case "/talk":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeTalk, Command: "talk", Instruction: strings.TrimSpace(rest)}, nil

	case "/ask":
		if startsWithCommand(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeAsk, Command: "ask", Instruction: strings.TrimSpace(rest)}, nil

	default:
		return Parsed{}, &ErrUnknownCommand{Name: head}
	}
}

// splitHead 取行首第一个 token（指令名）与其余部分。
func splitHead(s string) (head, rest string) {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// parsePageArgs 从 /page 之后解析页号与 instruction。缺页号或页号非法 → 报错。
func parsePageArgs(rest string) (int, string, error) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return 0, "", ErrPageNeedsIndex
	}
	numTok, instr := splitHead(rest)
	idx, err := strconv.Atoi(numTok)
	if err != nil {
		return 0, "", ErrPageNeedsIndex
	}
	if idx < 0 {
		return 0, "", fmt.Errorf("页号必须为非负整数，收到 %d", idx)
	}
	return idx, instr, nil
}

// startsWithCommand 判断文本是否以任一主指令（scope 或命令/模式）开头，用于检测组合（AGENT-CMD-007）。
func startsWithCommand(s string) bool {
	head, _ := splitHead(strings.TrimSpace(s))
	switch head {
	case "/current", "/page", "/overview", "/repo",
		"/prompt", "/recap", "/talk", "/ask":
		return true
	}
	return false
}
