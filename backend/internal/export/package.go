package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/workflow"
	xhtml "golang.org/x/net/html"
)

var cssURLPattern = regexp.MustCompile(`(?i)(?:url\(\s*['\"]?|@import\s+(?:url\(\s*)?['\"]?)(https?:)?//([^'\")\s;]+)`)
var cssAnyURLPattern = regexp.MustCompile(`(?i)url\(\s*['\"]?([^'\")]+)`)
var cssImportPattern = regexp.MustCompile(`(?i)@import\s+['\"]([^'\"]+)`)

func buildArtifact(ctx context.Context, op *Operation, renderer workflow.SlideRenderer) (*Artifact, []string, *PublicError) {
	switch op.Format {
	case FormatHTML:
		return buildHTML(ctx, op)
	case FormatPNG, FormatPDF:
		return buildRaster(ctx, op, renderer)
	default:
		return nil, nil, publicFailure("EXPORT_FORMAT_INVALID", "不支持这种导出格式。", false, nil)
	}
}

func buildRaster(ctx context.Context, op *Operation, renderer workflow.SlideRenderer) (*Artifact, []string, *PublicError) {
	if renderer == nil {
		return nil, nil, publicFailure("EXPORT_RENDER_FAILED", "渲染服务不可用，请稍后重试。", true, nil)
	}
	work := filepath.Join(filepath.Dir(op.Snapshot.Root), "work")
	if err := os.MkdirAll(work, 0o700); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法准备导出文件。", true, err)
	}
	paths := make([]string, 0, len(op.Snapshot.Slides))
	for _, slide := range op.Snapshot.Slides {
		if ctx.Err() != nil {
			return nil, nil, publicFailure("EXPORT_CANCELED", "导出已取消。", false, ctx.Err())
		}
		if resources, scanErr := scanResources(slide.HTML); scanErr != nil {
			return nil, nil, resourceFailure(slide, scanErr)
		} else if len(resources) > 0 {
			return nil, nil, &PublicError{Code: "EXPORT_EXTERNAL_RESOURCE", Message: "图片或 PDF 导出不能使用外部网络资源。", Details: map[string]any{"slides": []MissingSlide{{SlideID: slide.ID, Ordinal: slide.Ordinal, Title: slide.Title}}}, Retryable: false}
		}
		path := filepath.Join(work, slide.FileName)
		diagnostics, err := renderer.Render(ctx, workflow.RenderRequest{RunID: op.ID, ProjectDir: op.Snapshot.Root, SlideID: slide.ID, HTML: string(slide.HTML), ScreenshotPath: path, ViewportWidth: spec.CanvasWidth, ViewportHeight: spec.CanvasHeight, TimeoutMS: 15000, Frame: slide.Frame, BaseCSS: string(op.Snapshot.BaseCSS), ThemeID: op.Snapshot.ThemeID, ThemeCSS: string(op.Snapshot.ThemeCSS)})
		if err != nil {
			return nil, nil, &PublicError{Code: "EXPORT_RENDER_FAILED", Message: fmt.Sprintf("第 %d 页渲染失败。", slide.Ordinal), Details: map[string]any{"slides": []MissingSlide{{SlideID: slide.ID, Ordinal: slide.Ordinal, Title: slide.Title}}}, Retryable: true}
		}
		if diagnostics.ScreenshotBytes <= 0 || len(diagnostics.FailedResources) > 0 || len(diagnostics.ConsoleErrors) > 0 {
			return nil, nil, &PublicError{Code: "EXPORT_RENDER_FAILED", Message: fmt.Sprintf("第 %d 页存在无法完成的渲染错误。", slide.Ordinal), Details: map[string]any{"slides": []MissingSlide{{SlideID: slide.ID, Ordinal: slide.Ordinal, Title: slide.Title}}}, Retryable: false}
		}
		paths = append(paths, path)
		op.progress(PhaseRendering, slide.ID, slide.Ordinal, slide.Ordinal)
	}
	op.progress(PhasePackaging, "", 0, len(paths))
	artifactDir := filepath.Join(filepath.Dir(op.Snapshot.Root), "artifact")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法生成导出文件。", true, err)
	}
	stamp := time.Now().Format("20060102-1504")
	base := SafeName(op.Snapshot.ProjectTitle) + "-" + stamp
	if op.Format == FormatPNG {
		name := base + "-png.zip"
		path := filepath.Join(artifactDir, name)
		if err := zipNamedFiles(path, paths); err != nil {
			return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法打包 PNG 文件。", true, err)
		}
		result, err := artifact(path, name, "application/zip")
		if err != nil {
			return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法读取 PNG 导出文件。", true, err)
		}
		return result, nil, nil
	}
	name := base + ".pdf"
	path := filepath.Join(artifactDir, name)
	assembler, ok := renderer.(interface {
		AssemblePDF(context.Context, []string, string) (int, error)
	})
	if !ok {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "PDF 组装服务不可用。", true, nil)
	}
	pageCount, err := assembler.AssemblePDF(ctx, paths, path)
	if err != nil || pageCount != len(paths) {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法生成 PDF 文件。", true, err)
	}
	result, err := artifact(path, name, "application/pdf")
	if err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法读取 PDF 导出文件。", true, err)
	}
	return result, nil, nil
}

