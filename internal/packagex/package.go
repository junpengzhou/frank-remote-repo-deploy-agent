package packagex

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Artifact struct {
	Path      string
	Packaging string
}

func FindArtifact(moduleDir, packaging string) (Artifact, error) {
	packaging = strings.ToLower(packaging)
	if packaging != "war" && packaging != "jar" {
		return Artifact{}, fmt.Errorf("unsupported packaging %q", packaging)
	}
	matches, err := filepath.Glob(filepath.Join(moduleDir, "target", "*."+packaging))
	if err != nil {
		return Artifact{}, err
	}
	if len(matches) == 0 {
		return Artifact{}, fmt.Errorf("no %s artifact found under %s", packaging, filepath.Join(moduleDir, "target"))
	}
	sort.Slice(matches, func(i, j int) bool {
		left, _ := os.Stat(matches[i])
		right, _ := os.Stat(matches[j])
		if left == nil || right == nil {
			return matches[i] > matches[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	return Artifact{Path: matches[0], Packaging: packaging}, nil
}

func PrepareStaging(artifact Artifact, stagingRoot, module string) (string, error) {
	target := filepath.Join(stagingRoot, module)
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}
	switch artifact.Packaging {
	case "war":
		if err := unzip(artifact.Path, target); err != nil {
			return "", err
		}
	case "jar":
		if err := copyFile(artifact.Path, filepath.Join(target, filepath.Base(artifact.Path))); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported packaging %q", artifact.Packaging)
	}
	return target, nil
}

func unzip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target := filepath.Join(dest, file.Name)
		cleanDest, err := filepath.Abs(dest)
		if err != nil {
			return err
		}
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) && cleanTarget != cleanDest {
			return fmt.Errorf("illegal path in archive: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
