package downloader

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipanalytics/rir-sql-forge/internal/sources"
)

func TestDownloadStreamsToTempFile(t *testing.T) {
	client := Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("inetnum: 203.0.113.0 - 203.0.113.255\n\n")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}
	got, err := client.Download(context.Background(), t.TempDir(), sources.Source{RIR: "TEST", URL: "https://example.net/test.db.gz", Gzip: true})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(got.LocalPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) == 0 || filepath.Base(got.LocalPath) != "test.db.gz" {
		t.Fatalf("unexpected download path/body: %s %q", got.LocalPath, body)
	}
}

func TestDownloadCleansPartialOnHTTPError(t *testing.T) {
	client := Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString("nope")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}
	dir := t.TempDir()
	if _, err := client.Download(context.Background(), dir, sources.Source{RIR: "TEST", URL: "https://example.net/bad.db"}); err == nil {
		t.Fatal("expected HTTP error")
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("partial files left behind: %v", matches)
	}
}

func TestDownloadCleansPartialOnBodyError(t *testing.T) {
	client := Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errorReader{}),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}}
	dir := t.TempDir()
	if _, err := client.Download(context.Background(), dir, sources.Source{RIR: "TEST", URL: "https://example.net/bad-body.db"}); err == nil {
		t.Fatal("expected body read error")
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("partial files left behind: %v", matches)
	}
}

func TestDownloadHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := Client{HTTPClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			return nil, err
		}
		t.Fatal("expected canceled request context")
		return nil, nil
	})}}
	if _, err := client.Download(ctx, t.TempDir(), sources.Source{RIR: "TEST", URL: "https://example.net/canceled.db"}); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}