func buildHTML(ctx context.Context, op *Operation) (*Artifact, []string, *PublicError) {
	work := filepath.Join(filepath.Dir(op.Snapshot.Root), "work", "html")
	for _, dir := range []string{"runtime", "assets", "slides", "attachments"} {
		if err := os.MkdirAll(filepath.Join(work, dir), 0o700); err != nil {
			return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法准备 HTML 演示包。", true, err)
		}
	}
	baseCSS := append(append([]byte(nil), op.Snapshot.BaseCSS...), []byte(exportChromeCSS)...)
	if err := writeFile(filepath.Join(work, "assets", "base.css"), baseCSS); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入基础样式。", true, err)
	}
	if err := writeFile(filepath.Join(work, "assets", "theme.css"), op.Snapshot.ThemeCSS); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入主题样式。", true, err)
	}
	warnings := []string{}
	attachmentData, err := htmlAttachmentData(op.Snapshot)
	if err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法读取图片素材。", true, err)
	}
	slideFiles := make([]string, 0, len(op.Snapshot.Slides))
	for _, slide := range op.Snapshot.Slides {
		if ctx.Err() != nil {
			return nil, nil, publicFailure("EXPORT_CANCELED", "导出已取消。", false, ctx.Err())
		}
		rewritten, external, err := rewriteSlideHTML(slide.HTML, slide.Frame, op.Snapshot.BaseCSS, op.Snapshot.ThemeCSS, attachmentData)
		if err != nil {
			return nil, nil, resourceFailure(slide, err)
		}
		name := fmt.Sprintf("%03d.html", slide.Ordinal)
		if err := writeFile(filepath.Join(work, "slides", name), rewritten); err != nil {
			return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入页面文件。", true, err)
		}
		slideFiles = append(slideFiles, "slides/"+name)
		for _, resource := range external {
			warnings = append(warnings, fmt.Sprintf("第 %d 页包含外部资源 %s，播放时需要联网。", slide.Ordinal, resource))
		}
		op.progress(PhasePackaging, slide.ID, slide.Ordinal, slide.Ordinal)
	}
	for _, rel := range op.Snapshot.Attachments {
		if ctx.Err() != nil {
			return nil, nil, publicFailure("EXPORT_CANCELED", "导出已取消。", false, ctx.Err())
		}
		if err := copyFile(filepath.Join(op.Snapshot.Root, filepath.FromSlash(rel)), filepath.Join(work, filepath.FromSlash(rel))); err != nil {
			return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法复制图片素材。", true, err)
		}
	}
	if err := writeFile(filepath.Join(work, "runtime", "player.css"), []byte(playerCSS)); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入播放器。", true, err)
	}
	if err := writeFile(filepath.Join(work, "runtime", "player.js"), []byte(playerJS)); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入播放器。", true, err)
	}
	list, _ := json.Marshal(slideFiles)
	index := fmt.Sprintf(indexHTML, html.EscapeString(op.Snapshot.ProjectTitle), string(list))
	if err := writeFile(filepath.Join(work, "index.html"), []byte(index)); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法写入播放器入口。", true, err)
	}
	artifactDir := filepath.Join(filepath.Dir(op.Snapshot.Root), "artifact")
	if err := os.MkdirAll(artifactDir, 0o700); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法生成导出文件。", true, err)
	}
	name := SafeName(op.Snapshot.ProjectTitle) + "-" + time.Now().Format("20060102-1504") + "-html.zip"
	path := filepath.Join(artifactDir, name)
	if err := zipDirectory(path, work); err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法打包 HTML 演示文件。", true, err)
	}
	sort.Strings(warnings)
	warnings = uniqueStrings(warnings)
	result, err := artifact(path, name, "application/zip")
	if err != nil {
		return nil, nil, publicFailure("EXPORT_PACKAGE_FAILED", "无法读取 HTML 导出文件。", true, err)
	}
	return result, warnings, nil
}

