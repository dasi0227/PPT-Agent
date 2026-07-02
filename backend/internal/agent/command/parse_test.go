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

// AC-CMD-OVERVIEW：/overview → scope=overview，mode=normal，无 command。
func TestOverviewScope(t *testing.T) {
	p, err := Parse("/overview 主色改成品牌蓝")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopeOverview || p.Mode != model.ModeNormal || p.Command != "" {
		t.Errorf("got %+v", p)
	}
	if p.Instruction != "主色改成品牌蓝" {
		t.Errorf("instruction=%q", p.Instruction)
	}
}

// AC-CMD-REPO：/repo → scope=repo，mode=normal，无 command。
func TestRepoScope(t *testing.T) {
	p, err := Parse("/repo 把 neon-card 圆角调大")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Scope != model.ScopeRepo || p.Mode != model.ModeNormal || p.Command != "" {
		t.Errorf("got %+v", p)
	}
	if p.Instruction != "把 neon-card 圆角调大" {
		t.Errorf("instruction=%q", p.Instruction)
	}
}

// /prompt、/recap 是命令而非 mode：scope=current、mode=normal，仅置 Command。
func TestPromptRecapAreCommandsNotModes(t *testing.T) {
	pp, err := Parse("/prompt 我想让这页好看点")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pp.Command != "prompt" || pp.Mode != model.ModeNormal || pp.Scope != model.ScopeCurrent {
		t.Errorf("prompt got %+v", pp)
	}
	if pp.Instruction != "我想让这页好看点" {
		t.Errorf("prompt instruction=%q", pp.Instruction)
	}
	pr, err := Parse("/recap")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pr.Command != "recap" || pr.Mode != model.ModeNormal {
		t.Errorf("recap got %+v", pr)
	}
}

// /talk、/ask 是真正的 mode：写入 Mode，且记 Command。
func TestTalkAskAreModes(t *testing.T) {
	pt, err := Parse("/talk 我想把整体改成深色")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pt.Mode != model.ModeTalk || pt.Command != "talk" {
		t.Errorf("talk got %+v", pt)
	}
	pa, err := Parse("/ask 帮我把这页做成图表")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pa.Mode != model.ModeAsk || pa.Command != "ask" {
		t.Errorf("ask got %+v", pa)
	}
}

// AGENT-CMD-007：任意两个主指令组合仍报错（含新指令）。
func TestNoComboWithM5Commands(t *testing.T) {
	for _, in := range []string{"/overview /repo x", "/page 3 /overview x", "/talk /ask x", "/prompt /recap"} {
		if _, err := Parse(in); !errors.Is(err, ErrMultipleScopes) {
			t.Errorf("want ErrMultipleScopes for %q, got %v", in, err)
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
