// Package slidejson 定义 slide-json 结构（大纲每页的内容与意图）及其 schema 校验。
// schema 权威源为 docs/30-data-model/slide-json.schema.json，本包内嵌一份并有同步守卫测试。
package slidejson

// SlideJSON 是单页 slide 的结构化描述（区别于 M3 的 html 产物）。
// 字段与 slide-json.schema.json 对齐；id/idx 由服务端权威回填。
type SlideJSON struct {
	ID            string       `json:"id"`
	Idx           int          `json:"idx"`
	Layout        string       `json:"layout"`
	Title         string       `json:"title"`
	Subtitle      string       `json:"subtitle,omitempty"`
	Bullets       []string     `json:"bullets,omitempty"`
	ContentIntent string       `json:"content_intent,omitempty"`
	ChartIntent   *ChartIntent `json:"chart_intent,omitempty"`
	Notes         string       `json:"notes,omitempty"`
	Steps         int          `json:"steps,omitempty"`
}

// ChartIntent 是数据型页的图表意图。
type ChartIntent struct {
	Type     string `json:"type"`
	DataHint string `json:"data_hint,omitempty"`
}
