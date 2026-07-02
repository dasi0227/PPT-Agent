package slidejson

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed slide-json.schema.json
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
		if err := c.AddResource("slide-json.schema.json", strings.NewReader(string(schemaBytes))); err != nil {
			compileErr = err
			return
		}
		compiled, compileErr = c.Compile("slide-json.schema.json")
	})
	return compiled, compileErr
}

// ValidateRaw 校验一段 JSON 文本是否符合 slide-json schema（SPEC-OUTLINE-001）。
func ValidateRaw(raw []byte) error {
	sch, err := schema()
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := sch.Validate(v); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// Validate 校验一个 SlideJSON 值（先序列化再按 schema 校验，保证与落盘一致）。
func Validate(s SlideJSON) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return ValidateRaw(raw)
}

// Parse 反序列化一段 slide.json 文本为 SlideJSON（不做 schema 校验）。
func Parse(raw []byte) (SlideJSON, error) {
	var s SlideJSON
	if err := json.Unmarshal(raw, &s); err != nil {
		return SlideJSON{}, fmt.Errorf("parse slide-json: %w", err)
	}
	return s, nil
}

// LayoutEnum 返回 schema 中 layout 的合法枚举值（供工具错误提示与 prompt 使用）。
func LayoutEnum() []string {
	return append([]string(nil), layoutEnum...)
}

// layoutEnum 从内嵌 schema 解析一次，保证与权威 enum 同源（不硬编码第二份）。
var layoutEnum = mustLayoutEnum()

func mustLayoutEnum() []string {
	var doc struct {
		Properties struct {
			Layout struct {
				Enum []string `json:"enum"`
			} `json:"layout"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &doc); err != nil {
		panic("slidejson: cannot parse embedded schema: " + err.Error())
	}
	return doc.Properties.Layout.Enum
}

// IsValidLayout 报告 layout 是否在 schema enum 内（DS-LAYOUTS-001）。
func IsValidLayout(layout string) bool {
	for _, l := range layoutEnum {
		if l == layout {
			return true
		}
	}
	return false
}
