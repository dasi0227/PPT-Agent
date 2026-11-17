package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

var ErrResourceApprovalConflict = errors.New("resource approval changed or expired")

func resourceApprovalID(runID, callID string) string {
	sum := sha256.Sum256([]byte(runID + "\x00" + callID))
	return "resa_" + hex.EncodeToString(sum[:16])
}

func PrepareResourceEditApproval(runID, callID, resource string, tx *RunSession) (model.ResourceEditApproval, error) {
	id := resourceApprovalID(runID, callID)
	path := "." + resource + ".json"
	base, err := tx.ReadBaseline(ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path})
	baseExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return model.ResourceEditApproval{}, err
	}
	proposal, err := tx.ReadPath(path)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	targets := operationDiffTargets(tx)
	var target model.PublicTarget
	for _, value := range targets {
		if value.Type == "deck" && value.Part == resource {
			target = value
			break
		}
	}
	if target.Type == "" {
		return model.ResourceEditApproval{}, errors.New("resource approval has no changed target")
	}
	return model.ResourceEditApproval{RunID: runID, CallID: callID, InteractionID: id, Resource: resource,
		Revision: 1, ResourceEditCandidate: model.ResourceEditCandidate{BaseExists: baseExists, BaseHash: hashBytes(base),
			Base: json.RawMessage(base), Proposal: json.RawMessage(proposal), Draft: json.RawMessage(proposal)},
		Target: target, State: "pending"}, nil
}

func resourceApprovalBaselineMatches(projectDir string, record model.ResourceEditApproval) bool {
	raw, err := os.ReadFile(filepath.Join(projectDir, "."+record.Resource+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return !record.BaseExists
	}
	return err == nil && record.BaseExists && hashBytes(raw) == record.BaseHash
}

func ResourceEditApprovalBaselineMatches(projectDir string, record model.ResourceEditApproval) bool {
	return resourceApprovalBaselineMatches(projectDir, record)
}

func PrepareResourceEditDraft(projectDir string, record model.ResourceEditApproval, revision int64, draft json.RawMessage) (model.ResourceEditApproval, error) {
	if record.State != "pending" || record.Revision != revision || !resourceApprovalBaselineMatches(projectDir, record) {
		return model.ResourceEditApproval{}, ErrResourceApprovalConflict
	}
	sandbox, err := artifactfs.NewSandbox(projectDir)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	buffer := pptmutation.NewBuffer(sandbox)
	engine := pptmutation.Service{Workspace: buffer}
	if _, err := engine.ReplaceApprovedResource(record.Resource, draft); err != nil {
		return model.ResourceEditApproval{}, err
	}
	path := "." + record.Resource + ".json"
	normalized, err := buffer.Read(path)
	if err != nil {
		return model.ResourceEditApproval{}, err
	}
	if bytes.Equal(normalized, record.Draft) {
		return record, nil
	}
	record.Draft = append(json.RawMessage(nil), normalized...)
	record.Revision++
	record.Target = resourceApprovalDiff(projectDir, record)
	return record, nil
}

func resourceApprovalDiff(projectDir string, record model.ResourceEditApproval) model.PublicTarget {
	path := "." + record.Resource + ".json"
	source := func(raw []byte) reviewSourceFile {
		return reviewSourceFile{Content: string(raw), Hash: hashBytes(raw), Size: len(raw)}
	}
	before, after := map[string]reviewSourceFile{}, map[string]reviewSourceFile{}
	if record.BaseExists {
		before[path] = source(record.Base)
	}
	if len(record.Draft) > 0 {
		after[path] = source(record.Draft)
	}
	for _, target := range sourceDiffTargets(projectDir, before, after) {
		if target.Part == record.Resource {
			return target
		}
	}
	return model.PublicTarget{Type: "deck", Part: record.Resource, Diff: &model.ArtifactDiff{Kind: "fields", Status: "modified", Filename: path}}
}

func refusedResourceEditResult() ToolResult {
	return RefusedToolResult("用户拒绝了本次资源编辑，未保存任何修改。")
}

func stageApprovedResource(projectDir string, pack contextengine.ContextPack, tx *RunSession, record model.ResourceEditApproval) (ToolResult, error) {
	tx.RollbackOperation()
	buffer := pptmutation.NewBuffer(runWorkspace{session: tx, pack: pack, source: "edit_" + record.Resource})
	engine := pptmutation.Service{Workspace: buffer}
	changed, err := engine.ReplaceApprovedResource(record.Resource, record.Draft)
	if err != nil {
		return ToolResult{}, err
	}
	if err := buffer.Commit(); err != nil {
		return ToolResult{}, err
	}
	resource := Resource{Type: "deck", Part: record.Resource}
	ref, _ := refForResource(pack, resource)
	raw, _, err := readArtifact(projectDir, tx, ref)
	if err != nil {
		return ToolResult{}, err
	}
	out := SuccessfulToolResult("resource saved")
	if record.Resource == "outline" {
		out.Data = map[string]any{"content": string(raw)}
	} else {
		tx.recordUnchangedResource(resource, ref, raw)
		var content map[string]any
		if err := json.Unmarshal(raw, &content); err != nil {
			return ToolResult{}, err
		}
		out.Data = map[string]any{"content": content}
	}
	if changed {
		out.ChangedTargets = []ChangedTarget{{Type: "deck", Part: record.Resource, Hash: hashBytes(raw)}}
	}
	out.Evidence = []Evidence{newEvidence("schema", resource, hashBytes(raw), map[string]any{"valid": true})}
	setResourceObservation(&out, resource, raw)
	return out, nil
}
