package llm

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
	"gopkg.in/yaml.v3"
)

func TestSettingsSaveIsAtomicAndPreservesPinnedCredentials(t *testing.T) {
	cfg := config.LLMConfig{
		Profiles: []config.LLMProfile{{Name: "Main", Provider: "openai", Model: "main", Key: "private-main-key"}, {Name: "Side", Provider: "kimi", Model: "mini", Key: "private-side-key"}},
		MainRoad: config.MainRoadLLMConfig{Default: "Main"}, SideRoad: config.SideRoadLLMConfig{Default: "Side"},
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw, _ := yaml.Marshal(cfg)
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
		{PreviousName: "Main", Name: "Renamed", Provider: "openai", Model: "main"},
		{PreviousName: "Side", Name: "Side", Provider: "kimi", Model: "mini"},
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
	decoded, err := config.ParseLLMConfig(written)
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

func TestSettingsRejectsProviderChangeWithoutReplacementKey(t *testing.T) {
	cfg := config.LLMConfig{Profiles: []config.LLMProfile{{Name: "Model", Provider: "openai", Model: "m", Key: "secret"}}, MainRoad: config.MainRoadLLMConfig{Default: "Model"}, SideRoad: config.SideRoadLLMConfig{Default: "Model"}}
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw, _ := yaml.Marshal(cfg)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := NewConfiguredRegistry(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := registry.Settings()
	edit := SettingsEdit{Revision: settings.Revision, Profiles: []ProfileEdit{{PreviousName: "Model", Name: "Model", Provider: "kimi", Model: "m"}}, Main: cfg.MainRoad, Side: cfg.SideRoad}
	if _, err := registry.SaveSettings(edit); err == nil {
		t.Fatal("credentials were reused across providers")
	}
	replacement := "new-secret"
	edit.Profiles[0].Key = &replacement
	if _, err := registry.SaveSettings(edit); err != nil {
		t.Fatal(err)
	}
}
