package contextengine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	pptspec "github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

// ProjectResourceSnapshot is the current file-backed context shared by commands.
type ProjectResourceSnapshot struct {
	Meta             *pptspec.Meta
	MetaState        string
	Constraints      *pptspec.Constraints
	ConstraintsState string
	Outline          *pptspec.Outline
	OutlineState     string
	Slides           []SlideResourceSummary
}

type SlideResourceSummary struct {
	Ordinal   int
	Title     string
	Core      string
	Purpose   string
	SpecState string
	HTMLState string
}

func LoadProjectResourceSnapshot(workDir string) (ProjectResourceSnapshot, error) {
	var snapshot ProjectResourceSnapshot
	meta, state, err := readProjectResource(workDir, ".meta.json", "meta")
	if err != nil {
		return snapshot, err
	}
	snapshot.MetaState = state
	snapshot.Meta, _ = meta.(*pptspec.Meta)
	constraints, state, err := readProjectResource(workDir, ".constraints.json", "constraints")
	if err != nil {
		return snapshot, err
	}
	snapshot.ConstraintsState = state
	snapshot.Constraints, _ = constraints.(*pptspec.Constraints)
	outline, state, err := readProjectResource(workDir, ".outline.json", "outline")
	if err != nil {
		return snapshot, err
	}
	snapshot.OutlineState = state
	snapshot.Outline, _ = outline.(*pptspec.Outline)
	if snapshot.Outline == nil {
		return snapshot, nil
	}
	for _, loc := range pptspec.FlattenOutline(*snapshot.Outline) {
		value, state, err := readProjectResource(workDir, model.SlideSpecPath(loc.Slide.ID), "spec")
		if err != nil {
			return snapshot, err
		}
		summary := SlideResourceSummary{Ordinal: loc.Ordinal, Title: loc.Slide.Title, SpecState: state, HTMLState: "缺失"}
		if slide, ok := value.(*pptspec.SlideSpec); ok {
			summary.Core, summary.Purpose = slide.Core, string(slide.Purpose)
		}
		// Existence is the only HTML fact supplied to commands. Do not read or
		// validate HTML here, and do not treat existence as a rendering result.
		info, err := os.Stat(filepath.Join(workDir, model.SlideHTMLPath(loc.Slide.ID)))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return snapshot, err
		}
		if err == nil && info.Mode().IsRegular() {
			summary.HTMLState = "存在"
		}
		snapshot.Slides = append(snapshot.Slides, summary)
	}
	return snapshot, nil
}

func readProjectResource(workDir, path, kind string) (any, string, error) {
	raw, err := os.ReadFile(filepath.Join(workDir, path))
	if errors.Is(err, os.ErrNotExist) {
		return nil, "缺失", nil
	}
	if err != nil {
		return nil, "", err
	}
	value, err := pptspec.ParseStrictSourceJSON(raw, kind)
	if err != nil {
		return nil, "无效", nil
	}
	return value, "可用", nil
}

func CompileProjectResourceContext(snapshot ProjectResourceSnapshot) string {
	var text strings.Builder
	field := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			text.WriteString(label + "：" + EscapeCommandContextText(value) + "\n")
		}
	}
	list := func(label string, values []string) {
		items := []string{}
		for _, value := range values {
			if strings.TrimSpace(value) != "" {
				items = append(items, "- "+EscapeCommandContextText(value)+"\n")
			}
		}
		if len(items) > 0 {
			text.WriteString(label + "：\n" + strings.Join(items, ""))
		}
	}
	text.WriteString("<project_context>\n<meta>\n")
	if meta := snapshot.Meta; meta != nil {
		field("标题", meta.Title)
		field("语言", meta.Language)
		field("页数要求", meta.Count)
		field("受众", meta.Audience)
		field("目标", meta.Goal)
	} else {
		field("状态", snapshot.MetaState)
	}
	text.WriteString("</meta>\n<constraints>\n")
	if constraints := snapshot.Constraints; constraints != nil {
		list("内容要求", constraints.Content.Requirements)
		list("内容限制", constraints.Content.Prohibitions)
		list("视觉要求", constraints.Visual.Requirements)
		list("视觉限制", constraints.Visual.Prohibitions)
	} else {
		field("状态", snapshot.ConstraintsState)
	}
	text.WriteString("</constraints>\n<outline>\n")
	if outline := snapshot.Outline; outline != nil {
		page := 0
		for i, section := range outline.Sections {
			fmt.Fprintf(&text, "第 %d 章：%s\n", i+1, EscapeCommandContextText(section.Title))
			for _, slide := range section.Slides {
				page++
				fmt.Fprintf(&text, "  第 %d 页：%s\n", page, EscapeCommandContextText(slide.Title))
			}
			for j, subsection := range section.Subsections {
				fmt.Fprintf(&text, "  第 %d 节：%s\n", j+1, EscapeCommandContextText(subsection.Title))
				for _, slide := range subsection.Slides {
					page++
					fmt.Fprintf(&text, "    第 %d 页：%s\n", page, EscapeCommandContextText(slide.Title))
				}
			}
		}
	} else {
		field("状态", snapshot.OutlineState)
	}
	text.WriteString("</outline>\n<slide_summaries>\n")
	for _, slide := range snapshot.Slides {
		fmt.Fprintf(&text, "第 %d 页《%s》\n", slide.Ordinal, EscapeCommandContextText(slide.Title))
		field("核心信息", slide.Core)
		field("页面用途", slide.Purpose)
		field("设计稿", slide.SpecState)
		field("HTML", slide.HTMLState)
	}
	text.WriteString("</slide_summaries>\n</project_context>")
	return text.String()
}
