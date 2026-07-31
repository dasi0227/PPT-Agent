package contextengine

import (
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

type DOMBlock struct {
	Tag     string   `json:"tag"`
	Anchors []string `json:"anchors"`
	Text    string   `json:"text,omitempty"`
}

type HTMLSummary struct {
	Title          string     `json:"title"`
	Structure      []DOMBlock `json:"structure"`
	TextDigest     []string   `json:"text_digest"`
	AssetRefs      []string   `json:"asset_refs"`
	TokenRefs      []string   `json:"token_refs"`
	ScriptFeatures []string   `json:"script_features"`
	Warnings       []string   `json:"warnings"`
	SourceHash     string     `json:"source_hash"`
}

func SummarizeHTML(raw []byte) (HTMLSummary, error) {
	sum := HTMLSummary{Structure: []DOMBlock{}, TextDigest: []string{}, AssetRefs: []string{}, TokenRefs: []string{}, ScriptFeatures: []string{}, Warnings: []string{}}
	hash := sha256.Sum256(raw)
	sum.SourceHash = fmt.Sprintf("%x", hash[:])
	doc, err := html.Parse(strings.NewReader(string(raw)))
	if err != nil {
		return sum, err
	}
	assets, tokens, features := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			anchors := []string{}
			for _, a := range n.Attr {
				key, val := strings.ToLower(a.Key), strings.TrimSpace(a.Val)
				if key == "id" || key == "class" || strings.HasPrefix(key, "data-") {
					anchors = append(anchors, key+"="+val)
				}
				if (tag == "img" || tag == "script" || tag == "link" || tag == "source") && (key == "src" || key == "href") && val != "" {
					assets[val] = true
				}
				if key == "class" {
					for _, class := range strings.Fields(val) {
						if strings.Contains(class, "token") || strings.HasPrefix(class, "var-") {
							tokens[class] = true
						}
					}
				}
			}
			if tag == "title" && n.FirstChild != nil {
				sum.Title = strings.TrimSpace(n.FirstChild.Data)
			}
			if tag == "script" || tag == "style" {
				size := textSize(n)
				features[fmt.Sprintf("inline_%s_bytes:%d", tag, size)] = true
			}
			if semanticTag(tag) {
				text := compactText(nodeText(n), 240)
				sort.Strings(anchors)
				sum.Structure = append(sum.Structure, DOMBlock{Tag: tag, Anchors: anchors, Text: text})
			}
			if tag == "h1" || tag == "h2" || tag == "h3" || tag == "p" || tag == "li" {
				if text := compactText(nodeText(n), 180); text != "" && len(sum.TextDigest) < 32 {
					sum.TextDigest = append(sum.TextDigest, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	sum.AssetRefs = sortedKeys(assets)
	sum.TokenRefs = sortedKeys(tokens)
	sum.ScriptFeatures = sortedKeys(features)
	return sum, nil
}

func semanticTag(tag string) bool {
	switch tag {
	case "main", "section", "article", "header", "footer", "nav", "aside", "figure", "svg", "canvas":
		return true
	}
	return false
}

func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			_, _ = io.WriteString(&b, x.Data+" ")
		}
		if x != n && x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style") {
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func textSize(n *html.Node) int { return len(nodeText(n)) }

func compactText(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > limit {
		return s[:limit] + "…"
	}
	return s
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
