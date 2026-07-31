package tools

import "github.com/dasi0227/PPT-Agent/backend/internal/model"

type Capability string

const (
	CapabilityReadBlueprint     Capability = "read_blueprint"
	CapabilityWriteBlueprint    Capability = "write_blueprint"
	CapabilityReadPresentation  Capability = "read_presentation"
	CapabilityWritePresentation Capability = "write_presentation"
	CapabilityReadDesignSpec    Capability = "read_design_spec"
	CapabilityWriteDesignSpec   Capability = "write_design_spec"
	CapabilitySearchAssets      Capability = "search_assets"
	CapabilityReadAssets        Capability = "read_assets"
	CapabilityReadContext       Capability = "read_context"
	CapabilityMountAssets       Capability = "mount_assets"
	CapabilityRenderPreview     Capability = "render_preview"
	CapabilityControl           Capability = "control"
)

type CapabilityTool interface {
	Tool
	Capabilities() []Capability
}

func AllowedCapabilities(spec model.WorkSpec) map[Capability]bool {
	allowed := map[Capability]bool{CapabilityControl: true}
	allowed[CapabilityReadContext] = true
	if spec.Target.Artifact == model.ArtifactBlueprint {
		allowed[CapabilityReadBlueprint] = true
		allowed[CapabilityReadDesignSpec] = true
	} else {
		allowed[CapabilityReadBlueprint] = true
		allowed[CapabilityReadPresentation] = true
		allowed[CapabilityReadDesignSpec] = true
		allowed[CapabilitySearchAssets] = true
		allowed[CapabilityReadAssets] = true
		allowed[CapabilityRenderPreview] = true
	}
	if spec.Interaction.Intent == model.IntentConsult {
		return allowed
	}
	if spec.Target.Artifact == model.ArtifactBlueprint {
		allowed[CapabilityWriteBlueprint] = true
	} else {
		allowed[CapabilityWritePresentation] = true
		allowed[CapabilityMountAssets] = true
	}
	return allowed
}

func GateCapabilities(all []Tool, spec model.WorkSpec) []Tool {
	allowed := AllowedCapabilities(spec)
	out := make([]Tool, 0, len(all))
	for _, tool := range all {
		capabilityTool, ok := tool.(CapabilityTool)
		if !ok {
			// Legacy tools remain available only to legacy runners; new capability
			// runners must explicitly declare their authority.
			continue
		}
		ok = true
		for _, capability := range capabilityTool.Capabilities() {
			if !allowed[capability] {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, tool)
		}
	}
	return out
}
