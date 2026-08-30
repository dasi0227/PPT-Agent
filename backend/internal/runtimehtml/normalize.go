package runtimehtml

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

func Normalize(raw []byte, themeID string) ([]byte, error) {
	document, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	head := findElement(document, "head")
	if head == nil {
		return raw, nil
	}
	removeStylesheetLinks(head)
	appendLink(head, "base-link", "/api/v1/runtime/base.css")
	appendLink(head, "theme-link", "/api/v1/themes/"+themeID+"/css")
	var output bytes.Buffer
	if err := html.Render(&output, document); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func findElement(node *html.Node, name string) *html.Node {
	if node.Type == html.ElementNode && node.Data == name {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findElement(child, name); found != nil {
			return found
		}
	}
	return nil
}

func removeStylesheetLinks(head *html.Node) {
	for child := head.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && child.Data == "link" {
			id, href := attr(child, "id"), attr(child, "href")
			if id == "base-link" || id == "theme-link" ||
				strings.HasSuffix(href, "/common/base.css") || strings.HasSuffix(href, "/common/tokens.css") {
				head.RemoveChild(child)
			}
		}
		child = next
	}
}

func attr(node *html.Node, name string) string {
	for _, value := range node.Attr {
		if value.Key == name {
			return value.Val
		}
	}
	return ""
}

func appendLink(head *html.Node, id, href string) {
	head.AppendChild(&html.Node{
		Type: html.ElementNode, Data: "link",
		Attr: []html.Attribute{{Key: "id", Val: id}, {Key: "rel", Val: "stylesheet"}, {Key: "href", Val: href}},
	})
}
