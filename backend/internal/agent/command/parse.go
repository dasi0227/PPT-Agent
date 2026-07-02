// Package command 是指令的权威解析层（AGENT-CMD-INDEX）：把用户输入行首的指令
// 解析为 Run 的 scope/mode/page_index/instruction/command 字段，供 harness 动态裁剪工具集。
// M4 只落 scope 维度的 /current（默认）与 /page x；mode 维度（prompt/recap/talk/ask）与
// /overview、/repo 留到 M5。
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
	PageIndex   *int   // /page x 显式页号；/current 时为 nil（由调用方用当前预览页填充）
	Instruction string // 去除指令前缀后的自然语言编辑指令
	Command     string // 显式指令名（M4 恒空；mode 指令留到 M5）
}

// ErrMultipleScopes 表示一次输入出现多个 scope 主指令（AGENT-CMD-007 不支持组合）。
var ErrMultipleScopes = fmt.Errorf("一次只能使用一个 scope 指令（/current 或 /page x），不支持组合")

// ErrPageNeedsIndex 表示 /page 缺少页号（AGENT-CMD-002）。
var ErrPageNeedsIndex = fmt.Errorf("/page 指令必须携带页号，如 /page 3")

// ErrUnknownCommand 表示未知 /xxx 指令（AGENT-CMD-004）。
type ErrUnknownCommand struct{ Name string }

func (e *ErrUnknownCommand) Error() string {
	return fmt.Sprintf("未知指令 %q；M4 可用：/current、/page x", e.Name)
}

// knownM5Scopes 是本期（M4）尚未实现、但属已知指令的 scope（给出明确提示而非"未知"）。
var knownM5Scopes = map[string]bool{"/overview": true, "/repo": true}

// knownModes 是 mode 指令（M5 落地）；M4 遇到给出明确提示。
var knownModes = map[string]bool{"/prompt": true, "/recap": true, "/talk": true, "/ask": true}

// Parse 解析用户输入。仅当指令出现在**行首**才识别为指令（AGENT-CMD-001）。
// 无 scope 指令时默认 /current（AGENT-CMD-006）。
func Parse(input string) (Parsed, error) {
	trimmed := strings.TrimSpace(input)

	// 非指令：默认 current，整段作为 instruction。
	if !strings.HasPrefix(trimmed, "/") {
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Instruction: trimmed}, nil
	}

	head, rest := splitHead(trimmed)

	// 越权到 M5 的已知 scope/mode：明确提示，不静默降级（AGENT-CMD-004 的精神）。
	if knownM5Scopes[head] {
		return Parsed{}, fmt.Errorf("指令 %s 属于 M5（跨页/仓库），M4 暂不支持", head)
	}
	if knownModes[head] {
		return Parsed{}, fmt.Errorf("模式指令 %s 属于 M5，M4 暂不支持", head)
	}

	switch head {
	case "/current":
		// /current 后若又出现 scope 指令 → 组合，报错（AGENT-CMD-007）。
		if startsWithScope(rest) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopeCurrent, Mode: model.ModeNormal, Instruction: strings.TrimSpace(rest)}, nil

	case "/page":
		idx, instr, err := parsePageArgs(rest)
		if err != nil {
			return Parsed{}, err
		}
		if startsWithScope(instr) {
			return Parsed{}, ErrMultipleScopes
		}
		return Parsed{Scope: model.ScopePage, Mode: model.ModeNormal, PageIndex: &idx, Instruction: strings.TrimSpace(instr)}, nil

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

// startsWithScope 判断文本是否以 scope 指令开头（用于检测组合）。
func startsWithScope(s string) bool {
	head, _ := splitHead(strings.TrimSpace(s))
	switch head {
	case "/current", "/page", "/overview", "/repo":
		return true
	}
	return false
}
