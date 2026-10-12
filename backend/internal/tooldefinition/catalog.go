package tooldefinition

import (
	"fmt"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/prompt"
)

type Schema = prompt.DocumentedTool

// The complete directory is validated before any role or permission filtering.
var catalog = mustLoadCatalog()

func mustLoadCatalog() map[string]Schema {
	contracts := map[string]prompt.ToolContract{}
	for id, codes := range errorContracts {
		if err := validateErrorCodes(id, codes); err != nil {
			panic(err)
		}
		name := strings.SplitN(id, ".", 2)[0]
		contracts[id] = prompt.ToolContract{Name: name, Input: inputSchema(id), Output: OutputContract(id), ErrorCodes: codes}
	}
	tools, err := prompt.LoadTools(contracts)
	if err != nil {
		panic(fmt.Errorf("tool directory: %w", err))
	}
	return tools
}

// Get returns a fresh input tree; callers cannot mutate the canonical directory.
func Get(id string) Schema {
	tool, ok := catalog[id]
	if !ok {
		panic("unknown tool documentation binding: " + id)
	}
	tool.Parameters = prompt.CloneToolSchema(tool.Parameters)
	return tool
}
