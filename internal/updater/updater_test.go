package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckForUpdate_dev(t *testing.T) {
	ok, tag, url, err := CheckForUpdate("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false for dev version")
	}
	if tag != "" || url != "" {
		t.Fatal("expected empty tag and url for dev version")
	}
}

func TestCheckForUpdate_noNewVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{TagName: "v1.0.0"})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, _, _, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected false when versions are equal")
	}
}

func TestCheckForUpdate_hasUpdate(t *testing.T) {
	tag := "v2.0.0"
	wanted := fmt.Sprintf("futon_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{
			TagName: tag,
			Assets: []releaseAsset{
				{Name: "other_file", BrowserDownloadURL: "https://example.com/other"},
				{Name: wanted, BrowserDownloadURL: "https://example.com/futon.tar.gz"},
			},
		})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	ok, gotTag, gotURL, err := CheckForUpdate("v1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected true when update available")
	}
	if gotTag != tag {
		t.Fatalf("expected tag %s, got %s", tag, gotTag)
	}
	if gotURL != "https://example.com/futon.tar.gz" {
		t.Fatalf("expected URL https://example.com/futon.tar.gz, got %s", gotURL)
	}
}

func TestCheckForUpdate_noMatchingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(releaseInfo{
			TagName: "v2.0.0",
			Assets:  []releaseAsset{{Name: "wrong_name.tar.gz", BrowserDownloadURL: "https://example.com/wrong"}},
		})
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error when no matching asset")
	}
	if !strings.Contains(err.Error(), "no asset found") {
		t.Fatalf("expected 'no asset found', got: %v", err)
	}
}

func TestCheckForUpdate_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error on non-200 status")
	}
	if !strings.Contains(err.Error(), "HTTP status: 404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}

func TestCheckForUpdate_invalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{invalid json}`))
	}))
	defer srv.Close()

	origURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = origURL }()

	_, _, _, err := CheckForUpdate("v1.0.0")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func makeTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gzw.Close()
	return buf.Bytes()
}

func TestApplyUpdate_non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := ApplyUpdate(srv.URL)
	if err == nil {
		t.Fatal("expected error on non-200 status")
	}
	if !strings.Contains(err.Error(), "HTTP status: 404") {
		t.Fatalf("expected 404 error, got: %v", err)
	}
}

func TestApplyUpdate_badArchive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("this is not a gzip file"))
	}))
	defer srv.Close()

	err := ApplyUpdate(srv.URL)
	if err == nil {
		t.Fatal("expected error on bad archive")
	}
	if !strings.Contains(err.Error(), "failed to create gzip reader") {
		t.Fatalf("expected gzip error, got: %v", err)
	}
}

func TestApplyUpdate_noFutonBinary(t *testing.T) {
	archive := makeTarGz(t, map[string]string{"other_bin": "content"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer srv.Close()

	err := ApplyUpdate(srv.URL)
	if err == nil {
		t.Fatal("expected error when no futon binary in archive")
	}
	if !strings.Contains(err.Error(), "no futon binary found") {
		t.Fatalf("expected 'no futon binary found', got: %v", err)
	}
}

func TestDownloadAndExtract_success(t *testing.T) {
	archive := makeTarGz(t, map[string]string{
		"some_dir/futon": "new version content",
		"other.txt":      "ignore",
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	tarr := tar.NewReader(gzr)
	var found bool
	for {
		hdr, err := tarr.Next()
		if err != nil {
			break
		}
		if filepath.Base(hdr.Name) == "futon" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("futon binary not found in archive")
	}
}
