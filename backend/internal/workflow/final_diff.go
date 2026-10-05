package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/pmezard/go-difflib/difflib"
)

// Terminal events freeze the Run-start -> durable terminal state, not a sum of
// intermediate operations. The original baseline survives continuation.
func finalAffectedTargets(state *RunState) []model.PublicTarget {
	raw, err := os.ReadFile(reviewBaselinePath(state.projectDir, state.runID))
	var before map[string]reviewSourceFile
	if err == nil {
		err = json.Unmarshal(raw, &before)
	}
	var after map[string]reviewSourceFile
	if err == nil {
		after, err = reviewSourceFiles(context.Background(), state.projectDir)
	}
	if err != nil {
		recordTrace(state.trace, state.runID, "final_diff.failed", map[string]any{"error": err.Error()})
		targets := publicAffectedTargets(state.projectDir, state.committedChanges)
		for i := range targets {
			targets[i].Insertions, targets[i].Deletions = 0, 0
			filename := targets[i].DisplayName
			if targets[i].Type == "slide" {
				filename = model.SpecCollectionPath
				if targets[i].Part == "html" {
					filename = model.SlideHTMLPath(targets[i].SlideID)
				}
			} else if targets[i].Type == "deck" {
				filename = "." + targets[i].Part + ".json"
			}
			targets[i].Diff = &model.ArtifactDiff{Kind: "unavailable", Status: "modified", Filename: filename, Error: "变更差异读取失败。"}
		}
		return targets
	}
	return sourceDiffTargets(state.projectDir, before, after)
}

// Capture the current operation before CommitOperation clears its preimages.
// Only staged authored files are compared; unrelated disk edits are excluded.
func operationDiffTargets(session *RunSession) []model.PublicTarget {
	before, after := map[string]reviewSourceFile{}, map[string]reviewSourceFile{}
	source := func(raw []byte, hash string) reviewSourceFile {
		binary := !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0
		file := reviewSourceFile{Hash: hash, Size: len(raw), Binary: binary}
		if !binary {
			file.Content = string(raw)
		}
		return file
	}
	for path, entry := range session.artifacts {
		switch strings.Split(path, "/")[0] {
		case ".runtime", ".git", ".run", ".commit-tmp", "versions":
			continue
		}
		if entry.Existed {
			before[path] = source(entry.BeforeContent, entry.BeforeHash)
		}
		if !entry.Delete {
			after[path] = source(entry.AfterContent, entry.AfterHash)
		}
	}
	// An unchanged outline supplies frozen page ordinals without adding a diff.
	if _, changed := session.artifacts[".outline.json"]; !changed {
		if raw, err := session.ReadPath(".outline.json"); err == nil {
			outline := source(raw, hashBytes(raw))
			before[".outline.json"], after[".outline.json"] = outline, outline
		}
	}
	return sourceDiffTargets(session.projectDir, before, after)
}

