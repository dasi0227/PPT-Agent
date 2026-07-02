package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupFile(t *testing.T, name, content string) (*Sandbox, string) {
	t.Helper()
	root := t.TempDir()
	sb, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("sandbox: %v", err)
	}
	if content != "" || name != "" {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatalf("seed file: %v", err)
		}
	}
	return sb, root
}

// AC-TOOLS-003：old_text 唯一时替换成功。
func TestPatchAnchorUnique(t *testing.T) {
	sb, root := setupFile(t, "a.html", "<h1>Hello</h1><p>world</p>")
	p := NewPatchTool(sb, NonEmptyValidator{})
	res, err := p.Execute(context.Background(), map[string]any{
		"file": "a.html",
		"edits": []any{
			map[string]any{"old_text": "Hello", "new_text": "Hi"},
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got observation: %s", res.Observation)
	}
	got, _ := os.ReadFile(filepath.Join(root, "a.html"))
	if string(got) != "<h1>Hi</h1><p>world</p>" {
		t.Errorf("bad content: %q", got)
	}
}

// AC-TOOLS-003：old_text 出现两次 → 整体失败，文件不变。
func TestPatchAnchorNotUnique(t *testing.T) {
	original := "<p>x</p><p>x</p>"
	sb, root := setupFile(t, "a.html", original)
	p := NewPatchTool(sb, NonEmptyValidator{})
	res, err := p.Execute(context.Background(), map[string]any{
		"file":  "a.html",
		"edits": []any{map[string]any{"old_text": "x", "new_text": "y"}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.OK {
		t.Fatal("expected failure for non-unique anchor")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.html")); string(got) != original {
		t.Errorf("file must be unchanged, got %q", got)
	}
}

func TestPatchAnchorMissing(t *testing.T) {
	sb, _ := setupFile(t, "a.html", "<p>x</p>")
	p := NewPatchTool(sb, NonEmptyValidator{})
	res, _ := p.Execute(context.Background(), map[string]any{
		"file":  "a.html",
		"edits": []any{map[string]any{"old_text": "zzz", "new_text": "y"}},
	})
	if res.OK {
		t.Fatal("expected failure for missing anchor")
	}
}

// AC-TOOLS-004：patch 结果违反校验（内容变空）→ 拒绝落盘，文件不变。
func TestValidateBeforeWrite(t *testing.T) {
	original := "keep"
	sb, root := setupFile(t, "a.html", original)
	p := NewPatchTool(sb, NonEmptyValidator{})
	res, err := p.Execute(context.Background(), map[string]any{
		"file":  "a.html",
		"edits": []any{map[string]any{"old_text": "keep", "new_text": ""}},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if res.OK {
		t.Fatal("expected validate failure to reject write")
	}
	if got, _ := os.ReadFile(filepath.Join(root, "a.html")); string(got) != original {
		t.Errorf("file must be unchanged after rejected write, got %q", got)
	}
}

// AC-TOOLS-006：越界路径（.. 与绝对路径）整体拒绝，work_dir 外无写入。
func TestPathBoundary(t *testing.T) {
	sb, _ := setupFile(t, "a.html", "x")
	p := NewPatchTool(sb, NonEmptyValidator{})

	cases := []string{"../escape.html", "../../etc/passwd", "/etc/passwd"}
	for _, bad := range cases {
		res, err := p.Execute(context.Background(), map[string]any{
			"file":  bad,
			"edits": []any{map[string]any{"old_text": "x", "new_text": "y"}},
		})
		if err != nil {
			t.Fatalf("execute %q: %v", bad, err)
		}
		if res.OK {
			t.Errorf("path %q must be rejected", bad)
		}
	}
}

func TestSandboxResolveRejectsEscape(t *testing.T) {
	sb, _ := setupFile(t, "", "")
	if _, err := sb.Resolve("../x"); err == nil {
		t.Error("expected .. rejection")
	}
	if _, err := sb.Resolve("/abs"); err == nil {
		t.Error("expected absolute rejection")
	}
	if _, err := sb.Resolve("ok/child.txt"); err != nil {
		t.Errorf("valid relative path rejected: %v", err)
	}
}

func TestFinishTool(t *testing.T) {
	f := NewFinishTool()
	res, err := f.Execute(context.Background(), map[string]any{"summary": "all done"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !res.IsFinish || res.Summary != "all done" {
		t.Errorf("bad finish result: %+v", res)
	}
}

func TestValidateTool(t *testing.T) {
	sb, _ := setupFile(t, "a.html", "content")
	v := NewValidateTool(sb, NonEmptyValidator{})
	res, _ := v.Execute(context.Background(), map[string]any{"file": "a.html"})
	if !res.OK {
		t.Errorf("expected pass, got %s", res.Observation)
	}
}

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	f := NewFinishTool()
	r.Register(f)
	if got, ok := r.Get("finish"); !ok || got.Name() != "finish" {
		t.Error("registry get failed")
	}
	if len(r.All()) != 1 {
		t.Errorf("want 1 tool, got %d", len(r.All()))
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	r := NewRegistry()
	r.Register(NewFinishTool())
	r.Register(NewFinishTool())
}
