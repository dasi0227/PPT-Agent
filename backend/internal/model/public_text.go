package model

import (
	"regexp"
	"sort"
	"strings"
)

// PublicTextContext supplies display names, never authorization. SourceText is
// user-authored text whose technical vocabulary must survive presentation.
type PublicTextContext struct {
	Pages        map[string]string
	HiddenValues []string
	SourceText   string
}

var publicResourceRef = regexp.MustCompile(`\bslide:([A-Za-z0-9_-]+):(spec|html)\b|\b(sli_[A-Za-z0-9_-]+)\.(html)\b`)
var publicSlideID = regexp.MustCompile(`\bsli_[A-Za-z0-9_-]+\b`)
var publicInternalID = regexp.MustCompile(`\b(?:pro|prj|proj|run|thr|thread|loop)_[A-Za-z0-9_-]+\b`)
var publicVocabulary = regexp.MustCompile(`(?:\.(?:manifest|outline|design|spec)\.json\b|\b(?:completion gate|deck:manifest|deck:outline|deck:design|manifest\.json|outline\.json|design\.json|spec\.json|read_resource|edit_manifest|edit_design|edit_spec|init_outline|arrange_outline|write_html|patch_html|render_slide|create_plan|update_plan|review_completion|ask_user|RunCommand|RunScope|RunMode|RunPhase)\b)`)

var publicErrorToken = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

var publicTerms = map[string]string{
	"completion gate": "完成检查", "deck:manifest": "内容要求", ".manifest.json": "内容要求", "manifest.json": "内容要求",
	"deck:outline": "目录结构", ".outline.json": "目录结构", "outline.json": "目录结构",
	"deck:design": "视觉要求", ".design.json": "视觉要求", "design.json": "视觉要求", ".spec.json": "规格要求", "spec.json": "规格要求",
	"read_resource": "读取演示内容", "edit_manifest": "编辑内容要求", "edit_design": "编辑视觉要求", "edit_spec": "编辑规格要求", "init_outline": "初始化目录结构", "arrange_outline": "编排目录结构", "write_html": "生成幻灯片", "patch_html": "编辑幻灯片", "render_slide": "页面渲染检查",
	"create_plan": "制定计划", "update_plan": "更新计划", "review_completion": "完成检查", "ask_user": "提问",
	"RunCommand": "任务设置", "RunScope": "修改范围", "RunMode": "工作模式", "RunPhase": "任务阶段",
}

// PublicText translates known product references, not arbitrary English words,
// error-shaped tokens or user code. Protocol fields and URLs must not call it.
func PublicText(text string, contexts ...PublicTextContext) string {
	c := PublicTextContext{}
	if len(contexts) > 0 {
		c = contexts[0]
	}
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	hidden := append([]string{}, c.HiddenValues...)
	sort.SliceStable(hidden, func(i, j int) bool { return len(hidden[i]) > len(hidden[j]) })
	for _, value := range hidden {
		if value == "" {
			continue
		}
		pattern := regexp.MustCompile(regexp.QuoteMeta(value))
		var clean strings.Builder
		offset := 0
		for _, span := range pattern.FindAllStringIndex(text, -1) {
			clean.WriteString(text[offset:span[0]])
			leftBound := span[0] == 0 || !identifierByte(text[span[0]-1]) || !identifierByte(value[0])
			rightBound := span[1] == len(text) || !identifierByte(text[span[1]]) || !identifierByte(value[len(value)-1])
			if leftBound && rightBound {
				clean.WriteString("当前项目")
			} else {
				clean.WriteString(value)
			}
			offset = span[1]
		}
		clean.WriteString(text[offset:])
		text = clean.String()
	}
	page := func(id string) string {
		if name := c.Pages[id]; name != "" {
			return name
		}
		return "相关页面"
	}
	lines := strings.Split(text, "\n")
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		line = publicResourceRef.ReplaceAllStringFunc(line, func(ref string) string {
			m := publicResourceRef.FindStringSubmatch(ref)
			id, part := m[1], m[2]
			if id == "" {
				id, part = m[3], m[4]
			}
			label := "幻灯片"
			if part == "spec" || part == "spec.json" {
				label = "规格要求"
			}
			return page(id) + " · " + label
		})
		line = publicSlideID.ReplaceAllStringFunc(line, func(id string) string {
			if strings.Contains(c.SourceText, id) {
				return id
			}
			return page(id)
		})
		line = publicInternalID.ReplaceAllStringFunc(line, func(id string) string {
			if strings.Contains(c.SourceText, id) {
				return id
			}
			return "内部标识"
		})
		var out strings.Builder
		offset := 0
		for _, span := range publicVocabulary.FindAllStringIndex(line, -1) {
			term := line[span[0]:span[1]]
			out.WriteString(line[offset:span[0]])
			// Preserve arbitrary user paths and addresses, not partial translations.
			if strings.Contains(c.SourceText, term) || (span[0] > 0 && line[span[0]-1] == '/') {
				out.WriteString(term)
			} else {
				out.WriteString(publicTerms[term])
			}
			offset = span[1]
		}
		out.WriteString(line[offset:])
		lines[i] = publicErrorToken.ReplaceAllStringFunc(out.String(), func(code string) string {
			if def, ok := errorDefinitions[code]; ok && !strings.Contains(c.SourceText, code) {
				return def.SafeMessage
			}
			return code
		})
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func identifierByte(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_' || value == '-'
}
