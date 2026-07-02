package asset

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/dasi0227/PPT-Agent/backend/internal/designsystem"
)

//go:embed asset-manifest.schema.json
var schemaBytes []byte

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

func schema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.Draft = jsonschema.Draft2020
		if err := c.AddResource("asset-manifest.schema.json", strings.NewReader(string(schemaBytes))); err != nil {
			compileErr = err
			return
		}
		compiled, compileErr = c.Compile("asset-manifest.schema.json")
	})
	return compiled, compileErr
}

// ValidateManifestRaw 用 asset-manifest schema 校验一段 manifest JSON（AC-ASSET-001）。
// 任一 kind 的 manifest 通过该校验即视为协议合规。
func ValidateManifestRaw(raw []byte) error {
	sch, err := schema()
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := sch.Validate(v); err != nil {
		return fmt.Errorf("manifest schema validation failed: %w", err)
	}
	return nil
}

// ValidateManifest 校验一个 Manifest 值（先序列化再按 schema 校验，保证与落盘一致）。
func ValidateManifest(m Manifest) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return ValidateManifestRaw(raw)
}

// ValidateThemeTokens 校验 theme 资产的 tokens.css 提供必需 token 全集（AC-ASSET-002）。
// 返回缺失 token 列表；空表示齐备。非 theme 资产不需要调用。
func ValidateThemeTokens(tokensCSS []byte) []string {
	return designsystem.LintTokens(tokensCSS)
}

// Parse 反序列化 manifest JSON 为 Manifest（不含 schema 校验，调用方按需先 ValidateManifestRaw）。
func Parse(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return m, nil
}