func sourceDiffTargets(root string, before, after map[string]reviewSourceFile) []model.PublicTarget {
	targets := []model.PublicTarget{}
	ordinals := map[string]int{}
	for _, files := range []map[string]reviewSourceFile{before, after} {
		var outline spec.Outline
		if json.Unmarshal([]byte(files[".outline.json"].Content), &outline) == nil {
			for i, slide := range spec.FlattenOutline(outline) {
				ordinals[slide.Slide.ID] = i + 1
			}
		}
	}
	appendTarget := func(resource Resource, filename string, old, next reviewSourceFile, existed, exists bool) {
		if existed && exists && old.Hash == next.Hash {
			return
		}
		target := publicTarget(root, resource)
		if n := ordinals[resource.SlideID]; n > 0 {
			target.DisplayName = fmt.Sprintf("第 %d 页", n)
		}
		diff := &model.ArtifactDiff{Filename: filename, Status: "modified"}
		if !existed {
			diff.Status = "added"
		}
		if !exists {
			diff.Status = "deleted"
		}
		if old.Binary || next.Binary {
			diff.Kind = "binary"
		} else if resource.Part != "html" && strings.HasSuffix(filename, ".json") {
			var a, b any
			var parseErr error
			if existed {
				parseErr = decodeDiffJSON(old.Content, &a)
			}
			if exists && parseErr == nil {
				parseErr = decodeDiffJSON(next.Content, &b)
			}
			if parseErr != nil {
				diff.Kind, diff.Error = "unavailable", "结构化内容解析失败。"
			} else if resource.Part == "outline" {
				diff.Kind = "outline"
				diff.Groups, parseErr = outlineTreeDiff(a, b)
				if parseErr != nil {
					diff.Kind, diff.Error = "unavailable", "目录结构解析失败。"
				} else {
					for _, group := range diff.Groups {
						for _, row := range group.Rows {
							if row.Kind == "added" {
								target.Insertions++
							}
							if row.Kind == "removed" {
								target.Deletions++
							}
						}
					}
					if target.Insertions == 0 && target.Deletions == 0 && existed && exists {
						return
					}
				}
			} else {
				diff.Kind = "fields"
				diff.Fields = jsonFieldDiffValues(a, b, existed, exists, "")
				if len(diff.Fields) == 0 {
					if existed && exists {
						return
					}
					diff.Fields = []model.FieldDiff{}
				}
				for _, field := range diff.Fields {
					for _, row := range field.Rows {
						if row.Kind == "added" {
							target.Insertions++
						} else {
							target.Deletions++
						}
					}
				}
			}
		} else {
			diff.Kind = "text"
			diff.Hunks, target.Insertions, target.Deletions = textDiff(old.Content, next.Content)
			if len(diff.Hunks) == 0 {
				return
			}
		}
		target.Diff = diff
		targets = append(targets, target)
	}
	paths := map[string]bool{}
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	for path := range paths {
		a, existed := before[path]
		b, exists := after[path]
		if path == model.SpecCollectionPath {
			oldEntries, nextEntries := map[string]json.RawMessage{}, map[string]json.RawMessage{}
			oldErr, nextErr := error(nil), error(nil)
			if existed {
				oldErr = json.Unmarshal([]byte(a.Content), &oldEntries)
			}
			if exists {
				nextErr = json.Unmarshal([]byte(b.Content), &nextEntries)
			}
			if oldErr != nil || nextErr != nil {
				appendTarget(Resource{Type: "file", Part: "content", Path: path}, path, a, b, existed, exists)
				continue
			}
			ids := map[string]bool{}
			for id := range oldEntries {
				ids[id] = true
			}
			for id := range nextEntries {
				ids[id] = true
			}
			for id := range ids {
				old, had := oldEntries[id]
				next, has := nextEntries[id]
				appendTarget(Resource{Type: "slide", Part: "spec", SlideID: id}, path,
					reviewSourceFile{Content: string(old), Hash: hashBytes(old)}, reviewSourceFile{Content: string(next), Hash: hashBytes(next)}, had, has)
			}
			continue
		}
		resource := Resource{Type: "file", Part: "content", Path: path}
		switch path {
		case ".manifest.json":
			resource = Resource{Type: "deck", Part: "manifest"}
		case ".design.json":
			resource = Resource{Type: "deck", Part: "design"}
		case ".outline.json":
			resource = Resource{Type: "deck", Part: "outline"}
		default:
			id := strings.TrimSuffix(path, ".html")
			if id != path && stableSlideID.MatchString(id) {
				resource = Resource{Type: "slide", Part: "html", SlideID: id}
			}
		}
		appendTarget(resource, path, a, b, existed, exists)
	}
	ranks := map[string]int{"manifest": 0, "outline": 1, "design": 2, "spec": 3, "html": 4, "content": 5}
	sort.Slice(targets, func(i, j int) bool {
		a, b := targets[i], targets[j]
		if ranks[a.Part] != ranks[b.Part] {
			return ranks[a.Part] < ranks[b.Part]
		}
		if ordinals[a.SlideID] != ordinals[b.SlideID] {
			return ordinals[a.SlideID] < ordinals[b.SlideID]
		}
		return a.Diff.Filename+":"+a.SlideID < b.Diff.Filename+":"+b.SlideID
	})
	return targets
}

func decodeDiffJSON(content string, value *any) error {
	if !json.Valid([]byte(content)) {
		return fmt.Errorf("invalid JSON source")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(content))
	decoder.UseNumber()
	return decoder.Decode(value)
}

