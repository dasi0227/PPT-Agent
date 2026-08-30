// Package seed embeds the factory Theme and Component repositories and Runtime base CSS.
package seed

import (
	"embed"
	"io/fs"
)

//go:embed all:assets all:common
var files embed.FS

// FS 返回内嵌 seed 文件系统（根下含 assets/ 与 common/）。
func FS() fs.FS { return files }
