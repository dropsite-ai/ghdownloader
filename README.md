# ghdownloader

Download latest releases from a GitHub repository.

## Installation

### Go Package

```bash
go get github.com/dropsite-ai/ghdownloader
```

### Homebrew (macOS or Compatible)

If you use Homebrew, install ghdownloader with:
```bash
brew tap dropsite-ai/homebrew-tap
brew install ghdownloader
```

### Download Binaries

Grab the latest pre-built binaries from the [GitHub Releases](https://github.com/dropsite-ai/ghdownloader/releases). Extract them, then run the `ghdownloader` executable directly.

### Build from Source

1. **Clone the repository**:
   ```bash
   git clone https://github.com/dropsite-ai/ghdownloader.git
   cd ghdownloader
   ```
2. **Build using Go**:
   ```bash
   go build -o ghdownloader cmd/main.go
   ```

## Usage

### Command-Line

Download the latest releases from one or more GitHub repositories using:

```bash
ghdownloader -repo owner/repo -repo anotherOwner/anotherRepo \
  -dest "./downloads" -token YOUR_GITHUB_TOKEN -match "linux"
```

- **-repo**: Specify one repository per flag in the format `owner/repo`. This flag can be repeated for multiple repositories.
- **-dest**: Destination directory for downloaded binaries (default: `./downloads`).
- **-token**: Your GitHub Personal Access Token (if omitted, the program uses the `GITHUB_TOKEN` environment variable).
- **-match**: (Optional) A substring filter to only download assets whose names match this filter.

#### How Releases Are Organized

- **Tagged Releases**: For each release with a valid tag (e.g., `v1.2.3`), ghdownloader creates a subdirectory named after that tag under your specified `-dest`. If the file already exists in that subdirectory, it won't be re-downloaded.  
- **Latest (No Tag)**: If the release has no tag, ghdownloader names the subdirectory `latest`. In this scenario, existing files are **always overwritten**—ghdownloader re-downloads them every run.

### Programmatic Usage

You can also integrate the downloader into your Go applications:

```go
package main

import (
    "fmt"
    "log"

    "github.com/dropsite-ai/ghdownloader"
)

func main() {
    token := "YOUR_GITHUB_TOKEN"   // Leave empty for unauthenticated requests.
    destDir := "./downloads"
    repos := []string{"owner/repo", "anotherOwner/anotherRepo"}
    match := "linux"

    // Create a new downloader and set an optional filter.
    downloader := ghdownloader.New(token, destDir)
    downloader.SetMatchFilter(match)
    
    // Download the latest releases.
    binPaths, err := downloader.DownloadLatestReleases(repos)
    if err != nil {
        log.Fatalf("Download error: %v", err)
    }
    
    fmt.Println("Downloaded binaries:")
    for _, path := range binPaths {
        fmt.Println(path)
    }
}
```

## Test

```bash
make test
```

## Release

```bash
make release
```