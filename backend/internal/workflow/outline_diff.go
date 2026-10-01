package workflow

import (
	"encoding/json"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/pmezard/go-difflib/difflib"
)

type outlineTreeNode struct {
	title, purpose, parent, kind, order string
	depth                               int
	children                            []string
}

type outlineTreeSnapshot struct {
	nodes map[string]outlineTreeNode
	roots []string
}

func outlineTree(value any) (outlineTreeSnapshot, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return outlineTreeSnapshot{}, err
	}
	var outline spec.Outline
	if err := json.Unmarshal(raw, &outline); err != nil {
		return outlineTreeSnapshot{}, err
	}
	tree := outlineTreeSnapshot{nodes: map[string]outlineTreeNode{}}
	add := func(id string, node outlineTreeNode) error {
		if id == "" {
			return fmt.Errorf("outline node ID is required")
		}
		if _, exists := tree.nodes[id]; exists {
			return fmt.Errorf("duplicate outline node ID")
		}
		tree.nodes[id] = node
		return nil
	}
	for i, section := range outline.Sections {
		chapter := outlineTreeNode{title: section.Title, purpose: section.Purpose, kind: "chapter", order: fmt.Sprintf("%02d", i+1)}
		for _, slide := range section.Slides {
			chapter.children = append(chapter.children, slide.SlideID)
			if err := add(slide.SlideID, outlineTreeNode{title: slide.Title, parent: section.ID, kind: "page", depth: 1}); err != nil {
				return tree, err
			}
		}
		for j, sub := range section.Subsections {
			chapter.children = append(chapter.children, sub.ID)
			subchapter := outlineTreeNode{title: sub.Title, purpose: sub.Purpose, parent: section.ID, kind: "subchapter", depth: 1, order: fmt.Sprintf("%02d.%d", i+1, j+1)}
			for _, slide := range sub.Slides {
				subchapter.children = append(subchapter.children, slide.SlideID)
				if err := add(slide.SlideID, outlineTreeNode{title: slide.Title, parent: sub.ID, kind: "page", depth: 2}); err != nil {
					return tree, err
				}
			}
			if err := add(sub.ID, subchapter); err != nil {
				return tree, err
			}
		}
		if err := add(section.ID, chapter); err != nil {
			return tree, err
		}
		tree.roots = append(tree.roots, section.ID)
	}
	return tree, nil
}

// Each affected chapter keeps its unchanged children as visible context.
// Stable IDs and sibling sequence matches avoid treating index shifts as moves.
func outlineTreeDiff(before, after any) ([]model.OutlineDiffGroup, error) {
	a, err := outlineTree(before)
	if err != nil {
		return nil, err
	}
	b, err := outlineTree(after)
	if err != nil {
		return nil, err
	}
	stable := map[string]bool{}
	markStable := func(old, next []string) {
		for _, op := range difflib.NewMatcherWithJunk(old, next, false, nil).GetOpCodes() {
			if op.Tag == 'e' {
				for _, id := range old[op.I1:op.I2] {
					stable[id] = true
				}
			}
		}
	}
	markStable(a.roots, b.roots)
	for id, old := range a.nodes {
		if next, exists := b.nodes[id]; exists {
			markStable(old.children, next.children)
		}
	}
	row := func(node outlineTreeNode, kind string) model.OutlineDiffRow {
		return model.OutlineDiffRow{Kind: kind, Node: node.kind, Depth: node.depth, Title: node.title, Order: node.order}
	}
	purpose := func(node outlineTreeNode, kind string) model.OutlineDiffRow {
		return model.OutlineDiffRow{Kind: kind, Node: "purpose", Depth: node.depth + 1, Title: node.purpose}
	}
	var oneSide func(string, bool, bool) []model.OutlineDiffRow
	oneSide = func(id string, oldSide, inherited bool) []model.OutlineDiffRow {
		current, other, kind := b, a, "added"
		if oldSide {
			current, other, kind = a, b, "removed"
		}
		node := current.nodes[id]
		counterpart, exists := other.nodes[id]
		// A moved ancestor does not count every unchanged descendant again.
		if inherited && exists && stable[id] && node.parent == counterpart.parent && node.kind == counterpart.kind && node.title == counterpart.title {
			kind = "context"
		}
		rows := []model.OutlineDiffRow{row(node, kind)}
		if exists && node.purpose != counterpart.purpose {
			fieldKind := "added"
			if oldSide {
				fieldKind = "removed"
			}
			rows = append(rows, purpose(node, fieldKind))
		}
		for _, child := range node.children {
			rows = append(rows, oneSide(child, oldSide, true)...)
		}
		return rows
	}
	var pair func(string) []model.OutlineDiffRow
	var siblings func([]string, []string) []model.OutlineDiffRow
	siblings = func(old, next []string) []model.OutlineDiffRow {
		rows := []model.OutlineDiffRow{}
		for _, op := range difflib.NewMatcherWithJunk(old, next, false, nil).GetOpCodes() {
			if op.Tag == 'e' {
				for _, id := range old[op.I1:op.I2] {
					rows = append(rows, pair(id)...)
				}
			} else {
				for _, id := range old[op.I1:op.I2] {
					rows = append(rows, oneSide(id, true, false)...)
				}
				for _, id := range next[op.J1:op.J2] {
					rows = append(rows, oneSide(id, false, false)...)
				}
			}
		}
		return rows
	}
	pair = func(id string) []model.OutlineDiffRow {
		old, next := a.nodes[id], b.nodes[id]
		rows := []model.OutlineDiffRow{}
		if old.title == next.title {
			rows = append(rows, row(next, "context"))
		} else {
			rows = append(rows, row(old, "removed"), row(next, "added"))
		}
		if old.purpose != next.purpose {
			rows = append(rows, purpose(old, "removed"), purpose(next, "added"))
		}
		return append(rows, siblings(old.children, next.children)...)
	}
	groups := []model.OutlineDiffGroup{}
	addGroup := func(rows []model.OutlineDiffRow) {
		for _, row := range rows {
			if row.Kind != "context" {
				groups = append(groups, model.OutlineDiffGroup{Rows: rows})
				return
			}
		}
	}
	for _, op := range difflib.NewMatcherWithJunk(a.roots, b.roots, false, nil).GetOpCodes() {
		if op.Tag == 'e' {
			for _, id := range a.roots[op.I1:op.I2] {
				addGroup(pair(id))
			}
		} else {
			for _, id := range a.roots[op.I1:op.I2] {
				addGroup(oneSide(id, true, false))
			}
			for _, id := range b.roots[op.J1:op.J2] {
				addGroup(oneSide(id, false, false))
			}
		}
	}
	return groups, nil
}