func artifact(path, name, mime string) (*Artifact, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &Artifact{Path: path, Name: name, MIME: mime, Size: info.Size(), Download: ""}, nil
}
func resourceFailure(slide SlideSnapshot, err error) *PublicError {
	return &PublicError{Code: "EXPORT_RESOURCE_INVALID", Message: fmt.Sprintf("第 %d 页包含无法导出的本地资源。", slide.Ordinal), Details: map[string]any{"slides": []MissingSlide{{SlideID: slide.ID, Ordinal: slide.Ordinal, Title: slide.Title}}}, Retryable: false}
}

func rewriteSlideHTML(raw []byte, frame spec.RuntimeFrameContext, baseCSS, themeCSS []byte, attachmentData map[string]string) ([]byte, []string, error) {
	doc, err := xhtml.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	head := findNode(doc, "head")
	body := findNode(doc, "body")
	if head == nil || body == nil {
		return nil, nil, fmt.Errorf("missing document structure")
	}
	external := []string{}
	var walk func(*xhtml.Node) error
	walk = func(node *xhtml.Node) error {
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			if child.Type == xhtml.ElementNode {
				if child.Data == "script" && strings.Contains(attrValue(child, "src"), "selection-bridge") {
					node.RemoveChild(child)
					child = next
					continue
				}
				if child.Data == "link" && isRuntimeStyle(child) {
					node.RemoveChild(child)
					child = next
					continue
				}
				if child.Data == "style" && child.FirstChild != nil {
					external = append(external, cssResources(child.FirstChild.Data)...)
					if err := validateCSSResources(child.FirstChild.Data, attachmentData); err != nil {
						return err
					}
					child.FirstChild.Data = rewriteCSSAttachments(child.FirstChild.Data, attachmentData)
				}
				for index := range child.Attr {
					a := &child.Attr[index]
					if a.Key == "style" {
						external = append(external, cssResources(a.Val)...)
						if err := validateCSSResources(a.Val, attachmentData); err != nil {
							return err
						}
						a.Val = rewriteCSSAttachments(a.Val, attachmentData)
						continue
					}
					if !resourceAttribute(child.Data, a.Key) {
						continue
					}
					values := []string{a.Val}
					if a.Key == "srcset" {
						values = parseSrcset(a.Val)
					}
					for _, value := range values {
						if safe := externalDisplay(value); safe != "" {
							external = append(external, safe)
							continue
						}
						if rewritten, ok := rewriteAttachment(value, attachmentData); ok {
							if a.Key != "srcset" {
								a.Val = rewritten
							}
							continue
						}
						if !allowedEmbeddedResource(value) {
							return fmt.Errorf("unsupported local resource")
						}
					}
					if a.Key == "srcset" {
						a.Val = rewriteSrcset(a.Val, attachmentData)
					}
				}
			}
			if err := walk(child); err != nil {
				return err
			}
			child = next
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, nil, err
	}
	head.AppendChild(linkNode("base-link", "../assets/base.css"))
	head.AppendChild(linkNode("theme-link", "../assets/theme.css"))
	head.AppendChild(styleNode("export-base-inline", string(baseCSS)))
	head.AppendChild(styleNode("export-theme-inline", string(themeCSS)))
	head.InsertBefore(attachmentPrelude(attachmentData), head.FirstChild)
	injectChrome(body, frame)
	var output bytes.Buffer
	if err := xhtml.Render(&output, doc); err != nil {
		return nil, nil, err
	}
	return output.Bytes(), uniqueStrings(external), nil
}

