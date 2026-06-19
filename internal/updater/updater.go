package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner = "KabosuNeko"
	repoName  = "Futon"
)

var apiURL = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"

type releaseInfo struct {
	TagName string      `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func CheckForUpdate(currentVersion string) (bool, string, string, error) {
	if currentVersion == "dev" {
		return false, "", "", nil
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(apiURL)
	if err != nil {
		return false, "", "", fmt.Errorf("failed to check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", "", fmt.Errorf("check update failed, HTTP status: %d", resp.StatusCode)
	}

	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return false, "", "", fmt.Errorf("failed to parse release info: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	if latest <= current {
		return false, "", "", nil
	}

	wanted := fmt.Sprintf("futon_%s_%s_%s.tar.gz", rel.TagName, runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range rel.Assets {
		if a.Name == wanted {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return false, "", "", fmt.Errorf("no asset found for %s", wanted)
	}

	return true, rel.TagName, downloadURL, nil
}

func ApplyUpdate(downloadURL string) error {
	tmpDir := os.TempDir()

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	absPath, err := filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download update, HTTP status: %d", resp.StatusCode)
	}

	tmpArc, err := os.CreateTemp(tmpDir, "futon_update_*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp archive file: %w", err)
	}
	defer os.Remove(tmpArc.Name())

	if _, err := io.Copy(tmpArc, resp.Body); err != nil {
		tmpArc.Close()
		return fmt.Errorf("failed to save archive: %w", err)
	}
	tmpArc.Close()

	f, err := os.Open(tmpArc.Name())
	if err != nil {
		return fmt.Errorf("failed to open saved archive: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tarr := tar.NewReader(gzr)

	var extracted bool
	for {
		header, err := tarr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar archive: %w", err)
		}

		if filepath.Base(header.Name) != "futon" {
			continue
		}

		tmpFile := filepath.Join(tmpDir, "futon_update")
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}

		if _, err := io.Copy(f, tarr); err != nil {
			f.Close()
			os.Remove(tmpFile)
			return fmt.Errorf("failed to extract binary: %w", err)
		}
		f.Close()

		oldPath := absPath + ".old"
		if err := os.Rename(absPath, oldPath); err != nil {
			os.Remove(tmpFile)
			return fmt.Errorf("failed to backup current binary: %w", err)
		}

		if err := os.Rename(tmpFile, absPath); err != nil {
			os.Rename(oldPath, absPath)
			os.Remove(tmpFile)
			return fmt.Errorf("failed to replace binary: %w", err)
		}

		if err := os.Chmod(absPath, 0755); err != nil {
			return fmt.Errorf("failed to set permissions: %w", err)
		}

		os.Remove(oldPath)
		extracted = true
		break
	}

	if !extracted {
		return fmt.Errorf("no futon binary found in archive")
	}

	return nil
}
