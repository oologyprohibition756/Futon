package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
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


