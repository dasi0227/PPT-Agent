package assetops

import (
	"bytes"
	"context"
	"fmt"
	stdhtml "html"
	"path"
	"path/filepath"
	"strings"

	nethtml "golang.org/x/net/html"

	"github.com/dasi0227/PPT-Agent/backend/internal/asset"
	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
	"github.com/dasi0227/PPT-Agent/backend/internal/harness/tools"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

// MountAssetTool 将 layout/component/fx 资产移植进指定页。成功后产生 slide 版本。
type MountAssetTool struct {
	store       Store
	projectRoot string
	workRoot    string
	projectID   string
	runID       string
	lockedIdx   *int
	clock       func() int64
	newID       func() string
	mounted     bool
}

func NewMountAssetTool(store Store, projectRoot, workRoot, projectID, runID string, lockedIdx *int, clock func() int64, newID func() string) *MountAssetTool {
	return &MountAssetTool{
		store: store, projectRoot: projectRoot, workRoot: workRoot, projectID: projectID, runID: runID,
		lockedIdx: lockedIdx, clock: clock, newID: newID,
	}
}

func (t *MountAssetTool) Name() string       { return "mount_asset" }
func (t *MountAssetTool) Class() tools.Class { return tools.ClassWrite }
func (t *MountAssetTool) Scopes() []model.Scope {
	return []model.Scope{model.ScopeCurrent, model.ScopePage, model.ScopeOverview}
}
func (t *MountAssetTool) Mounted() bool { return t.mounted }

func (t *MountAssetTool) Description() string {
	return "按资产 manifest 的挂载约定，将 layout/component/fx 资产注入指定页；写入前必须通过 html-output-spec 校验，成功后产生页版本。"
}

func (t *MountAssetTool) Parameters() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"slide_idx", "asset_id"},
		"properties": map[string]any{
			"slide_idx": map[string]any{"type": "integer", "minimum": 0},
			"asset_id":  map[string]any{"type": "string"},
			"params":    map[string]any{"type": "object", "description": "按资产 params schema 填参；缺省走 default"},
		},
	}
}

func (t *MountAssetTool) Execute(ctx context.Context, args map[string]any) (tools.Result, error) {
	idx, ok := toInt(args["slide_idx"])
	if !ok || idx < 0 {
		return fail("参数错误：slide_idx 必须为非负整数"), nil
	}
	if t.lockedIdx != nil && idx != *t.lockedIdx {
		return fail(fmt.Sprintf("越权：本次编辑仅能改第 %d 页，收到 slide_idx=%d", *t.lockedIdx, idx)), nil
	}
	assetID, _ := args["asset_id"].(string)
	if assetID == "" {
		return fail("参数错误：asset_id 为空"), nil
	}
	params, err := parseParams(args["params"])
	if err != nil {
		return fail(err.Error()), nil
	}

	projectSandbox, err := tools.NewSandbox(t.projectRoot)
	if err != nil {
		return tools.Result{}, err
	}
	assetSandbox, err := tools.NewSandbox(t.workRoot)
	if err != nil {
		return tools.Result{}, err
	}
	a, err := t.store.GetAsset(ctx, assetID)
	if err != nil {
		return fail("资产不存在：" + err.Error()), nil
	}
	manifestRaw, err := assetSandbox.Read(a.ManifestPath)
	if err != nil {
		return fail("读取 manifest 失败：" + err.Error()), nil
	}
	if err := asset.ValidateManifestRaw(manifestRaw); err != nil {
		return fail("manifest schema 校验未通过：" + err.Error()), nil
	}
	m, err := asset.Parse(manifestRaw)
	if err != nil {
		return fail("解析 manifest 失败：" + err.Error()), nil
	}
	if m.Kind == asset.KindTheme {
		return fail("theme 资产请使用 apply_theme，不可 mount 到单页"), nil
	}

	rel := fmt.Sprintf("slides/%03d/index.html", idx)
	currentRaw, err := projectSandbox.Read(rel)
	if err != nil {
		return fail(fmt.Sprintf("读取第 %d 页失败：%v", idx, err)), nil
	}
	nextHTML, err := t.renderMountedHTML(idx, string(currentRaw), a, m, params, assetSandbox)
	if err != nil {
		return fail(err.Error()), nil
	}
	if reason := validateMountedHTMLStructure(nextHTML); reason != "" {
		return fail("html 关键结构校验失败，拒绝落盘：" + reason), nil
	}
	if ok, reason := designsystem.LintSlideResult([]byte(nextHTML)); !ok {
		return fail("html-output-spec 校验失败，拒绝落盘：" + reason), nil
	}
	if err := projectSandbox.Write(rel, []byte(nextHTML)); err != nil {
		return fail("写入页失败：" + err.Error()), nil
	}
	versionNo, err := t.snapshotSlide(ctx, projectSandbox, idx, nextHTML)
	if err != nil {
		_ = projectSandbox.Write(rel, currentRaw)
		return tools.Result{}, err
	}
	t.mounted = true
	return tools.Result{
		OK:          true,
		Observation: fmt.Sprintf("资产 %s 已挂载到第 %d 页并通过校验，版本 v%d", a.Name, idx, versionNo),
		Artifact:    &tools.Artifact{Type: "slide_html", Ref: rel},
	}, nil
}

