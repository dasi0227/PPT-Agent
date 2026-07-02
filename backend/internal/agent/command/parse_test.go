package command

import (
	"errors"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// AC-CMD-INDEX-001 / AC-CMD-CURRENT-001：无指令 → 默认 current，整段为 instruction。
func TestDefaultCurrentScope(t *testing.T) {
	p, err := Parse("把标题改大")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopeCurrent {
		t.Errorf("scope=%q, want current", p.Scope)
	}
	if p.PageIndex != nil {
		t.Errorf("current scope page_index should be nil (caller fills current page), got %v", *p.PageIndex)
	}
	if p.Instruction != "把标题改大" {
		t.Errorf("instruction=%q", p.Instruction)
	}
}

// /current 显式前缀等价于默认 current。
func TestExplicitCurrent(t *testing.T) {
	p, err := Parse("/current 把标题改大")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopeCurrent || p.Instruction != "把标题改大" {
		t.Errorf("got %+v", p)
	}
}

// AC-CMD-PAGE-001：/page x → scope=page, page_index=x, 余下为 instruction。
func TestPageScopeParse(t *testing.T) {
	p, err := Parse("/page 3 把标题改大一号")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopePage {
		t.Errorf("scope=%q, want page", p.Scope)
	}
	if p.PageIndex == nil || *p.PageIndex != 3 {
		t.Errorf("page_index=%v, want 3", p.PageIndex)
	}
	if p.Instruction != "把标题改大一号" {
		t.Errorf("instruction=%q", p.Instruction)
	}
}

// SPEC-CMD-PAGE-005：仅 /page x（无指令文本）→ 跳转，无 instruction。
func TestPageWithoutInstruction(t *testing.T) {
	p, err := Parse("/page 2")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.PageIndex == nil || *p.PageIndex != 2 || p.Instruction != "" {
		t.Errorf("got %+v", p)
	}
}

// AGENT-CMD-002：/page 缺页号 → 报错。
func TestPageNeedsIndex(t *testing.T) {
	if _, err := Parse("/page 把标题改大"); !errors.Is(err, ErrPageNeedsIndex) {
		t.Errorf("want ErrPageNeedsIndex, got %v", err)
	}
	if _, err := Parse("/page"); !errors.Is(err, ErrPageNeedsIndex) {
		t.Errorf("want ErrPageNeedsIndex for bare /page, got %v", err)
	}
}

// AGENT-CMD-007：多个 scope 指令 → 报错要求单选。
func TestNoCombo(t *testing.T) {
	if _, err := Parse("/page 3 /current 改"); !errors.Is(err, ErrMultipleScopes) {
		t.Errorf("want ErrMultipleScopes, got %v", err)
	}
}

// AGENT-CMD-004：未知指令 → 明确提示。
func TestUnknownCommand(t *testing.T) {
	var unknown *ErrUnknownCommand
	if _, err := Parse("/bogus x"); !errors.As(err, &unknown) {
		t.Errorf("want ErrUnknownCommand, got %v", err)
	}
}

// M5 scope/mode 指令在 M4 给出明确提示（不静默降级）。
func TestM5CommandsRejected(t *testing.T) {
	for _, in := range []string{"/overview 改主色", "/repo 改组件", "/talk 聊聊", "/ask 问", "/prompt 改写", "/recap"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("expected rejection for M5 command %q", in)
		}
	}
}

// AGENT-CMD-001：指令必须在行首；非行首的 / 不识别为指令。
func TestCommandMustBeLineHead(t *testing.T) {
	p, err := Parse("请用 /page 语法")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopeCurrent || p.Instruction != "请用 /page 语法" {
		t.Errorf("non-head slash must be treated as plain text, got %+v", p)
	}
}
