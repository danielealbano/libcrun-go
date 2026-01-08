//go:build linux && cgo

package main

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// ImageConfig holds the extracted configuration from an OCI image.
type ImageConfig struct {
	Entrypoint []string
	Cmd        []string
	Env        []string
	WorkingDir string
	User       string
}

// PulledImage represents a pulled and extracted image.
type PulledImage struct {
	RootFS string      // Path to extracted rootfs
	Config ImageConfig // Image configuration
}

// formatBytes formats bytes into human-readable format.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// PullAndExtract pulls an OCI image and extracts it to a temporary directory.
// The caller is responsible for cleaning up the returned rootfs path.
func PullAndExtract(imageRef string) (*PulledImage, error) {
	// Parse the image reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("invalid image reference %q: %w", imageRef, err)
	}

	fmt.Printf("Pulling image: %s\n", ref.Name())

	// Pull the image using default keychain (reads ~/.docker/config.json)
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Get image config
	configFile, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get image config: %w", err)
	}

	config := ImageConfig{
		Entrypoint: configFile.Config.Entrypoint,
		Cmd:        configFile.Config.Cmd,
		Env:        configFile.Config.Env,
		WorkingDir: configFile.Config.WorkingDir,
		User:       configFile.Config.User,
	}

	// Create temporary directory for rootfs
	rootfs, err := os.MkdirTemp("", "crungo-rootfs-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract layers with progress
	fmt.Printf("Extracting to: %s\n", rootfs)
	if err := extractImage(img, rootfs); err != nil {
		os.RemoveAll(rootfs)
		return nil, fmt.Errorf("failed to extract image: %w", err)
	}

	// Create minimal /etc/passwd if it doesn't exist (required by libcrun)
	if err := ensurePasswd(rootfs); err != nil {
		os.RemoveAll(rootfs)
		return nil, fmt.Errorf("failed to create /etc/passwd: %w", err)
	}

	fmt.Println("Done!")
	return &PulledImage{
		RootFS: rootfs,
		Config: config,
	}, nil
}

// extractImage extracts all layers of an image to the target directory.
func extractImage(img v1.Image, targetDir string) error {
	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get layers: %w", err)
	}

	totalLayers := len(layers)
	fmt.Printf("Downloading and extracting %d layers:\n", totalLayers)

	for i, layer := range layers {
		layerNum := i + 1

		// Get layer size for progress
		size, _ := layer.Size()

		fmt.Printf("  [%d/%d] Downloading %s... ", layerNum, totalLayers, formatBytes(size))

		if err := extractLayerWithProgress(layer, targetDir, layerNum, totalLayers); err != nil {
			fmt.Println("✗")
			return fmt.Errorf("failed to extract layer %d: %w", layerNum, err)
		}
	}

	return nil
}

// extractLayerWithProgress extracts a single layer with progress indication.
func extractLayerWithProgress(layer v1.Layer, targetDir string, layerNum, totalLayers int) error {
	reader, err := layer.Uncompressed()
	if err != nil {
		return fmt.Errorf("failed to get uncompressed layer: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)

	fileCount := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		fileCount++

		// Handle whiteout files (deletions in overlay filesystem)
		baseName := filepath.Base(header.Name)
		if strings.HasPrefix(baseName, ".wh.") {
			// This is a whiteout marker - delete the corresponding file
			targetName := strings.TrimPrefix(baseName, ".wh.")
			targetPath := filepath.Join(targetDir, filepath.Dir(header.Name), targetName)
			os.RemoveAll(targetPath)
			continue
		}

		// Clean the path to prevent path traversal
		cleanPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanPath, "..") {
			continue // Skip paths that try to escape
		}

		targetPath := filepath.Join(targetDir, cleanPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
			}

			// Remove existing file if it exists (layers can overwrite)
			os.Remove(targetPath)

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", targetPath, err)
			}

			if _, err := io.Copy(file, tr); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			file.Close()

		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for symlink %s: %w", targetPath, err)
			}

			// Remove existing file/symlink if it exists
			os.Remove(targetPath)

			if err := os.Symlink(header.Linkname, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink %s -> %s: %w", targetPath, header.Linkname, err)
			}

		case tar.TypeLink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for hardlink %s: %w", targetPath, err)
			}

			// Remove existing file if it exists
			os.Remove(targetPath)

			linkTarget := filepath.Join(targetDir, header.Linkname)
			if err := os.Link(linkTarget, targetPath); err != nil {
				// If hard link fails, try copying the file
				if copyErr := copyFile(linkTarget, targetPath); copyErr != nil {
					return fmt.Errorf("failed to create hardlink %s -> %s: %w (copy also failed: %v)", targetPath, linkTarget, err, copyErr)
				}
			}

		case tar.TypeChar, tar.TypeBlock:
			// Skip device nodes - we can't create them without root and they're rarely needed
			continue

		case tar.TypeFifo:
			// Skip FIFOs
			continue
		}
	}

	fmt.Printf("extracted %d files ✓\n", fileCount)
	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ensurePasswd creates a minimal /etc/passwd file if it doesn't exist.
// This is required by libcrun to detect the HOME environment variable.
func ensurePasswd(rootfs string) error {
	etcDir := filepath.Join(rootfs, "etc")
	passwdPath := filepath.Join(etcDir, "passwd")

	// Check if passwd already exists
	if _, err := os.Stat(passwdPath); err == nil {
		return nil // Already exists
	}

	// Create /etc directory if it doesn't exist
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		return err
	}

	// Create minimal passwd file
	content := "root:x:0:0:root:/root:/bin/sh\nnobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin\n"
	return os.WriteFile(passwdPath, []byte(content), 0644)
}

// ParseImageRef normalizes an image reference, adding default registry and tag if needed.
func ParseImageRef(ref string) (string, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return "", err
	}
	return parsed.Name(), nil
}
