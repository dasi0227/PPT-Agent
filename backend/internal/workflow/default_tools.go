package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/contextengine"

// DefaultDomainToolProvider is the only model-visible PPT business surface.
// It intentionally registers no aliases for the removed artifact-oriented API.
type DefaultDomainToolProvider struct {
	Pack     contextengine.ContextPack
	Renderer SlideRenderer
}

func (p DefaultDomainToolProvider) RegisterDomainTools(registry *ToolRegistry) error {
	renderer := p.Renderer
	if renderer == nil {
		renderer = NewNodeSlideRenderer(NodeRendererConfig{})
	}
	tools := []struct {
		tool       DomainTool
		readOnly   bool
		capability string
		risk       RiskLevel
		phases     []RunPhase
	}{
		{pptReadTool{pack: p.Pack}, true, "ppt.read", RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
		{pptWriteTool{pack: p.Pack}, false, "ppt.write", RiskMedium, []RunPhase{PhaseExecuting}},
		{pptEditTool{pack: p.Pack}, false, "ppt.edit", RiskMedium, []RunPhase{PhaseExecuting}},
		{referenceSearchTool{pack: p.Pack}, true, "context.search", RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
		{slideRenderTool{pack: p.Pack, renderer: renderer}, true, "ppt.render", RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
	}
	for _, item := range tools {
		if err := registry.Register(item.tool, item.readOnly, item.capability, item.risk, item.phases...); err != nil {
			return err
		}
	}
	return nil
}