func (t *MountAssetTool) renderMountedHTML(idx int, current string, a model.Asset, m asset.Manifest, params map[string]any, assetSandbox *tools.Sandbox) (string, error) {
	switch m.Kind {
	case asset.KindLayout:
		return t.renderLayout(a, m, params, assetSandbox)
	case asset.KindComponent:
		return t.renderComponent(current, a, m, params, assetSandbox)
	case asset.KindFx:
		return t.renderFX(idx, current, a, m)
	default:
		return "", fmt.Errorf("不支持 mount kind=%s", m.Kind)
	}
}

func (t *MountAssetTool) renderLayout(a model.Asset, m asset.Manifest, params map[string]any, assetSandbox *tools.Sandbox) (string, error) {
	if m.Assets.HTML == "" {
		return "", fmt.Errorf("layout 资产缺少 html 载荷")
	}
	raw, err := assetSandbox.Read(path.Join(a.Dir, m.Assets.HTML))
	if err != nil {
		return "", fmt.Errorf("读取 layout html 失败：%w", err)
	}
	html := applyParams(string(raw), m.Params, params)
	if m.Assets.CSS != "" {
		css, err := assetSandbox.Read(path.Join(a.Dir, m.Assets.CSS))
		if err != nil {
			return "", fmt.Errorf("读取 layout css 失败：%w", err)
		}
		html = embedStyleInFullHTML(html, a.Name, string(css))
	}
	return html, nil
}

func (t *MountAssetTool) renderComponent(current string, a model.Asset, m asset.Manifest, params map[string]any, assetSandbox *tools.Sandbox) (string, error) {
	if m.Mount == nil {
		return "", fmt.Errorf("component 资产缺少 mount 约定")
	}
	raw, err := assetSandbox.Read(path.Join(a.Dir, m.Assets.HTML))
	if err != nil {
		return "", fmt.Errorf("读取 component html 失败：%w", err)
	}
	fragment := applyParams(string(raw), m.Params, params)

	doc, err := nethtml.Parse(strings.NewReader(current))
	if err != nil {
		return "", fmt.Errorf("解析 slide html 失败：%w", err)
	}
	if m.Assets.CSS != "" {
		css, err := assetSandbox.Read(path.Join(a.Dir, m.Assets.CSS))
		if err != nil {
			return "", fmt.Errorf("读取 component css 失败：%w", err)
		}
		appendStyle(doc, a.Name, string(css))
	}
	target := findTarget(doc, m.Mount.Target)
	if target == nil {
		return "", fmt.Errorf("挂载目标不存在：%s", m.Mount.Target)
	}
	nodes, err := parseFragment(fragment)
	if err != nil {
		return "", err
	}
	insertNodes(target, nodes, m.Mount.Position)
	return renderHTML(doc), nil
}

