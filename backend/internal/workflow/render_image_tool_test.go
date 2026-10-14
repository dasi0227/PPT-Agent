package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

func TestReadImageAllowsOnlyLatestImageOfAnExistingPage(t *testing.T) {
	root, css, pack := generationPackFixture(t)
	pack.Project.Decorations = decoration.DefaultPositions()
	outlineSource, err := json.Marshal(pack.Outline.Outline)
	if err != nil {
		t.Fatal(err)
	}
	writeGenerationFile(t, root, "image.svg", []byte("original asset"))
	proof, err := currentRenderProof(pack, root, nil, generationSlide, hashBytes([]byte(generationHTML)))
	if err != nil {
		t.Fatal(err)
	}
	proof.ResourceHash, err = renderimage.ResourceHash(root, map[string]string{"image.svg": ""})
	if err != nil {
		t.Fatal(err)
	}
	write := func(shot string) renderimage.Entry {
		t.Helper()
		entry := renderimage.Entry{ProjectID: pack.Project.ID, SlideID: generationSlide, RunID: "previous_run", ScreenshotID: shot,
			SourceHash: proof.ArtifactHash, DependencyHash: proof.DependencyHash(), ResourceHashes: map[string]string{"image.svg": hashBytes([]byte("original asset"))}}
		var data bytes.Buffer
		if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, entry.ImagePath())
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := renderimage.Publish(root, entry); err != nil {
			t.Fatal(err)
		}
		return entry
	}
	old, latest := write("shot_old"), write("shot_latest")
	read := func(args map[string]any) ToolResult {
		return (readImageTool{}).Execute(context.Background(), DomainToolInput{RunID: "new_run", ProjectDir: root, Context: pack, Args: args})
	}
	readPage := func() ToolResult { return read(map[string]any{"slide_id": generationSlide}) }
	result := readPage()
	if !result.OK || result.Observation != "" || len(result.ObservationParts) != 1 || result.ObservationParts[0].ImageRef != latest.ImageRef() {
		t.Fatalf("latest valid screenshot did not return its image block: %+v", result)
	}
	images, err := latestRenderedImages(pack, root, nil)
	if err != nil || len(images) != 1 || images[0].Stale || images[0].ImagePath != latest.ImagePath() {
		t.Fatalf("current screenshot context disagrees with image read: %+v %v", images, err)
	}
	for _, source := range []string{".design.json", ".manifest.json", model.SlideSpecPath(generationSlide)} {
		path := filepath.Join(root, source)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeGenerationFile(t, root, source, []byte(`{"broken":`))
		if result := readPage(); !result.OK {
			t.Fatalf("pixel-independent %s corruption blocked screenshot: %+v", source, result)
		}
		if images, err := latestRenderedImages(pack, root, nil); err != nil || len(images) != 1 || images[0].Stale {
			t.Fatalf("pixel-independent %s corruption changed freshness: %+v %v", source, images, err)
		}
		if _, err := currentRenderProof(pack, root, nil, generationSlide, proof.ArtifactHash); err == nil {
			t.Fatalf("full authoring evidence ignored corrupt %s", source)
		}
		writeGenerationFile(t, root, source, before)
	}
	for _, dependency := range []struct {
		path string
		next []byte
	}{
		{model.SlideHTMLPath(generationSlide), []byte(generationHTML + "<!-- changed -->")},
		{".outline.json", []byte(strings.Replace(string(outlineSource), `"title":"Section"`, `"title":"New section"`, 1))},
		{"image.svg", []byte("changed asset")},
		{filepath.Join("..", "..", "..", "assets", "themes", "clean", "theme.css"), []byte(css + "\nh1 { color: red }")},
	} {
		path := filepath.Join(root, dependency.path)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, dependency.next, 0o600); err != nil {
			t.Fatal(err)
		}
		if result := readPage(); result.Code != "IMAGE_INVALID" {
			t.Fatalf("changed pixel dependency %s accepted: %+v", dependency.path, result)
		}
		if images, err := latestRenderedImages(pack, root, nil); err != nil || len(images) != 1 || !images[0].Stale {
			t.Fatalf("changed dependency not marked stale: %+v %v", images, err)
		}
		if err := os.WriteFile(path, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	pageNumberPosition := pack.Project.Decorations.PageNumber
	pack.Project.Decorations.PageNumber = "bottom-left"
	if result := readPage(); result.Code != "IMAGE_INVALID" {
		t.Fatalf("changed decorations did not invalidate screenshot: %+v", result)
	}
	pack.Project.Decorations.PageNumber = pageNumberPosition
	imagePath := filepath.Join(root, latest.ImagePath())
	pixels, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{nil, []byte("not a PNG"), pixels[:len(pixels)/2], []byte(strings.Repeat("x", (10<<20)+1))} {
		if err := os.WriteFile(imagePath, invalid, 0o600); err != nil {
			t.Fatal(err)
		}
		if result := readPage(); result.Code != "IMAGE_INVALID" {
			t.Fatalf("invalid screenshot file accepted or treated as missing: %+v", result)
		}
	}
	if err := os.Remove(imagePath); err != nil {
		t.Fatal(err)
	}
	if result := readPage(); result.Code != model.CodeNotFound {
		t.Fatalf("missing screenshot file misclassified: %+v", result)
	}
	if err := os.Remove(filepath.Join(root, ".outline.json")); err != nil {
		t.Fatal(err)
	}
	if result := readPage(); result.Code != model.CodeOutlineNotExist {
		t.Fatalf("missing outline was masked by the missing screenshot: %+v", result)
	}
	writeGenerationFile(t, root, ".outline.json", outlineSource)
	if err := os.WriteFile(imagePath, pixels, 0o600); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(root, ".runtime", "render-index", generationSlide+".json")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(indexPath, []byte("broken index"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := readPage(); result.Code != model.CodeInternal {
		t.Fatalf("broken Runtime index treated as an image defect: %+v", result)
	}
	if err := os.WriteFile(indexPath, index, 0o600); err != nil {
		t.Fatal(err)
	}
	writeGenerationFile(t, root, ".outline.json", []byte(`{"sections":`))
	if result := readPage(); result.Code != "IMAGE_INVALID" {
		t.Fatalf("unreadable page locations leaked a source error: %+v", result)
	}
	writeGenerationFile(t, root, ".outline.json", []byte(strings.Replace(string(outlineSource), `"purpose":"Explain"`, `"purpose":17`, 1)))
	if result := readPage(); !result.OK {
		t.Fatalf("unused outline authoring fields blocked pixel access: %+v", result)
	}
	writeGenerationFile(t, root, ".outline.json", outlineSource)
	if result := renderedImageFailure(&fs.PathError{Op: "read", Path: imagePath, Err: fs.ErrPermission}, generationSlide); result.Code != model.CodeInternal {
		t.Fatalf("screenshot I/O error treated as image invalidity: %+v", result)
	}
	for _, args := range []map[string]any{
		{"image_path": old.ImagePath()}, {"image_path": "../../secret.png"},
		{"image_path": latest.ImagePath(), "attachment_id": "att_one"},
		{"image_path": latest.ImagePath(), "variant": "original"},
	} {
		if result := read(args); result.Code != model.CodeToolArgumentInvalid {
			t.Fatalf("unauthorized or ambiguous read accepted: %+v", args)
		}
	}
	pack.Outline.Outline.Sections = []spec.Section{}
	outline, err := json.Marshal(pack.Outline.Outline)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".outline.json"), outline, 0o600); err != nil {
		t.Fatal(err)
	}
	if result := readPage(); result.Code != model.CodeNotFound {
		t.Fatalf("deleted page image accepted: %+v", result)
	}
	if images, err := latestRenderedImages(pack, root, nil); err != nil || len(images) != 0 {
		t.Fatalf("deleted page in runtime index: %+v", images)
	}
}
