package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dropsite-ai/ghdownloader"
)

func main() {
	// Define flags
	token := flag.String("token", "", "GitHub Personal Access Token. Defaults to GITHUB_TOKEN environment variable if not provided.")
	destDir := flag.String("dest", "./downloads", "Destination directory for downloaded binaries")
	repos := flag.String("repos", "", "Comma-separated list of GitHub repositories in 'owner/repo' format (required)")
	match := flag.String("match", "", "Substring to filter assets by name (optional)")

	flag.Parse()

	// Use GITHUB_TOKEN environment variable if token flag is empty
	if *token == "" {
		*token = os.Getenv("GITHUB_TOKEN")
	}

	// (Optional) Warn the user if no token is provided, but do not exit.
	if *token == "" {
		fmt.Println("Warning: No GitHub token provided. Proceeding with unauthenticated requests (rate limits apply).")
	}

	if *repos == "" {
		fmt.Println("Error: At least one repository is required.")
		flag.Usage()
		os.Exit(1)
	}

	// Parse the repositories
	userRepos := strings.Split(*repos, ",")

	// Create a new downloader
	downloader := ghdownloader.New(*token, *destDir)

	// Set the match filter in the downloader
	downloader.SetMatchFilter(*match)

	// Download the latest releases
	fmt.Println("Starting download...")
	binPaths, err := downloader.DownloadLatestReleases(userRepos)
	if err != nil {
		log.Fatalf("Error downloading releases: %v\n", err)
	}

	fmt.Println("Download completed successfully.")
	fmt.Println("Downloaded binaries:")
	for _, path := range binPaths {
		fmt.Println(path)
	}
}
