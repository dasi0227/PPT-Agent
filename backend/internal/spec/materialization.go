package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func ContentHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func SourceHash(outlineRaw, specRaw, designRaw []byte) string {
	size := len(outlineRaw) + len(specRaw) + len(designRaw) + 32
	combined := make([]byte, 0, size)
	for _, item := range []struct {
		name string
		raw  []byte
	}{
		{name: "outline", raw: outlineRaw},
		{name: "spec", raw: specRaw},
		{name: "design", raw: designRaw},
	} {
		combined = append(combined, item.name...)
		combined = append(combined, 0)
		combined = append(combined, item.raw...)
		combined = append(combined, 0)
	}
	return ContentHash(combined)
}

func ReadMaterialization(path string) (MaterializationRecord, error) {
	var value MaterializationRecord
	raw, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		return MaterializationRecord{}, err
	}
	if err := ValidateMaterialization(value); err != nil {
		return MaterializationRecord{}, err
	}
	return value, nil
}

func DeriveMaterializationState(
	hasHTML bool,
	record *MaterializationRecord,
	currentOutline, currentSpec, currentDesign int,
	artifactHash, sourceHash string,
) string {
	if !hasHTML {
		return "not_materialized"
	}
	if record == nil {
		return "unknown"
	}
	if record.Artifact.Hash != artifactHash ||
		record.Source.Outline > currentOutline ||
		record.Source.Spec > currentSpec ||
		record.Source.Design > currentDesign {
		return "unknown"
	}
	if record.Source.Outline < currentOutline || record.Source.Spec < currentSpec {
		return "spec_stale"
	}
	if record.Source.Design < currentDesign {
		return "design_stale"
	}
	if record.Source.Hash != sourceHash {
		return "unknown"
	}
	return "fresh"
}