func (t *MountAssetTool) renderFX(idx int, current string, a model.Asset, m asset.Manifest) (string, error) {
	if m.Mount == nil {
		return "", fmt.Errorf("fx 资产缺少 mount 约定")
	}
	doc, err := nethtml.Parse(strings.NewReader(current))
	if err != nil {
		return "", fmt.Errorf("解析 slide html 失败：%w", err)
	}
	target := findTarget(doc, m.Mount.Target)
	if target == nil {
		stage := findTarget(doc, "slide-root")
		if stage == nil {
			return "", fmt.Errorf("slide-stage 不存在，无法挂载 fx")
		}
		target = &nethtml.Node{
			Type: nethtml.ElementNode,
			Data: "div",
			Attr: []nethtml.Attribute{
				{Key: "class", Val: "asset-fx-host"},
				{Key: "data-fx", Val: string(m.Name)},
			},
		}
		stage.AppendChild(target)
	}
	if m.Assets.JS == "" {
		return "", fmt.Errorf("fx 资产缺少 js 载荷")
	}
	// M6 后端只生成产物路径；M7 预览静态服务需同时暴露 project work_dir 与全局 _assets/。
	// 若 M7 改为统一 /assets/... 路由，这里应替换为稳定服务端 URL。
	scriptRel, err := filepath.Rel(
		filepath.Join(t.projectRoot, "slides", fmt.Sprintf("%03d", idx)),
		filepath.Join(t.workRoot, filepath.FromSlash(path.Join(a.Dir, m.Assets.JS))),
	)
	if err != nil {
		return "", err
	}
	appendModuleScript(doc, filepath.ToSlash(scriptRel))
	return renderHTML(doc), nil
}

func (t *MountAssetTool) snapshotSlide(ctx context.Context, sandbox *tools.Sandbox, idx int, html string) (int, error) {
	target := model.SlideVersionTarget(t.projectID, idx)
	no, err := t.store.NextVersionNo(ctx, "slide", target)
	if err != nil {
		return 0, err
	}
	snap := fmt.Sprintf("versions/slide-%03d/v%d.html", idx, no)
	if err := sandbox.Write(snap, []byte(html)); err != nil {
		return 0, err
	}
	if err := t.store.CreateVersion(ctx, model.Version{
		ID: t.newID(), TargetType: "slide", TargetID: target, VersionNo: no,
		SnapshotPath: snap, RunID: t.runID, CreatedAt: t.clock(),
	}); err != nil {
		return 0, err
	}
	if err := t.store.SetSlideVersion(ctx, t.projectID, idx, no); err != nil {
		_ = t.store.DeleteVersion(ctx, "slide", target, no)
		_ = sandbox.Delete(snap)
		return 0, err
	}
	return no, nil
}

func parseParams(v any) (map[string]any, error) {
	if v == nil {
		return map[string]any{}, nil
	}
	mp, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("参数错误：params 必须为对象")
	}
	return mp, nil
}

func applyParams(s string, schema map[string]asset.Param, values map[string]any) string {
	out := s
	for name, p := range schema {
		value := p.Default
		if v, ok := values[name]; ok {
			value = v
		}
		repl := stdhtml.EscapeString(fmt.Sprint(value))
		out = strings.ReplaceAll(out, "{{"+name+"}}", repl)
	}
	return out
}

func validateMountedHTMLStructure(html string) string {
	lower := strings.ToLower(html)
	for _, required := range []string{"<!doctype html", "<html", "<head", "<body"} {
		if !strings.Contains(lower, required) {
			return "缺少 " + required
		}
	}
	if !strings.Contains(html, "slide-stage") {
		return "缺少 .slide-stage"
	}
	if !strings.Contains(html, "tokens.css") || !strings.Contains(html, "base.css") {
		return "缺少公共样式层引用"
	}
	return ""
}

func embedStyleInFullHTML(html, name, css string) string {
	style := fmt.Sprintf(`<style data-asset-style="%s">%s</style>`, stdhtml.EscapeString(name), css)
	repls := []string{
		`<link rel="stylesheet" href="style.css">`,
		`<link href="style.css" rel="stylesheet">`,
		`<link rel='stylesheet' href='style.css'>`,
	}
	for _, old := range repls {
		if strings.Contains(html, old) {
			return strings.Replace(html, old, style, 1)
		}
	}
	if strings.Contains(html, "</head>") {
		return strings.Replace(html, "</head>", style+"</head>", 1)
	}
	return style + html
}