func fieldRow(kind string, value any) model.FieldDiffRow {
	raw, _ := json.Marshal(value)
	return model.FieldDiffRow{Kind: kind, Value: string(raw)}
}
func jsonFieldDiff(before, after any, path string) []model.FieldDiff {
	return jsonFieldDiffValues(before, after, before != nil, after != nil, path)
}
func jsonFieldDiffValues(before, after any, existed, exists bool, path string) []model.FieldDiff {
	if existed == exists && reflect.DeepEqual(before, after) {
		return nil
	}
	a, aObject := before.(map[string]any)
	b, bObject := after.(map[string]any)
	if (aObject || !existed) && (bObject || !exists) && (aObject || bObject) {
		keys := map[string]bool{}
		for key := range a {
			keys[key] = true
		}
		for key := range b {
			keys[key] = true
		}
		ordered := []string{}
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		fields := []model.FieldDiff{}
		for _, key := range ordered {
			child := key
			if path != "" {
				child = path + "." + key
			}
			old, had := a[key]
			next, has := b[key]
			fields = append(fields, jsonFieldDiffValues(old, next, had, has, child)...)
		}
		return fields
	}
	field := model.FieldDiff{Field: path, Rows: []model.FieldDiffRow{}}
	oldArray, oldList := before.([]any)
	newArray, newList := after.([]any)
	if (oldList || !existed) && (newList || !exists) && (oldList || newList) {
		encode := func(values []any) []string {
			out := make([]string, len(values))
			for i, v := range values {
				raw, _ := json.Marshal(v)
				out[i] = string(raw)
			}
			return out
		}
		matcher := difflib.NewMatcherWithJunk(encode(oldArray), encode(newArray), false, nil)
		ops := matcher.GetOpCodes()
		// All old values first, then all new values, preserving each side's order.
		for _, op := range ops {
			if op.Tag == 'd' || op.Tag == 'r' {
				for _, v := range oldArray[op.I1:op.I2] {
					field.Rows = append(field.Rows, fieldRow("removed", v))
				}
			}
		}
		for _, op := range ops {
			if op.Tag == 'i' || op.Tag == 'r' {
				for _, v := range newArray[op.J1:op.J2] {
					field.Rows = append(field.Rows, fieldRow("added", v))
				}
			}
		}
	} else {
		if existed {
			field.Rows = append(field.Rows, fieldRow("removed", before))
		}
		if exists {
			field.Rows = append(field.Rows, fieldRow("added", after))
		}
	}
	if len(field.Rows) == 0 {
		return nil
	}
	return []model.FieldDiff{field}
}

func textLines(source string) []string {
	if source == "" {
		return nil
	}
	lines := strings.SplitAfter(source, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
func textDiff(before, after string) ([]model.DiffHunk, int, int) {
	a, b := textLines(before), textLines(after)
	matcher := difflib.NewMatcherWithJunk(a, b, false, nil)
	added, removed := 0, 0
	hunks := []model.DiffHunk{}
	previousOldEnd, previousNewEnd := 0, 0
	for _, group := range matcher.GetGroupedOpCodes(3) {
		first, last := group[0], group[len(group)-1]
		hunk := model.DiffHunk{OldStart: first.I1 + 1, OldCount: last.I2 - first.I1, NewStart: first.J1 + 1, NewCount: last.J2 - first.J1, Rows: []model.TextDiffRow{}}
		// Freeze omitted lines between hunks with the same operation snapshot.
		// They remain outside the changed-line counts and expand without disk reads.
		if len(hunks) > 0 {
			for i, j := previousOldEnd, previousNewEnd; i < first.I1; i, j = i+1, j+1 {
				hunk.ContextBefore = append(hunk.ContextBefore, model.TextDiffRow{Kind: "context", Text: strings.TrimSuffix(a[i], "\n"), OldLine: i + 1, NewLine: j + 1})
			}
		}
		if hunk.OldCount == 0 {
			hunk.OldStart = first.I1
		}
		if hunk.NewCount == 0 {
			hunk.NewStart = first.J1
		}
		for _, op := range group {
			if op.Tag == 'e' {
				for i, j := op.I1, op.J1; i < op.I2; i, j = i+1, j+1 {
					hunk.Rows = append(hunk.Rows, model.TextDiffRow{Kind: "context", Text: strings.TrimSuffix(a[i], "\n"), OldLine: i + 1, NewLine: j + 1})
				}
				continue
			}
			if op.Tag == 'd' || op.Tag == 'r' {
				for i := op.I1; i < op.I2; i++ {
					hunk.Rows = append(hunk.Rows, model.TextDiffRow{Kind: "removed", Text: strings.TrimSuffix(a[i], "\n"), OldLine: i + 1})
					removed++
				}
			}
			if op.Tag == 'i' || op.Tag == 'r' {
				for j := op.J1; j < op.J2; j++ {
					hunk.Rows = append(hunk.Rows, model.TextDiffRow{Kind: "added", Text: strings.TrimSuffix(b[j], "\n"), NewLine: j + 1})
					added++
				}
			}
		}
		hunks = append(hunks, hunk)
		previousOldEnd, previousNewEnd = last.I2, last.J2
	}
	return hunks, added, removed
}
