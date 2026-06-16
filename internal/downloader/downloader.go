package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ipanalytics/rir-sql-forge/internal/sources"
)

// Client downloads source files using an HTTP client.
type Client struct {
	HTTPClient *http.Client
}

// Download streams src.URL into tempDir and returns a source with LocalPath set.
func (c Client) Download(ctx context.Context, tempDir string, src sources.Source) (sources.Source, error) {
	if src.URL == "" {
		return src, nil
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return src, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return src, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Minute}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return src, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return src, fmt.Errorf("download %s: HTTP %d", src.URL, resp.StatusCode)
	}
	name := filepath.Base(req.URL.Path)
	if name == "." || name == "/" || name == "" {
		name = src.RIR + ".db"
	}
	path := filepath.Join(tempDir, name)
	tmp := path + ".part"
	file, err := os.Create(tmp)
	if err != nil {
		return src, err
	}
	_, copyErr := io.Copy(file, resp.Body)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return src, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return src, closeErr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return src, err
	}
	src.LocalPath = path
	return src, nil
}
