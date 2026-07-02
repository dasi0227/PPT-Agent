// Package seed 内嵌出厂预置资产源（themes/layouts/components/fx）与公共层基座（common/）。
// go:embed 只能引用本目录树下的文件，故内嵌置于此包；asset 包经 FS() 消费（DS-SEED-001）。
package seed

import (
	"embed"
	"io/fs"
)

//go:embed all:assets all:common
var files embed.FS

// FS 返回内嵌 seed 文件系统（根下含 assets/ 与 common/）。
func FS() fs.FS { return files }
