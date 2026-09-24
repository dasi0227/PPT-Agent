// Package shortcuts defines the application key bindings shared with the frontend.
package shortcuts

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

//go:embed catalog.json
var catalogJSON []byte

type Binding struct {
	Trigger string `json:"trigger,omitempty"`
	Code    string `json:"code,omitempty"`
	Primary bool   `json:"primary,omitempty"`
	Alt     bool   `json:"alt,omitempty"`
	Shift   bool   `json:"shift,omitempty"`
}
type Definition struct {
	ID      string  `json:"id"`
	Group   string  `json:"group"`
	Label   string  `json:"label"`
	Kind    string  `json:"kind"`
	Default Binding `json:"default"`
}

var Catalog = func() []Definition {
	var result []Definition
	if err := json.Unmarshal(catalogJSON, &result); err != nil {
		panic(err)
	}
	return result
}()

type Settings struct {
	Revision int64              `json:"revision"`
	Bindings map[string]Binding `json:"bindings"`
}

var ErrConflict = errors.New("快捷键设置已在其它窗口更新，请重新读取后再保存")
var validCode = regexp.MustCompile(`^(Key[A-Z]|Digit[0-9]|Arrow(Left|Right|Up|Down)|Enter|Space|Equal|Minus|BracketLeft|BracketRight|Backslash|Semicolon|Quote|Comma|Period|Slash|Backquote)$`)

const TriggerCharacters = "/@$#%!?&*~^:;=+-_.|\\"

func Validate(bindings map[string]Binding) error {
	if len(bindings) != len(Catalog) {
		return errors.New("快捷键配置必须包含所有功能")
	}
	seen := map[string]string{}
	for _, def := range Catalog {
		b, ok := bindings[def.ID]
		if !ok {
			return fmt.Errorf("缺少配置：%s", def.Label)
		}
		var signature string
		if def.Kind == "trigger" {
			if len(b.Trigger) != 1 || !strings.Contains(TriggerCharacters, b.Trigger) || b.Code != "" || b.Primary || b.Alt || b.Shift {
				return fmt.Errorf("%s：请输入单个支持的标点符号", def.Label)
			}
			signature = "trigger:" + b.Trigger
		} else {
			if b.Trigger != "" || !validCode.MatchString(b.Code) || (!b.Primary && !b.Alt) || (b.Code == "Equal" && b.Shift) {
				return fmt.Errorf("%s：请至少使用 Command/Ctrl 或 Option/Alt，可组合 Shift", def.Label)
			}
			if b.Primary && strings.Contains("|KeyA|KeyC|KeyV|KeyX|KeyZ|KeyY|KeyF|KeyL|KeyQ|KeyW|KeyR|KeyN|", "|"+b.Code+"|") {
				return fmt.Errorf("%s：该组合保留给系统编辑或浏览器操作", def.Label)
			}
			signature = fmt.Sprintf("key:%s:%t:%t:%t", b.Code, b.Primary, b.Alt, b.Shift)
		}
		if label, exists := seen[signature]; exists {
			return fmt.Errorf("%s 与 %s 的绑定冲突", label, def.Label)
		}
		seen[signature] = def.Label
	}
	return nil
}

func Defaults() map[string]Binding {
	result := map[string]Binding{}
	for _, def := range Catalog {
		result[def.ID] = def.Default
	}
	return result
}

// Storage saves only overrides, guarded by an atomic revision check.
type Storage interface {
	ReadShortcutSettings(context.Context) (int64, map[string]Binding, error)
	WriteShortcutSettings(context.Context, int64, map[string]Binding) error
}
type Service struct{ store Storage }

func NewService(store Storage) *Service { return &Service{store: store} }
func (s *Service) Get(ctx context.Context) (Settings, error) {
	revision, overrides, err := s.store.ReadShortcutSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	bindings := Defaults()
	for id, value := range overrides {
		bindings[id] = value
	}
	if err = Validate(bindings); err != nil {
		return Settings{}, err
	}
	return Settings{revision, bindings}, nil
}
func (s *Service) Save(ctx context.Context, edit Settings) (Settings, error) {
	if err := Validate(edit.Bindings); err != nil {
		return Settings{}, err
	}
	overrides := map[string]Binding{}
	for _, def := range Catalog {
		if edit.Bindings[def.ID] != def.Default {
			overrides[def.ID] = edit.Bindings[def.ID]
		}
	}
	if err := s.store.WriteShortcutSettings(ctx, edit.Revision, overrides); err != nil {
		return Settings{}, err
	}
	return Settings{edit.Revision + 1, edit.Bindings}, nil
}
