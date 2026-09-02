package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestComponentResolveByNamesPreservesOrderAndDeduplicates(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/components/card/index.html"), componentFile("能力卡片", "Card", "<div>card</div>"))
	writeRepositoryFile(t, filepath.Join(root, "assets/components/chart/index.html"), componentFile("趋势图", "Chart", "<div>chart</div>"))

	components, err := NewComponentService(WorkRoot(root)).ResolveByNames([]string{" 趋势图 ", "能力卡片", "趋势图"})
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 2 || components[0].ID != "chart" || components[1].ID != "card" {
		t.Fatalf("resolved components=%+v", components)
	}
	if !strings.Contains(components[0].HTML, "<div>chart</div>") {
		t.Fatalf("component HTML was not loaded: %+v", components[0])
	}
}

func TestComponentResolveByNamesRejectsMissingAmbiguousDisabledAndTooMany(t *testing.T) {
	root := t.TempDir()
	writeRepositoryFile(t, filepath.Join(root, "assets/components/one/index.html"), componentFile("重名", "First", "<div>one</div>"))
	writeRepositoryFile(t, filepath.Join(root, "assets/components/two/index.html"), componentFile("重名", "Second", "<div>two</div>"))
	writeRepositoryFile(t, filepath.Join(root, "assets/components/off/index.html"), componentFile("已禁用", "Disabled", "<div>off</div>"))
	service := NewComponentService(WorkRoot(root))
	if _, err := service.SetDisabled("off", true); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		names  []string
		code   string
		status int
	}{
		{[]string{"不存在"}, "COMPONENT_NOT_FOUND", 404},
		{[]string{"重名"}, "COMPONENT_NAME_AMBIGUOUS", 409},
		{[]string{"已禁用"}, "COMPONENT_DISABLED", 409},
		{make([]string, model.MaxRunComponents+1), "COMPONENT_SELECTION_INVALID", 400},
		{[]string{" "}, "COMPONENT_SELECTION_INVALID", 400},
	}
	for _, test := range cases {
		_, err := service.ResolveByNames(test.names)
		var agentErr *model.AgentError
		if !errors.As(err, &agentErr) || agentErr.Code != test.code || agentErr.HTTPStatus() != test.status {
			t.Errorf("ResolveByNames(%q) error=%v status=%d, want %s/%d", test.names, err, agentErr.HTTPStatus(), test.code, test.status)
		}
	}
}

func TestComponentResolveByNamesEnforcesTotalHTMLBudget(t *testing.T) {
	root := t.TempDir()
	names := make([]string, 4)
	for index := range names {
		names[index] = fmt.Sprintf("Component %d", index)
		writeRepositoryFile(
			t,
			filepath.Join(root, "assets/components", fmt.Sprintf("c%d", index), "index.html"),
			componentFile(names[index], "Large component", strings.Repeat("x", 50<<10)),
		)
	}

	_, err := NewComponentService(WorkRoot(root)).ResolveByNames(names)
	var agentErr *model.AgentError
	if !errors.As(err, &agentErr) || agentErr.Code != "CONTEXT_BUDGET_EXCEEDED" || agentErr.HTTPStatus() != 413 {
		t.Fatalf("budget error=%v", err)
	}
}
