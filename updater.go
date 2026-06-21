package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const updateExitCode = 42

// CheckUpdate checks GitHub for a newer release and downloads it.
// Returns the path to the new binary, or "" if no update available.
func CheckUpdate(owner, repo, currentVersion string) (string, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return "", nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if compareVersions(latestVersion, currentVersion) <= 0 {
		slog.Info("up to date", "version", currentVersion)
		return "", nil
	}

	// Find the .exe asset
	var downloadURL string
	targetName := "kfsSms.exe"
	if runtime.GOOS != "windows" {
		targetName = "kfsSms"
	}
	for _, asset := range release.Assets {
		if asset.Name == targetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no matching asset found for %s", targetName)
	}

	// Download
	slog.Info("downloading update", "version", latestVersion, "url", downloadURL)
	newPath := "kfsSms-new.exe"
	if runtime.GOOS != "windows" {
		newPath = "kfsSms-new"
	}

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer dlResp.Body.Close()

	f, err := os.Create(newPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, dlResp.Body); err != nil {
		os.Remove(newPath)
		return "", fmt.Errorf("write file: %w", err)
	}

	slog.Info("update downloaded", "path", newPath, "version", latestVersion)
	return newPath, nil
}

// compareVersions compares two semantic version strings (X.Y.Z).
// Returns: 1 if a > b, -1 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	ap := splitVersion(a)
	bp := splitVersion(b)

	maxLen := len(ap)
	if len(bp) > maxLen {
		maxLen = len(bp)
	}

	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func splitVersion(v string) []int {
	parts := strings.Split(v, ".")
	result := make([]int, len(parts))
	for i, p := range parts {
		n, _ := strconv.Atoi(p)
		result[i] = n
	}
	return result
}
