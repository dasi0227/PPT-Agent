package tools

// Registry 持有一组工具，按名索引。harness 门控从中筛选可用子集。
type Registry struct {
	byName map[string]Tool
	order  []string
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]Tool{}}
}

// Register 注册一个工具；重名会 panic（编程错误，编译期/启动期暴露）。
func (r *Registry) Register(t Tool) {
	if _, ok := r.byName[t.Name()]; ok {
		panic("tools: duplicate tool " + t.Name())
	}
	r.byName[t.Name()] = t
	r.order = append(r.order, t.Name())
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

// All 按注册顺序返回全部工具。
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.byName[name])
	}
	return out
}
