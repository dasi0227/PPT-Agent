package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/pptmutation"
)

var ErrResourceApprovalConflict = errors.New("resource approval changed or expired")
var resourceApprovalMu sync.Mutex

type ResourceEditApproval struct {
	RunID         string                            `json:"run_id"`
	CallID        string                            `json:"call_id"`
	InteractionID string                            `json:"interaction_id"`
	Resource      string                            `json:"resource"`
	Revision      int64                             `json:"revision"`
	BaseExists    bool                              `json:"base_exists"`
	BaseHash      string                            `json:"base_hash,omitempty"`
	Base          json.RawMessage                   `json:"base"`
	Proposal      json.RawMessage                   `json:"proposal"`
	Draft         json.RawMessage                   `json:"draft"`
	Target        model.PublicTarget                `json:"target"`
	State         string                            `json:"state"`
	Answer        *model.ResourceEditApprovalAnswer `json:"answer,omitempty"`
}

func resourceApprovalID(callID string) string {
	sum := sha256.Sum256([]byte(callID))
	return "resa_" + hex.EncodeToString(sum[:16])
}

func resourceApprovalPath(projectDir, runID, interactionID string) (string, error) {
	if !strings.HasPrefix(interactionID, "resa_") || len(interactionID) != len("resa_")+32 {
		return "", ErrResourceApprovalConflict
	}
	for _, c := range interactionID[len("resa_"):] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", ErrResourceApprovalConflict
		}
	}
	runHash := sha256.Sum256([]byte(runID))
	return filepath.Join(projectDir, ".runtime", "resource-approvals", hex.EncodeToString(runHash[:16]), interactionID+".json"), nil
}

func FinalizeResourceEditApprovals(projectDir, runID string) error {
	resourceApprovalMu.Lock()
	defer resourceApprovalMu.Unlock()
	runHash := sha256.Sum256([]byte(runID))
	directory := filepath.Join(projectDir, ".runtime", "resource-approvals", hex.EncodeToString(runHash[:16]))
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var record ResourceEditApproval
		if err := json.Unmarshal(raw, &record); err != nil {
			return err
		}
		if record.RunID != runID || record.State != "answered" || record.Answer == nil {
			if err := os.Remove(path); err != nil {
				return err
			}
			continue
		}
		// Keep only the decision receipt for idempotent retries after the Run ends.
		record.Base, record.Proposal, record.Draft, record.Target = nil, nil, nil, model.PublicTarget{}
		if err := saveResourceApprovalLocked(projectDir, record); err != nil {
			return err
		}
	}
	return nil
}

func loadResourceApprovalLocked(projectDir, runID, interactionID string) (ResourceEditApproval, error) {
	path, err := resourceApprovalPath(projectDir, runID, interactionID)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	var record ResourceEditApproval
	if err := json.Unmarshal(raw, &record); err != nil {
		return ResourceEditApproval{}, err
	}
	if record.RunID != runID || record.InteractionID != interactionID {
		return ResourceEditApproval{}, ErrResourceApprovalConflict
	}
	return record, nil
}

func saveResourceApprovalLocked(projectDir string, record ResourceEditApproval) error {
	path, err := resourceApprovalPath(projectDir, record.RunID, record.InteractionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".approval-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func GetResourceEditApproval(projectDir, runID, interactionID string) (ResourceEditApproval, error) {
	resourceApprovalMu.Lock()
	defer resourceApprovalMu.Unlock()
	return loadResourceApprovalLocked(projectDir, runID, interactionID)
}

func CreateResourceEditApproval(projectDir, runID, callID, resource string, tx *RunSession) (ResourceEditApproval, error) {
	resourceApprovalMu.Lock()
	defer resourceApprovalMu.Unlock()
	id := resourceApprovalID(callID)
	if old, err := loadResourceApprovalLocked(projectDir, runID, id); err == nil {
		return old, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return ResourceEditApproval{}, err
	}
	path := "." + resource + ".json"
	base, err := tx.ReadBaseline(ArtifactRef{Kind: ArtifactDerived, ID: path, Path: path})
	baseExists := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ResourceEditApproval{}, err
	}
	proposal, err := tx.ReadPath(path)
	if err != nil {
		return ResourceEditApproval{}, err
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
		return ResourceEditApproval{}, errors.New("resource approval has no changed target")
	}
	record := ResourceEditApproval{RunID: runID, CallID: callID, InteractionID: id, Resource: resource,
		Revision: 1, BaseExists: baseExists, BaseHash: hashBytes(base), Base: json.RawMessage(base),
		Proposal: json.RawMessage(proposal), Draft: json.RawMessage(proposal), Target: target, State: "pending"}
	if err := saveResourceApprovalLocked(projectDir, record); err != nil {
		return ResourceEditApproval{}, err
	}
	return record, nil
}