func scanResources(raw []byte) ([]string, error) {
	doc, err := xhtml.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	resources := []string{}
	var walk func(*xhtml.Node) error
	walk = func(n *xhtml.Node) error {
		if n.Type == xhtml.ElementNode {
			if n.Data == "style" && n.FirstChild != nil {
				resources = append(resources, cssResources(n.FirstChild.Data)...)
			}
			for _, a := range n.Attr {
				if a.Key == "style" {
					resources = append(resources, cssResources(a.Val)...)
				} else if resourceAttribute(n.Data, a.Key) {
					values := []string{a.Val}
					if a.Key == "srcset" {
						values = parseSrcset(a.Val)
					}
					for _, v := range values {
						if display := externalDisplay(v); display != "" {
							resources = append(resources, display)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(doc); err != nil {
		return nil, err
	}
	return uniqueStrings(resources), nil
}

func resourceAttribute(tag, key string) bool {
	switch tag {
	case "script":
		return key == "src"
	case "link":
		return key == "href"
	case "img", "source":
		return key == "src" || key == "srcset"
	case "video":
		return key == "src" || key == "poster"
	case "iframe", "embed":
		return key == "src"
	case "object":
		return key == "data"
	}
	return false
}
func externalDisplay(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "//") {
		value = "https:" + value
	}
	u, err := url.Parse(value)
	if err != nil || !(u.Scheme == "http" || u.Scheme == "https") {
		return ""
	}
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}
func allowedEmbeddedResource(value string) bool {
	v := strings.TrimSpace(value)
	return v == "" || strings.HasPrefix(v, "data:") || strings.HasPrefix(v, "blob:") || strings.HasPrefix(v, "#")
}
func rewriteAttachment(value string, attachmentData map[string]string) (string, bool) {
	v := strings.TrimSpace(value)
	if data := attachmentData[v]; data != "" {
		return data, true
	}
	return value, false
}
func parseSrcset(value string) []string {
	out := []string{}
	for _, part := range strings.Split(value, ",") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) > 0 {
			out = append(out, fields[0])
		}
	}
	return out
}
func rewriteSrcset(value string, attachmentData map[string]string) string {
	parts := strings.Split(value, ",")
	for i, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) > 0 {
			if next, ok := rewriteAttachment(fields[0], attachmentData); ok {
				fields[0] = next
			}
			parts[i] = strings.Join(fields, " ")
		}
	}
	return strings.Join(parts, ", ")
}
func cssResources(value string) []string {
	matches := cssURLPattern.FindAllStringSubmatch(value, -1)
	out := []string{}
	for _, m := range matches {
		scheme := m[1]
		if scheme == "" {
			scheme = "https:"
		}
		if len(m) > 2 {
			if display := externalDisplay(scheme + "//" + m[2]); display != "" {
				out = append(out, display)
			}
		}
	}
	return out
}
func rewriteCSSAttachments(value string, attachmentData map[string]string) string {
	for path, data := range attachmentData {
		if strings.HasPrefix(path, "/attachments/") {
			value = strings.ReplaceAll(value, path, data)
		}
	}
	value = strings.ReplaceAll(value, "url(/attachments/", "url(../attachments/")
	value = strings.ReplaceAll(value, "url('/attachments/", "url('../attachments/")
	return strings.ReplaceAll(value, `url("/attachments/`, `url("../attachments/`)
}
func validateCSSResources(value string, attachmentData map[string]string) error {
	matches := append(cssAnyURLPattern.FindAllStringSubmatch(value, -1), cssImportPattern.FindAllStringSubmatch(value, -1)...)
	for _, match := range matches {
		resource := strings.TrimSpace(match[1])
		if externalDisplay(resource) != "" || allowedEmbeddedResource(resource) {
			continue
		}
		if _, ok := rewriteAttachment(resource, attachmentData); ok {
			continue
		}
		return fmt.Errorf("unsupported local CSS resource")
	}
	return nil
}
func styleNode(id, css string) *xhtml.Node {
	node := &xhtml.Node{Type: xhtml.ElementNode, Data: "style", Attr: []xhtml.Attribute{{Key: "id", Val: id}}}
	node.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: css})
	return node
}

