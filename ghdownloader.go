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
	var client *github.Client
	if token == "" {
		client = github.NewClient(nil)
	} else {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
	}

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

		if err := d.downloadAsset(asset); err != nil {
			// Log and continue with other assets
			fmt.Printf("Warning: failed to download asset '%s' from %s/%s: %v\n", asset.GetName(), owner, repo, err)
		}
	}

	return nil
}

// downloadAsset downloads a single asset and saves it to the destination directory.
// It uses the asset’s API URL to get a redirect URL and then downloads the asset from that URL.
func (d *Downloader) downloadAsset(asset *github.ReleaseAsset) error {
	// Use the API URL for authenticated download
	apiURL := asset.GetURL()
	if apiURL == "" {
		return fmt.Errorf("asset '%s' does not have an API URL", asset.GetName())
	}

	// Create file path for saving the asset
	fileName := asset.GetName()
	filePath := filepath.Join(d.destDir, fileName)

	// Skip download if file already exists
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

	// First request: get the redirect URL from the asset API endpoint
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}
	if d.token != "" {
		req.Header.Set("Authorization", "token "+d.token)
	}
	req.Header.Set("Accept", "application/octet-stream")

	// Use a custom HTTP client to prevent automatic redirects
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Stop following redirects so we can capture the 302 response
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get asset redirect URL: %v", err)
	}
	defer resp.Body.Close()

	// Expect a 302 Found response containing the redirect URL
	if resp.StatusCode != http.StatusFound {
		return fmt.Errorf("unexpected status code (expected 302 Found): got %s", resp.Status)
	}
	redirectURL := resp.Header.Get("Location")
	if redirectURL == "" {
		return fmt.Errorf("no redirect location found for asset '%s'", asset.GetName())
	}

	// Second request: download the asset using the redirect URL
	secondReq, err := http.NewRequest("GET", redirectURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for redirected URL: %v", err)
	}
	// The redirect URL is typically pre-signed; setting Accept is sufficient.
	secondReq.Header.Set("Accept", "application/octet-stream")

	secondResp, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		return fmt.Errorf("failed to download asset from redirect URL: %v", err)
	}
	defer secondResp.Body.Close()

	if secondResp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status downloading asset from redirect URL: %s", secondResp.Status)
	}

	// Write the downloaded content to file
	if _, err = io.Copy(file, secondResp.Body); err != nil {
		return fmt.Errorf("failed to write to file '%s': %v", filePath, err)
	}

	fmt.Printf("Downloaded '%s' to '%s'\n", asset.GetName(), filePath)
	d.mu.Lock()
	d.binPaths = append(d.binPaths, filePath)
	d.mu.Unlock()

	return nil
}
