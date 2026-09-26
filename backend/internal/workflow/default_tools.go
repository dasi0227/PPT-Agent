package workflow

import "github.com/dasi0227/PPT-Agent/backend/internal/contextengine"

// DefaultDomainToolProvider is the only model-visible PPT business surface.
// It intentionally registers no aliases for the removed artifact-oriented API.
type DefaultDomainToolProvider struct {
	Pack       contextengine.ContextPack
	Renderer   SlideRenderer
	Components ComponentLoader
	Skills     SkillLoader
	Themes     ThemeLoader
}

func (p DefaultDomainToolProvider) RegisterDomainTools(registry *ToolRegistry) error {
	renderer := p.Renderer
	if renderer == nil {
		renderer = NewNodeSlideRenderer(NodeRendererConfig{})
	}
	tools := []struct {
		tool       DomainTool
		readOnly   bool
		capability ToolCapability
		risk       RiskLevel
		phases     []RunPhase
	}{
		{pptReadTool{pack: p.Pack}, true, CapabilityPPTRead, RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
		{slideRenderTool{pack: p.Pack, renderer: renderer, themes: p.Themes}, true, CapabilityPPTRender, RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
		{readImageTool{}, true, CapabilityImageRead, RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}},
	}
	if p.Components != nil {
		tools = append(tools, struct {
			tool       DomainTool
			readOnly   bool
			capability ToolCapability
			risk       RiskLevel
			phases     []RunPhase
		}{loadComponentTool{loader: p.Components}, true, CapabilityRead, RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}})
	}
	if p.Skills != nil {
		tools = append(tools, struct {
			tool       DomainTool
			readOnly   bool
			capability ToolCapability
			risk       RiskLevel
			phases     []RunPhase
		}{loadSkillTool{loader: p.Skills}, true, CapabilityRead, RiskLow, []RunPhase{PhaseChat, PhasePlanning, PhaseExecuting}})
	}
	for _, item := range tools {
		if err := registry.Register(item.tool, item.readOnly, item.capability, item.risk, item.phases...); err != nil {
			return err
		}
	}
	for _, name := range []string{"edit_manifest", "edit_design", "edit_spec", "write_html", "patch_html", "init_outline", "arrange_outline"} {
		if err := registry.Register(resourceEditTool{pack: p.Pack, name: name}, false, CapabilityPPTMutate, RiskMedium, PhaseExecuting); err != nil {
			return err
		}
	}
	return registry.RegisterDynamic(
		projectCommandTool{},
		CapabilityProjectCommandRead,
		CapabilityProjectCommandEdit,
		PhaseChat,
		PhasePlanning,
		PhaseExecuting,
	)
}
