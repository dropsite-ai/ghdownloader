package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dropsite-ai/ghdownloader"
)

// repoList implements flag.Value to allow multiple -repo flags.
type repoList []string

func (r *repoList) String() string {
	return strings.Join(*r, ",")
}

func (r *repoList) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func main() {
	token := flag.String("token", "", "GitHub Personal Access Token. Defaults to GITHUB_TOKEN environment variable if not provided.")
	destDir := flag.String("dest", "./downloads", "Destination directory for downloaded binaries")
	var repos repoList
	flag.Var(&repos, "repo", "Repository in 'owner/repo' format. Can be specified multiple times. (Required)")
	match := flag.String("match", "", "Substring to filter assets by name (optional)")

	flag.Parse()

	// Use GITHUB_TOKEN environment variable if token flag is empty.
	if *token == "" {
		*token = os.Getenv("GITHUB_TOKEN")
	}

	// Warn if no token is provided.
	if *token == "" {
		fmt.Println("Warning: No GitHub token provided. Proceeding with unauthenticated requests (rate limits apply).")
	}

	// Validate that at least one repository is provided.
	if len(repos) == 0 {
		fmt.Println("Error: At least one repository is required.")
		flag.Usage()
		os.Exit(1)
	}

	// Create a new downloader.
	downloader := ghdownloader.New(*token, *destDir)
	downloader.SetMatchFilter(*match)

	// Download the latest releases.
	fmt.Println("Starting download...")
	binPaths, err := downloader.DownloadLatestReleases(repos)
	if err != nil {
		log.Fatalf("Error downloading releases: %v\n", err)
	}

	fmt.Println("Download completed successfully.")
	fmt.Println("Downloaded binaries:")
	for _, path := range binPaths {
		fmt.Println(path)
	}
}
