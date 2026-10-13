// Package branding owns the single human-uploaded image outside authoring files.
package branding

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dasi0227/PPT-Agent/backend/internal/artifactfs"
	"github.com/dasi0227/PPT-Agent/backend/internal/attachment"
	"github.com/dasi0227/PPT-Agent/backend/internal/decoration"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func Path(logo *decoration.LogoMetadata) (string, error) {
	if logo == nil || (logo.Extension != "png" && logo.Extension != "jpg") {
		return "", fmt.Errorf("invalid project logo metadata")
	}
	return "branding/logo." + logo.Extension, nil
}

var ErrNotFound = errors.New("project logo not found")

func logoSandbox(workDir string) (*artifactfs.Sandbox, error) {
	sandbox, err := artifactfs.NewSandbox(model.ProjectRoot(workDir))
	if err != nil {
		return nil, err
	}
	dir, err := sandbox.Resolve("branding")
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
		return nil, fmt.Errorf("project branding must be a real directory")
	}
	return sandbox, nil
}

func Hash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func Verify(ctx context.Context, originalName string, source io.Reader) (*decoration.LogoMetadata, []byte, error) {
	image, err := attachment.Verify(ctx, source)
	if err != nil {
		return nil, nil, err
	}
	name := filepath.Base(originalName)
	return &decoration.LogoMetadata{Hash: Hash(image.Data), OriginalName: name, MediaType: image.MediaType,
		Extension: image.Extension, SizeBytes: int64(len(image.Data)), Width: image.Width, Height: image.Height}, image.Data, nil
}

func Read(workDir string, logo *decoration.LogoMetadata) ([]byte, error) {
	if logo == nil {
		return nil, ErrNotFound
	}
	sandbox, err := logoSandbox(workDir)
	if err != nil {
		return nil, err
	}
	rel, err := Path(logo)
	if err != nil {
		return nil, err
	}
	path, err := sandbox.Resolve(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != logo.SizeBytes || info.Size() > attachment.MaxFileBytes {
		return nil, fmt.Errorf("project logo file is invalid")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if Hash(raw) != logo.Hash {
		return nil, fmt.Errorf("project logo hash mismatch")
	}
	return raw, nil
}

func Write(workDir string, logo *decoration.LogoMetadata, raw []byte) error {
	rel, err := Path(logo)
	if err != nil {
		return err
	}
	if Hash(raw) != logo.Hash || int64(len(raw)) != logo.SizeBytes {
		return fmt.Errorf("project logo content does not match metadata")
	}
	sandbox, err := logoSandbox(workDir)
	if err != nil {
		return err
	}
	return sandbox.Write(rel, raw)
}

func Delete(workDir string, logo *decoration.LogoMetadata) error {
	rel, err := Path(logo)
	if err != nil {
		return err
	}
	sandbox, err := logoSandbox(workDir)
	if err != nil {
		return err
	}
	err = sandbox.Delete(rel)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	dir, err := sandbox.Resolve("branding")
	if err != nil {
		return err
	}
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func PreviewImage(projectID string, logo *decoration.LogoMetadata) *decoration.Image {
	if logo == nil {
		return nil
	}
	return &decoration.Image{Src: "/api/v1/projects/" + projectID + "/logo/content?v=" + logo.Hash, Hash: logo.Hash}
}

func RenderImage(workDir string, logo *decoration.LogoMetadata) (*decoration.Image, error) {
	if logo == nil {
		return nil, nil
	}
	raw, err := Read(workDir, logo)
	if err != nil {
		return nil, err
	}
	return &decoration.Image{Src: "data:" + logo.MediaType + ";base64," + base64.StdEncoding.EncodeToString(raw), Hash: logo.Hash}, nil
}
