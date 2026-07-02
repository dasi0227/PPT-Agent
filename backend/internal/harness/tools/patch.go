package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// PatchTool 对沙箱内某文件做锚定文本替换（patch_slide/patch_design 的共享原语）。
// 语义（ARCH-TOOLS-003/004）：每个 old_text 必须在文件内唯一出现，否则整体失败；
// 全部替换后先跑 validate，不合规则拒绝落盘，均不产生副作用。
type PatchTool struct {
	sandbox   *Sandbox
	validator Validator
}

func NewPatchTool(sandbox *Sandbox, v Validator) *PatchTool {
	if v == nil {
		v = NopValidator{}
	}
	return &PatchTool{sandbox: sandbox, validator: v}
}

func (t *PatchTool) Name() string          { return "patch" }
func (t *PatchTool) Class() Class          { return ClassWrite }
func (t *PatchTool) Scopes() []model.Scope { return nil }

func (t *PatchTool) Description() string {
	return "对指定文件做锚定文本替换。每个 old_text 必须在文件内唯一出现，否则整体失败并要求补充上下文。"
}

func (t *PatchTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"file", "edits"},
		"properties": map[string]any{
			"file": map[string]any{"type": "string", "description": "work_dir 内相对路径"},
			"edits": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":     "object",
					"required": []any{"old_text", "new_text"},
					"properties": map[string]any{
						"old_text": map[string]any{"type": "string", "description": "锚点：文件内唯一的原文片段"},
						"new_text": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func (t *PatchTool) Execute(_ context.Context, args map[string]any) (Result, error) {
	file, ok := args["file"].(string)
	if !ok || file == "" {
		return fail("参数错误：缺少 file"), nil
	}
	edits, err := parseEdits(args["edits"])
	if err != nil {
		return fail(err.Error()), nil
	}

	// 路径边界校验（ARCH-TOOLS-006）：越界整体失败，不写任何文件。
	raw, err := t.sandbox.Read(file)
	if err != nil {
		return fail(fmt.Sprintf("读取失败：%v", err)), nil
	}
	content := string(raw)

	for i, e := range edits {
		n := strings.Count(content, e.oldText)
		if n == 0 {
			return fail(fmt.Sprintf("锚点不存在（edit #%d），请补充上下文", i+1)), nil
		}
		if n > 1 {
			return fail(fmt.Sprintf("锚点不唯一（edit #%d 出现 %d 次），请补充上下文", i+1, n)), nil
		}
		content = strings.Replace(content, e.oldText, e.newText, 1)
	}

	// 落盘前 validate（ARCH-TOOLS-004）：不合规拒绝落盘。
	if okv, reason := t.validator.Validate(file, []byte(content)); !okv {
		return fail(fmt.Sprintf("校验失败，拒绝落盘：%s", reason)), nil
	}

	if err := t.sandbox.Write(file, []byte(content)); err != nil {
		return fail(fmt.Sprintf("写入失败：%v", err)), nil
	}
	return Result{
		OK:          true,
		Observation: fmt.Sprintf("已应用 %d 处替换到 %s", len(edits), file),
		Artifact:    &Artifact{Type: "file", Ref: file},
	}, nil
}

type edit struct {
	oldText string
	newText string
}

func parseEdits(v any) ([]edit, error) {
	list, ok := v.([]any)
	if !ok || len(list) == 0 {
		return nil, fmt.Errorf("参数错误：edits 必须为非空数组")
	}
	out := make([]edit, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("参数错误：edit #%d 非对象", i+1)
		}
		ot, _ := m["old_text"].(string)
		nt, _ := m["new_text"].(string)
		if ot == "" {
			return nil, fmt.Errorf("参数错误：edit #%d 缺少 old_text", i+1)
		}
		out = append(out, edit{oldText: ot, newText: nt})
	}
	return out, nil
}

func fail(reason string) Result {
	return Result{OK: false, Observation: reason}
}
