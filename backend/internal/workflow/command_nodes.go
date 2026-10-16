package workflow

import (
	"fmt"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/commandexec"
)

// Commands use the same durable operation journal as native editing tools.
// File metadata participates in conflicts and rollback, not just content bytes.
func (s *RunSession) stageCommandNode(path string, before commandexec.Tree, after *commandexec.Node) {
	entry := sessionArtifact{Ref: projectFileRef(path), Relative: path, Source: "run_command", AfterNode: after, Delete: after == nil, BeforeHash: hashBytes(nil), AfterHash: hashBytes(nil)}
	if old, ok := before[path]; ok {
		entry.BeforeNode = &old
		entry.Existed = true
		entry.BeforeContent = old.Content
		entry.BeforeHash = hashBytes(old.Content)
	}
	if after != nil {
		entry.AfterContent = after.Content
		entry.AfterHash = hashBytes(after.Content)
	}
	s.artifacts[path] = entry
}

func nodeHash(node *commandexec.Node) string {
	if node == nil {
		return ""
	}
	return node.Hash()
}

func restoreCommandNodes(root string, artifacts []RunSessionArtifactState, committed bool) error {
	desired := map[string]*commandexec.Node{}
	for _, entry := range artifacts {
		if entry.BeforeNode == nil && entry.AfterNode == nil {
			continue
		}
		if hashBytes(entry.BeforeContent) != entry.BeforeHash || hashBytes(entry.AfterContent) != entry.AfterHash {
			return fmt.Errorf("command mutation journal hash mismatch")
		}
		// ApplyNode validates lexical and parent paths, but validate before reading too.
		path := filepath.Clean(entry.Relative)
		if path == "." || filepath.IsAbs(path) || path == ".." || len(path) > 3 && path[:3] == "../" {
			return ErrInvalidArtifactPath
		}
		current, err := commandexec.ReadNode(filepath.Join(root, path))
		if err != nil {
			return err
		}
		hash := nodeHash(current)
		if hash != nodeHash(entry.BeforeNode) && hash != nodeHash(entry.AfterNode) {
			if committed {
				continue
			}
			return fmt.Errorf("%w: %s", ErrArtifactHashMismatch, path)
		}
		target := entry.BeforeNode
		if committed {
			target = entry.AfterNode
		}
		if hash != nodeHash(target) {
			desired[path] = target
		}
	}
	for _, path := range commandexec.OrderedChanges(desired) {
		if err := commandexec.ApplyNode(root, path, desired[path]); err != nil {
			return err
		}
		if err := syncDirectory(filepath.Dir(filepath.Join(root, path))); err != nil {
			return err
		}
	}
	return nil
}
