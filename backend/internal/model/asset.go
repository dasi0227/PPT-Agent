package model

// Asset 是个人仓库统一资产的元数据（4 类共享信封；载荷存文件系统，此处仅索引）。
// 对应 assets 表（DATA-MODEL / ADR-0009）。载荷文件通过 Dir/ManifestPath 定位。
type Asset struct {
	ID           string
	Name         string
	Kind         string // layout|component|theme|fx
	Version      string
	Source       string // preset|user
	Description  string
	Tags         []string
	ManifestPath string // _assets/<kind_dir>/<id>/manifest.json（相对 work_root）
	Dir          string // _assets/<kind_dir>/<id>（相对 work_root）
	CreatedAt    int64
	UpdatedAt    int64
}
