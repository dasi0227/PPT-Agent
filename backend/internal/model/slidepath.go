package model

import "fmt"

// slide 物理路径集中拼接（页身份重构：目录名用稳定 slide_id，磁盘零迁移）。

// SlideDir 返回某页目录相对路径（相对 project workDir）。
func SlideDir(slideID string) string { return "slides/" + slideID }

// SlideSpecPath returns one slide's spec.json path relative to the project.
func SlideSpecPath(slideID string) string { return SlideDir(slideID) + "/spec.json" }

// SlideHTMLPath 返回某页 index.html 相对路径。
func SlideHTMLPath(slideID string) string { return SlideDir(slideID) + "/index.html" }

// SlideMaterializationPath returns Runtime-owned HTML materialization metadata.
func SlideMaterializationPath(slideID string) string {
	return SlideDir(slideID) + "/materialization.json"
}

// SlideHTMLVersionSnapshot 返回某页某版本 html 快照相对路径。
func SlideHTMLVersionSnapshot(slideID string, versionNo int) string {
	return fmt.Sprintf("versions/slide-html-%s/v%d.html", slideID, versionNo)
}

func SlideSpecVersionSnapshot(slideID string, versionNo int) string {
	return fmt.Sprintf("versions/slide-spec-%s/v%d.json", slideID, versionNo)
}

func OutlineVersionSnapshot(versionNo int) string {
	return fmt.Sprintf("versions/outline/v%d.json", versionNo)
}

func DesignVersionSnapshot(versionNo int) string {
	return fmt.Sprintf("versions/design/v%d.json", versionNo)
}
