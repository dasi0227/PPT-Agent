package service

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type repositoryFileMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type repositoryFrontmatterStyle struct {
	open  string
	close string
}

var (
	cssFrontmatterStyle  = repositoryFrontmatterStyle{open: "/*\n", close: "*/"}
	htmlFrontmatterStyle = repositoryFrontmatterStyle{open: "<!--\n", close: "-->"}
	mdFrontmatterStyle   = repositoryFrontmatterStyle{}
)

func parseRepositoryFrontmatter(raw []byte, style repositoryFrontmatterStyle) (repositoryFileMetadata, string, error) {
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	start := style.open + "---\n"
	if !strings.HasPrefix(text, start) {
		return repositoryFileMetadata{}, "", fmt.Errorf("%w: repository frontmatter is required", ErrRepositoryCorrupt)
	}
	endMarker := "\n---\n" + style.close
	end := strings.Index(text[len(start):], endMarker)
	if end < 0 {
		return repositoryFileMetadata{}, "", fmt.Errorf("%w: repository frontmatter is not terminated", ErrRepositoryCorrupt)
	}
	end += len(start)

	decoder := yaml.NewDecoder(strings.NewReader(text[len(start):end]))
	decoder.KnownFields(true)
	var metadata repositoryFileMetadata
	if err := decoder.Decode(&metadata); err != nil {
		return repositoryFileMetadata{}, "", fmt.Errorf("%w: %v", ErrRepositoryCorrupt, err)
	}
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Description = strings.TrimSpace(metadata.Description)
	if metadata.Name == "" || metadata.Description == "" {
		return repositoryFileMetadata{}, "", fmt.Errorf("%w: repository name and description are required", ErrRepositoryCorrupt)
	}

	body := text[end+len(endMarker):]
	body = strings.TrimPrefix(body, "\n")
	return metadata, body, nil
}

func renderRepositoryFrontmatter(metadata repositoryFileMetadata, body string, style repositoryFrontmatterStyle) ([]byte, error) {
	raw, err := yaml.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(style.open)
	out.WriteString("---\n")
	out.Write(raw)
	out.WriteString("---\n")
	out.WriteString(style.close)
	if style.close != "" {
		out.WriteByte('\n')
	}
	out.WriteString(body)
	return out.Bytes(), nil
}

func replaceRepositoryFileMetadata(
	path string,
	original []byte,
	body string,
	style repositoryFrontmatterStyle,
	metadata repositoryFileMetadata,
	persistTags func() error,
) error {
	next, err := renderRepositoryFrontmatter(metadata, body, style)
	if err != nil {
		return err
	}
	if len(next) > maxRepositoryFileSize {
		return ErrRepositoryFileTooLarge
	}
	if err := atomicRewriteRepositoryFile(path, next); err != nil {
		return err
	}
	if err := persistTags(); err != nil {
		if rollbackErr := atomicRewriteRepositoryFile(path, original); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore repository file: %w", rollbackErr))
		}
		return err
	}
	return nil
}

func atomicRewriteRepositoryFile(path string, raw []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafeRepositoryPath
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".repository-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err = tmp.Chmod(info.Mode().Perm()); err == nil {
		_, err = tmp.Write(raw)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