func resourceApprovalBaselineMatches(projectDir string, record ResourceEditApproval) bool {
	raw, err := os.ReadFile(filepath.Join(projectDir, "."+record.Resource+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return !record.BaseExists
	}
	return err == nil && record.BaseExists && hashBytes(raw) == record.BaseHash
}

func ResourceEditApprovalBaselineMatches(projectDir string, record ResourceEditApproval) bool {
	return resourceApprovalBaselineMatches(projectDir, record)
}

func UpdateResourceEditApproval(projectDir, runID, interactionID string, revision int64, draft json.RawMessage) (ResourceEditApproval, error) {
	resourceApprovalMu.Lock()
	defer resourceApprovalMu.Unlock()
	record, err := loadResourceApprovalLocked(projectDir, runID, interactionID)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	if record.State != "pending" || record.Revision != revision || !resourceApprovalBaselineMatches(projectDir, record) {
		return ResourceEditApproval{}, ErrResourceApprovalConflict
	}
	if len(draft) > maxPPTContentBytes {
		return ResourceEditApproval{}, fmt.Errorf("draft exceeds content limit")
	}
	sandbox, err := artifactfs.NewSandbox(projectDir)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	buffer := pptmutation.NewBuffer(sandbox)
	engine := pptmutation.Service{Workspace: buffer}
	if _, err := engine.ReplaceApprovedResource(record.Resource, draft); err != nil {
		return ResourceEditApproval{}, err
	}
	path := "." + record.Resource + ".json"
	normalized, err := buffer.Read(path)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	if bytes.Equal(normalized, record.Draft) {
		return record, nil
	}
	record.Draft = append(json.RawMessage(nil), normalized...)
	record.Revision++
	record.Target = resourceApprovalDiff(projectDir, record)
	if err := saveResourceApprovalLocked(projectDir, record); err != nil {
		return ResourceEditApproval{}, err
	}
	return record, nil
}

func resourceApprovalDiff(projectDir string, record ResourceEditApproval) model.PublicTarget {
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

func DecideResourceEditApproval(projectDir, runID, interactionID string, answer model.ResourceEditApprovalAnswer, submit func() error) (ResourceEditApproval, error) {
	resourceApprovalMu.Lock()
	defer resourceApprovalMu.Unlock()
	record, err := loadResourceApprovalLocked(projectDir, runID, interactionID)
	if err != nil {
		return ResourceEditApproval{}, err
	}
	if record.State == "answered" && record.Answer != nil && *record.Answer == answer {
		return record, nil
	}
	if record.State != "pending" || record.CallID != answer.CallID || record.Revision != answer.Revision ||
		(answer.Decision != "approve" && answer.Decision != "reject") || !resourceApprovalBaselineMatches(projectDir, record) {
		return ResourceEditApproval{}, ErrResourceApprovalConflict
	}
	if err := submit(); err != nil {
		return ResourceEditApproval{}, err
	}
	record.State = "answered"
	record.Answer = &answer
	if err := saveResourceApprovalLocked(projectDir, record); err != nil {
		return ResourceEditApproval{}, err
	}
	return record, nil
}

func stageApprovedResource(projectDir string, pack contextengine.ContextPack, tx *RunSession, record ResourceEditApproval) (ToolResult, error) {
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
	out.Data = map[string]any{"changed": changed}
	if record.Resource == "outline" {
		out.Data["content"] = string(raw)
	} else {
		var content any
		if err := json.Unmarshal(raw, &content); err != nil {
			return ToolResult{}, err
		}
		out.Data["content"] = content
		fields := []string{}
		if record.Target.Diff != nil {
			for _, field := range record.Target.Diff.Fields {
				fields = append(fields, field.Field)
			}
		}
		out.Data["changed_fields"] = fields
	}
	if changed {
		out.ChangedTargets = []ChangedTarget{{Type: "deck", Part: record.Resource, Hash: hashBytes(raw)}}
	}
	out.Evidence = []Evidence{newEvidence("schema", resource, hashBytes(raw), map[string]any{"valid": true})}
	setResourceObservation(&out, resource, raw)
	return out, nil
}
