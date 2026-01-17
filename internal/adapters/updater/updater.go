package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"xtools/internal/version"
)

// UpdateInfo contains information about available updates
type UpdateInfo struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion"`
	IsUpdateAvailable bool   `json:"isUpdateAvailable"`
	ReleaseURL        string `json:"releaseUrl"`
	ReleaseNotes      string `json:"releaseNotes"`
	PublishedAt       string `json:"publishedAt"`
}

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

// Updater checks for application updates from GitHub releases
type Updater struct {
	owner      string
	repo       string
	httpClient *http.Client
}

// NewUpdater creates a new Updater instance
func NewUpdater() *Updater {
	return &Updater{
		owner: version.GitHubOwner,
		repo:  version.GitHubRepo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckForUpdates checks GitHub for the latest release
func (u *Updater) CheckForUpdates(ctx context.Context) (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", u.owner, u.repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "XTools-Updater")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases yet
		return &UpdateInfo{
			CurrentVersion:    version.Version,
			LatestVersion:     version.Version,
			IsUpdateAvailable: false,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Skip draft and prerelease versions
	if release.Draft || release.Prerelease {
		return &UpdateInfo{
			CurrentVersion:    version.Version,
			LatestVersion:     version.Version,
			IsUpdateAvailable: false,
		}, nil
	}

	latestVersion := normalizeVersion(release.TagName)
	currentVersion := normalizeVersion(version.Version)

	return &UpdateInfo{
		CurrentVersion:    version.Version,
		LatestVersion:     latestVersion,
		IsUpdateAvailable: compareVersions(latestVersion, currentVersion) > 0,
		ReleaseURL:        release.HTMLURL,
		ReleaseNotes:      release.Body,
		PublishedAt:       release.PublishedAt,
	}, nil
}

// GetCurrentVersion returns the current app version
func (u *Updater) GetCurrentVersion() string {
	return version.Version
}

// normalizeVersion removes 'v' prefix and '-release' suffix from version strings
// Handles formats like: v1.0.2-release, v1.0.0, 1.0.0-release
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSuffix(v, "-release")
	return v
}

// compareVersions compares two semantic versions
// Returns: 1 if a > b, -1 if a < b, 0 if equal
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	// Pad shorter version with zeros
	for len(aParts) < 3 {
		aParts = append(aParts, "0")
	}
	for len(bParts) < 3 {
		bParts = append(bParts, "0")
	}

	for i := 0; i < 3; i++ {
		aNum := parseVersionPart(aParts[i])
		bNum := parseVersionPart(bParts[i])

		if aNum > bNum {
			return 1
		}
		if aNum < bNum {
			return -1
		}
	}

	return 0
}

// parseVersionPart converts a version part string to int
func parseVersionPart(s string) int {
	// Remove any non-numeric suffix (e.g., "1-beta" -> "1")
	for i, c := range s {
		if c < '0' || c > '9' {
			s = s[:i]
			break
		}
	}

	var num int
	fmt.Sscanf(s, "%d", &num)
	return num
}
