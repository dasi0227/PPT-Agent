package model

import "fmt"

// slide 物理路径集中拼接（页身份重构：目录名用稳定 slide_id，磁盘零迁移）。

// SlideDir 返回某页目录相对路径（相对 project workDir）。
func SlideDir(slideID string) string { return "slides/" + slideID }

// SlideJSONPath 返回某页 slide.json 相对路径。
func SlideJSONPath(slideID string) string { return SlideDir(slideID) + "/slide.json" }

// SlideHTMLPath 返回某页 index.html 相对路径。
func SlideHTMLPath(slideID string) string { return SlideDir(slideID) + "/index.html" }

// SlideVersionSnapshot 返回某页某版本 html 快照相对路径。
func SlideVersionSnapshot(slideID string, versionNo int) string {
	return fmt.Sprintf("versions/slide-%s/v%d.html", slideID, versionNo)
}