func appendStyle(doc *nethtml.Node, name, css string) {
	head := findElement(doc, "head")
	if head == nil {
		return
	}
	style := &nethtml.Node{Type: nethtml.ElementNode, Data: "style", Attr: []nethtml.Attribute{{Key: "data-asset-style", Val: name}}}
	style.AppendChild(&nethtml.Node{Type: nethtml.TextNode, Data: css})
	head.AppendChild(style)
}

func appendModuleScript(doc *nethtml.Node, src string) {
	body := findElement(doc, "body")
	if body == nil {
		return
	}
	script := &nethtml.Node{Type: nethtml.ElementNode, Data: "script", Attr: []nethtml.Attribute{{Key: "type", Val: "module"}, {Key: "data-asset-fx", Val: src}}}
	script.AppendChild(&nethtml.Node{Type: nethtml.TextNode, Data: fmt.Sprintf("import { init } from %q;\ninit(document);\n", src)})
	body.AppendChild(script)
}

func parseFragment(fragment string) ([]*nethtml.Node, error) {
	doc, err := nethtml.Parse(strings.NewReader("<!doctype html><html><body>" + fragment + "</body></html>"))
	if err != nil {
		return nil, fmt.Errorf("解析资产片段失败：%w", err)
	}
	body := findElement(doc, "body")
	if body == nil {
		return nil, fmt.Errorf("解析资产片段失败：缺少 body")
	}
	var nodes []*nethtml.Node
	for body.FirstChild != nil {
		n := body.FirstChild
		body.RemoveChild(n)
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func insertNodes(target *nethtml.Node, nodes []*nethtml.Node, position string) {
	switch position {
	case "prepend":
		first := target.FirstChild
		for _, n := range nodes {
			target.InsertBefore(n, first)
		}
	case "replace":
		parent := target.Parent
		if parent == nil {
			return
		}
		for _, n := range nodes {
			parent.InsertBefore(n, target)
		}
		parent.RemoveChild(target)
	case "wrap":
		if len(nodes) == 0 || target.Parent == nil {
			return
		}
		wrapper := nodes[0]
		parent := target.Parent
		parent.InsertBefore(wrapper, target)
		parent.RemoveChild(target)
		wrapper.AppendChild(target)
	default:
		for _, n := range nodes {
			target.AppendChild(n)
		}
	}
}

func findTarget(doc *nethtml.Node, selector string) *nethtml.Node {
	selector = strings.TrimSpace(selector)
	if selector == "" || selector == "slide-root" {
		return findByClass(doc, "slide-stage")
	}
	if strings.HasPrefix(selector, ".") {
		return findByClass(doc, strings.TrimPrefix(selector, "."))
	}
	if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
		body := strings.TrimSuffix(strings.TrimPrefix(selector, "["), "]")
		parts := strings.SplitN(body, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			return findByAttr(doc, key, val)
		}
	}
	return nil
}

func findElement(n *nethtml.Node, name string) *nethtml.Node {
	if n.Type == nethtml.ElementNode && n.Data == name {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := findElement(c, name); got != nil {
			return got
		}
	}
	return nil
}

func findByClass(n *nethtml.Node, class string) *nethtml.Node {
	if n.Type == nethtml.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "class" && hasClass(a.Val, class) {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := findByClass(c, class); got != nil {
			return got
		}
	}
	return nil
}

func findByAttr(n *nethtml.Node, key, val string) *nethtml.Node {
	if n.Type == nethtml.ElementNode {
		for _, a := range n.Attr {
			if a.Key == key && a.Val == val {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if got := findByAttr(c, key, val); got != nil {
			return got
		}
	}
	return nil
}

func hasClass(classes, want string) bool {
	for _, c := range strings.Fields(classes) {
		if c == want {
			return true
		}
	}
	return false
}

func renderHTML(n *nethtml.Node) string {
	var b bytes.Buffer
	_ = nethtml.Render(&b, n)
	return b.String()
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
