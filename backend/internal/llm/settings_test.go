package llm

import (
	"encoding/json"
	"errors"
	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func marshalSettingsConfig(cfg config.LLMConfig) ([]byte, error) {
	return yaml.Marshal(config.FileConfig{
		App: config.AppConfig{Port: 8787}, LLMConfig: cfg,
		Agent: config.DefaultAgentConfig(), Sandbox: config.DefaultSandboxConfig(),
	})
}

func TestSettingsSaveIsAtomicAndPreservesPinnedCredentials(t *testing.T) {
	cfg := config.LLMConfig{
		Profiles: []config.LLMProfile{{Name: "Main", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "main", Key: "private-main-key"}, {Name: "Side", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "mini", Key: "private-side-key"}},
		MainRoad: config.MainRoadLLMConfig{Default: "Main"}, SideRoad: config.SideRoadLLMConfig{Default: "Side"},
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw, _ := marshalSettingsConfig(cfg)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewConfiguredRegistry(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	pinned := registry.Snapshot()
	before, _ := registry.Settings()
	public, _ := json.Marshal(before)
	if strings.Contains(string(public), "private-") {
		t.Fatal("read settings exposed credentials")
	}
	edit := SettingsEdit{Revision: before.Revision, Profiles: []ProfileEdit{
		{PreviousName: "Main", Name: "Renamed", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "main"},
		{PreviousName: "Side", Name: "Side", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "mini"},
	}, Main: config.MainRoadLLMConfig{Default: "Renamed", Fallback: "Side"}, Side: cfg.SideRoad}
	after, err := registry.SaveSettings(edit)
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision == before.Revision || registry.Default() != "Renamed" || pinned.Default() != "Main" {
		t.Fatal("save did not preserve the in-flight snapshot")
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := config.ParseFileConfig(written)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Profiles[0].Key != "private-main-key" || decoded.Profiles[1].Key != "private-side-key" {
		t.Fatal("renaming lost an unchanged credential")
	}
	if _, err := registry.SaveSettings(edit); err == nil {
		t.Fatal("stale settings overwrote a newer revision")
	}
	edit.Revision = after.Revision
	edit.Profiles[0].PreviousName = "Renamed"
	edit.Main.Default = "Deleted"
	if _, err := registry.SaveSettings(edit); err == nil {
		t.Fatal("dangling model reference was accepted")
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != string(written) || registry.Default() != "Renamed" {
		t.Fatal("invalid save changed active settings or disk")
	}
	// An external edit is never silently overwritten by the UI.
	if err := os.WriteFile(path, append(written, []byte("\n# external edit\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	edit.Main.Default = "Renamed"
	_, err = registry.SaveSettings(edit)
	var conflict *SettingsError
	if !errors.As(err, &conflict) || conflict.Code != "SETTINGS_FILE_CHANGED" {
		t.Fatal("external edit was not protected")
	}
}
