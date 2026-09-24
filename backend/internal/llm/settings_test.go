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
		Profiles: []config.LLMProfile{{Name: "Main", Provider: "openai", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "main", Key: "private-main-key"}, {Name: "Side", Provider: "kimi", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "mini", Key: "private-side-key"}},
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
		{PreviousName: "Main", Name: "Renamed", Provider: "openai", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "main"},
		{PreviousName: "Side", Name: "Side", Provider: "kimi", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "mini"},
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
	cfg := config.LLMConfig{Profiles: []config.LLMProfile{{Name: "Model", Provider: "openai", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "m", Key: "secret"}}, MainRoad: config.MainRoadLLMConfig{Default: "Model"}, SideRoad: config.SideRoadLLMConfig{Default: "Model"}}
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
	edit := SettingsEdit{Revision: settings.Revision, Profiles: []ProfileEdit{{PreviousName: "Model", Name: "Model", Provider: "kimi", Protocol: "anthropic", BaseURL: "https://api.moonshot.cn/anthropic/v1", Model: "m"}}, Main: cfg.MainRoad, Side: cfg.SideRoad}
	if _, err := registry.SaveSettings(edit); err == nil {
		t.Fatal("credentials were reused across providers")
	}
	replacement := "new-secret"
	edit.Profiles[0].Key = &replacement
	if _, err := registry.SaveSettings(edit); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsPinsProtocolAndEndpointAndRequiresReplacementKey(t *testing.T) {
	for _, change := range []string{"endpoint", "protocol"} {
		t.Run(change, func(t *testing.T) {
			cfg := config.LLMConfig{Profiles: []config.LLMProfile{{Name: "Model", Provider: "openai", Protocol: ProtocolResponses, BaseURL: "https://first.example/v1", Model: "m", Key: "old-secret"}}, MainRoad: config.MainRoadLLMConfig{Default: "Model"}, SideRoad: config.SideRoadLLMConfig{Default: "Model"}}
			path := filepath.Join(t.TempDir(), "config.yaml")
			raw, _ := yaml.Marshal(cfg)
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			r, err := NewConfiguredRegistry(path, cfg)
			if err != nil {
				t.Fatal(err)
			}
			pinned := r.Snapshot()
			settings, _ := r.Settings()
			edit := SettingsEdit{Revision: settings.Revision, Profiles: []ProfileEdit{{PreviousName: "Model", Name: "Model", Provider: "openai", Protocol: ProtocolResponses, BaseURL: "https://first.example/v1", Model: "m"}}, Main: cfg.MainRoad, Side: cfg.SideRoad}
			if change == "endpoint" {
				edit.Profiles[0].BaseURL = "https://second.example/prefix/v1/"
			} else {
				edit.Profiles[0].Protocol = ProtocolAnthropic
			}
			if _, err := r.SaveSettings(edit); err == nil {
				t.Fatal("old credential crossed a connection boundary")
			}
			afterFailure, _ := os.ReadFile(path)
			if string(afterFailure) != string(raw) {
				t.Fatal("failed save changed the configuration")
			}
			key := "replacement-key"
			edit.Profiles[0].Key = &key
			after, err := r.SaveSettings(edit)
			if err != nil {
				t.Fatal(err)
			}
			old, _ := pinned.Resolve("Model")
			current, _ := r.Resolve("Model")
			if old.Protocol() != ProtocolResponses || old.URL() != "https://first.example/v1" || current.Protocol() != edit.Profiles[0].Protocol || current.URL() != config.NormalizeModelBaseURL(edit.Profiles[0].BaseURL) {
				t.Fatal("save changed a pinned connection or lost the new connection")
			}
			if after.Profiles[0].Protocol != current.Protocol() || after.Profiles[0].BaseURL != current.URL() || pinned.routing.Fingerprint == r.Snapshot().routing.Fingerprint {
				t.Fatal("published configuration did not include protocol and endpoint")
			}
		})
	}
}

func TestReloadSettingsPublishesOnlyValidChanges(t *testing.T) {
	cfg := config.LLMConfig{
		Profiles: []config.LLMProfile{{Name: "Main", Provider: "openai", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "m", Key: "old-secret"}},
		MainRoad: config.MainRoadLLMConfig{Default: "Main"}, SideRoad: config.SideRoadLLMConfig{Default: "Main"},
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	write(raw)
	registry, err := NewConfiguredRegistry(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := registry.Settings()
	pinned := registry.Snapshot()

	write(append(raw, []byte("\n# comment only\n")...))
	unchanged, err := registry.ReloadSettings()
	if err != nil || unchanged.Revision != before.Revision {
		t.Fatal("comment-only refresh invalidated the revision", err)
	}

	// Credential-only edits must also create a new configuration version.
	cfg.Profiles[0].Key = "new-secret"
	raw, err = yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	write(raw)
	after, err := registry.ReloadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if after.Revision == before.Revision || pinned.Public().Revision != before.Revision {
		t.Fatal("reload failed to publish a new version while preserving the pinned one")
	}
	public, err := json.Marshal(after)
	if err != nil || strings.Contains(string(public), "secret") {
		t.Fatal("reload exposed credentials")
	}
	edit := SettingsEdit{Revision: before.Revision, Profiles: []ProfileEdit{{PreviousName: "Main", Name: "Main", Provider: "openai", Protocol: "responses", BaseURL: "https://api.openai.com/v1", Model: "m"}}, Main: cfg.MainRoad, Side: cfg.SideRoad}
	_, err = registry.SaveSettings(edit)
	var conflict *SettingsError
	if !errors.As(err, &conflict) || conflict.Code != "SETTINGS_REVISION_CONFLICT" {
		t.Fatal("stale editor overwrote the reloaded version")
	}

	write([]byte("llm: [invalid-secret"))
	_, err = registry.ReloadSettings()
	if err == nil || strings.Contains(err.Error(), "invalid-secret") {
		t.Fatal("invalid reload succeeded or exposed YAML contents")
	}
	active, _ := registry.Settings()
	if active.Revision != after.Revision {
		t.Fatal("invalid reload replaced the active configuration")
	}
	write(raw)
	again, err := registry.ReloadSettings()
	if err != nil || again.Revision != after.Revision {
		t.Fatal("unchanged refresh created a new revision", err)
	}

	edit.Revision = after.Revision
	if _, err = registry.SaveSettings(edit); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := config.ParseLLMConfig(written)
	if err != nil || decoded.Profiles[0].Key != "new-secret" {
		t.Fatal("save did not retain the reloaded credential")
	}
}
