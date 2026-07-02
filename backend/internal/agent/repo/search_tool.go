package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// SearchAssetsTool 按 kind + 关键词检索个人仓库资产索引（search_assets，只读）。
// 返回资产元数据列表（不含完整载荷），供 LLM 定位要操作的资产。仅 repo scope。
type SearchAssetsTool struct {
	store Store
}

func NewSearchAssetsTool(store Store) *SearchAssetsTool { return &SearchAssetsTool{store: store} }

func (t *SearchAssetsTool) Name() string          { return "search_assets" }
func (t *SearchAssetsTool) Class() tools.Class    { return tools.ClassRead }
func (t *SearchAssetsTool) Scopes() []model.Scope { return []model.Scope{model.ScopeRepo} }

func (t *SearchAssetsTool) Description() string {
	return "按 kind 与关键词检索个人仓库资产，返回资产索引（id/name/kind/描述，不含完整载荷）。"
}

func (t *SearchAssetsTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"query"},
		"properties": map[string]any{
			"kind":  map[string]any{"type": "string", "enum": []any{"layout", "component", "theme", "fx"}, "description": "资产类型过滤，可空"},
			"query": map[string]any{"type": "string", "description": "语义关键词（匹配 name/description/tags）"},
		},
	}
}

func (t *SearchAssetsTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	kind, _ := args["kind"].(string)
	query, _ := args["query"].(string)
	assets, err := t.store.ListAssets(ctx, kind)
	if err != nil {
		return fail("检索失败：" + err.Error()), nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	var lines []string
	for _, a := range assets {
		if q != "" && !matchAsset(a, q) {
			continue
		}
		lines = append(lines, fmt.Sprintf("- id=%s name=%q kind=%s source=%s desc=%q", a.ID, a.Name, a.Kind, a.Source, a.Description))
	}
	if len(lines) == 0 {
		return tools.Result{OK: true, Observation: "未找到匹配资产"}, nil
	}
	return tools.Result{OK: true, Observation: fmt.Sprintf("命中 %d 个资产：\n%s", len(lines), strings.Join(lines, "\n"))}, nil
}

func matchAsset(a model.Asset, q string) bool {
	if strings.Contains(strings.ToLower(a.Name), q) || strings.Contains(strings.ToLower(a.Description), q) {
		return true
	}
	for _, tag := range a.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

func fail(reason string) tools.Result {
	return tools.Result{OK: false, Observation: reason}
}