func htmlAttachmentData(snapshot Snapshot) (map[string]string, error) {
	out := map[string]string{}
	for _, rel := range snapshot.Attachments {
		raw, err := os.ReadFile(filepath.Join(snapshot.Root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		mime := "application/octet-stream"
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".png":
			mime = "image/png"
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".webp":
			mime = "image/webp"
		}
		data := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
		canonical := "/" + filepath.ToSlash(rel)
		out[canonical] = data
		out[strings.TrimPrefix(canonical, "/")] = data
		out["../"+strings.TrimPrefix(canonical, "/")] = data
	}
	return out, nil
}

func attachmentPrelude(attachments map[string]string) *xhtml.Node {
	canonical := map[string]string{}
	for path, data := range attachments {
		if strings.HasPrefix(path, "/attachments/") {
			canonical[path] = data
		}
	}
	raw, _ := json.Marshal(canonical)
	safe := strings.ReplaceAll(string(raw), "<", `\u003c`)
	script := `(()=>{const files=` + safe + `;const remap=value=>typeof value==='string'?(files[value]||files['/'+value.replace(/^\.\.\//,'')]||value):value;const set=Element.prototype.setAttribute;Element.prototype.setAttribute=function(name,value){return set.call(this,name,remap(value))};for(const [type,key] of [[HTMLImageElement,'src'],[HTMLSourceElement,'src'],[HTMLVideoElement,'src'],[HTMLVideoElement,'poster']]){const descriptor=Object.getOwnPropertyDescriptor(type.prototype,key);if(descriptor?.set)Object.defineProperty(type.prototype,key,{...descriptor,set(value){descriptor.set.call(this,remap(value))}})}const nativeFetch=window.fetch;window.fetch=(input,init)=>nativeFetch(remap(typeof input==='string'?input:input?.url)||input,init);window.__PPT_ATTACHMENT_URL__=remap})();`
	node := &xhtml.Node{Type: xhtml.ElementNode, Data: "script", Attr: []xhtml.Attribute{{Key: "data-export-runtime", Val: "attachments"}}}
	node.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: script})
	return node
}
func isRuntimeStyle(n *xhtml.Node) bool {
	id, href := attrValue(n, "id"), attrValue(n, "href")
	return id == "base-link" || id == "theme-link" || strings.Contains(href, "/api/v1/runtime/base.css") || strings.Contains(href, "/api/v1/themes/") || strings.HasSuffix(href, "/common/base.css") || strings.HasSuffix(href, "/common/tokens.css")
}
func attrValue(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func findNode(n *xhtml.Node, name string) *xhtml.Node {
	if n.Type == xhtml.ElementNode && n.Data == name {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findNode(c, name); found != nil {
			return found
		}
	}
	return nil
}
func linkNode(id, href string) *xhtml.Node {
	return &xhtml.Node{Type: xhtml.ElementNode, Data: "link", Attr: []xhtml.Attribute{{Key: "id", Val: id}, {Key: "rel", Val: "stylesheet"}, {Key: "href", Val: href}}}
}
func injectChrome(body *xhtml.Node, frame spec.RuntimeFrameContext) {
	stage := findClassNode(body, "slide-stage")
	for _, item := range frame.Chrome {
		text := ""
		switch item.Type {
		case "page_number":
			if frame.Numbering.Visible {
				text = strconv.Itoa(frame.Ordinal)
			}
		case "section_marker":
			text = frame.Section.Title
		case "deck_title":
			text = frame.DeckTitle
		}
		if text == "" {
			continue
		}
		node := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", Attr: []xhtml.Attribute{{Key: "data-runtime-chrome", Val: item.Type}, {Key: "data-chrome-placement", Val: item.Placement}, {Key: "data-chrome-style", Val: item.Style}}, FirstChild: &xhtml.Node{Type: xhtml.TextNode, Data: text}}
		node.FirstChild.Parent = node
		if stage != nil {
			stage.AppendChild(node)
		} else {
			body.AppendChild(node)
		}
	}
}
func findClassNode(n *xhtml.Node, className string) *xhtml.Node {
	if n.Type == xhtml.ElementNode {
		for _, name := range strings.Fields(attrValue(n, "class")) {
			if name == className {
				return n
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findClassNode(c, className); found != nil {
			return found
		}
	}
	return nil
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func zipNamedFiles(target string, paths []string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(target)
		}
	}()
	for _, path := range paths {
		entry, err := writer.Create(filepath.Base(path))
		if err != nil {
			return err
		}
		source, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
func zipDirectory(target, root string) error {
	paths := []string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		return err
	}
	sort.Strings(paths)
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(file)
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(target)
		}
	}()
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("invalid zip path")
		}
		entry, err := writer.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		raw, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(entry, raw)
		_ = raw.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

const indexHTML = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><link rel="stylesheet" href="runtime/player.css"></head><body><main id="stage"><iframe id="slide" title="演示页面" sandbox="allow-scripts"></iframe></main><nav aria-label="演示控制"><button id="prev" type="button">上一页</button><span id="counter"></span><button id="next" type="button">下一页</button><button id="fullscreen" type="button">全屏</button></nav><script>window.__PPT_SLIDES__=%s;</script><script src="runtime/player.js"></script></body></html>`
const playerCSS = `html,body{margin:0;width:100%;height:100%;overflow:hidden;background:#111;color:#fff;font-family:system-ui,sans-serif}#stage{position:fixed;inset:0 0 52px;overflow:hidden}#slide{position:absolute;width:1920px;height:1080px;border:0;transform-origin:0 0;background:#fff}nav{position:fixed;left:0;right:0;bottom:0;height:52px;display:flex;align-items:center;justify-content:center;gap:12px;background:#191919}button{border:1px solid #555;border-radius:6px;background:#292929;color:#fff;padding:7px 14px;cursor:pointer}button:disabled{opacity:.4;cursor:default}#counter{min-width:70px;text-align:center;font-variant-numeric:tabular-nums}`
const playerJS = `(()=>{const slides=window.__PPT_SLIDES__||[];const frame=document.getElementById('slide');const counter=document.getElementById('counter');const prev=document.getElementById('prev');const next=document.getElementById('next');let index=0;function resize(){const stage=document.getElementById('stage');const scale=Math.min(stage.clientWidth/1920,stage.clientHeight/1080);frame.style.transform='translate('+((stage.clientWidth-1920*scale)/2)+'px,'+((stage.clientHeight-1080*scale)/2)+'px) scale('+scale+')'}function show(value){index=Math.max(0,Math.min(slides.length-1,value));frame.src=slides[index]||'about:blank';counter.textContent=slides.length?(index+1)+' / '+slides.length:'0 / 0';prev.disabled=index===0;next.disabled=index>=slides.length-1}prev.onclick=()=>show(index-1);next.onclick=()=>show(index+1);document.getElementById('fullscreen').onclick=()=>document.documentElement.requestFullscreen();addEventListener('resize',resize);addEventListener('keydown',event=>{if(event.key==='ArrowLeft')show(index-1);else if(event.key==='ArrowRight'||event.key===' '){event.preventDefault();show(index+1)}else if(event.key==='Home')show(0);else if(event.key==='End')show(slides.length-1)});resize();show(0)})();`
const exportChromeCSS = `
[data-runtime-chrome]{position:absolute!important;z-index:2!important;padding:3px 6px!important;color:rgba(20,25,35,.58)!important;font:500 14px/1.2 ui-monospace,SFMono-Regular,Menlo,monospace!important;letter-spacing:.04em!important;pointer-events:none!important}
[data-runtime-chrome][data-chrome-placement="top-left"]{top:3.2%;left:3.4%}[data-runtime-chrome][data-chrome-placement="top-center"]{top:3.2%;left:50%;transform:translateX(-50%)}[data-runtime-chrome][data-chrome-placement="top-right"]{top:3.2%;right:3.4%}[data-runtime-chrome][data-chrome-placement="bottom-left"]{bottom:3.2%;left:3.4%}[data-runtime-chrome][data-chrome-placement="bottom-center"]{bottom:3.2%;left:50%;transform:translateX(-50%)}[data-runtime-chrome][data-chrome-placement="bottom-right"]{bottom:3.2%;right:3.4%}[data-runtime-chrome][data-chrome-placement="left-edge"]{left:1.5%;top:50%;transform:translateY(-50%)}[data-runtime-chrome][data-chrome-placement="right-edge"]{right:1.5%;top:50%;transform:translateY(-50%)}
[data-runtime-chrome][data-chrome-style~="compact"]{font-size:16px!important}[data-runtime-chrome][data-chrome-style~="label"]{font:700 16px/1.2 ui-sans-serif,system-ui,sans-serif!important;letter-spacing:.08em!important;text-transform:uppercase!important}`
