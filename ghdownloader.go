package ghdownloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

// Downloader is responsible for downloading binaries from GitHub releases.
type Downloader struct {
	client      *github.Client
	destDir     string
	token       string
	wg          sync.WaitGroup
	mu          sync.Mutex
	binPaths    []string
	assetsMap   map[string][]*github.ReleaseAsset
	matchFilter string
}

// New creates a new Downloader.
// destDir is the directory where binaries will be saved.
func New(token, destDir string) *Downloader {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	return &Downloader{
		client:    client,
		destDir:   destDir,
		token:     token,
		assetsMap: make(map[string][]*github.ReleaseAsset),
	}
}

// SetMatchFilter sets the match filter for asset names.
func (d *Downloader) SetMatchFilter(match string) {
	d.matchFilter = match
}

// DownloadLatestReleases downloads the latest release binaries for the given user/repos.
// userRepos should be in the format "owner/repo".
func (d *Downloader) DownloadLatestReleases(userRepos []string) ([]string, error) {
	if err := os.MkdirAll(d.destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %v", err)
	}

	errChan := make(chan error, len(userRepos))

	for _, userRepo := range userRepos {
		owner, repo, err := parseUserRepo(userRepo)
		if err != nil {
			return nil, fmt.Errorf("invalid user/repo format '%s': %v", userRepo, err)
		}

		d.wg.Add(1)
		go func(owner, repo string) {
			defer d.wg.Done()
			if err := d.downloadLatestRelease(owner, repo); err != nil {
				errChan <- fmt.Errorf("failed to download %s/%s: %v", owner, repo, err)
			}
		}(owner, repo)
	}

	d.wg.Wait()
	close(errChan)

	// Collect errors
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return d.binPaths, fmt.Errorf("errors occurred:\n%s", strings.Join(errs, "\n"))
	}

	return d.binPaths, nil
}

// parseUserRepo splits "owner/repo" into owner and repo.
func parseUserRepo(userRepo string) (string, string, error) {
	parts := strings.Split(userRepo, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected format 'owner/repo'")
	}
	return parts[0], parts[1], nil
}

// downloadLatestRelease fetches the latest release and downloads its assets.
func (d *Downloader) downloadLatestRelease(owner, repo string) error {
	release, _, err := d.client.Repositories.GetLatestRelease(context.Background(), owner, repo)
	if err != nil {
		return fmt.Errorf("error fetching latest release: %v", err)
	}

	if release.GetDraft() || release.GetPrerelease() {
		// Optionally skip drafts and pre-releases
		return fmt.Errorf("latest release is draft or pre-release")
	}

	assets := release.Assets
	if len(assets) == 0 {
		return fmt.Errorf("no assets found in the latest release")
	}

	// Download assets matching the filter
	for _, asset := range assets {
		if d.matchFilter != "" && !strings.Contains(asset.GetName(), d.matchFilter) {
			fmt.Printf("Skipping asset '%s' (does not match filter '%s')\n", asset.GetName(), d.matchFilter)
			continue
		}

		if err := d.downloadAsset(owner, repo, asset); err != nil {
			// Log and continue with other assets
			fmt.Printf("Warning: failed to download asset '%s' from %s/%s: %v\n", asset.GetName(), owner, repo, err)
		}
	}

	return nil
}

// downloadAsset downloads a single asset and saves it to the destination directory.
func (d *Downloader) downloadAsset(owner, repo string, asset *github.ReleaseAsset) error {
	url := asset.GetBrowserDownloadURL()
	if url == "" {
		return fmt.Errorf("asset '%s' does not have a download URL", asset.GetName())
	}

	// Create a file path: destDir/owner_repo_assetName
	fileName := fmt.Sprintf("%s_%s_%s", owner, repo, asset.GetName())
	filePath := filepath.Join(d.destDir, fileName)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("File '%s' already exists. Skipping download.\n", filePath)
		d.mu.Lock()
		d.binPaths = append(d.binPaths, filePath)
		d.mu.Unlock()
		return nil
	}

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %v", filePath, err)
	}
	defer file.Close()

	// Get the asset
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	// Set authorization header
	req.Header.Set("Authorization", "token "+d.token)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download asset: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading asset: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to file '%s': %v", filePath, err)
	}

	fmt.Printf("Downloaded '%s' to '%s'\n", asset.GetName(), filePath)

	d.mu.Lock()
	d.binPaths = append(d.binPaths, filePath)
	d.mu.Unlock()

	return nil
}
