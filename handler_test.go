package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Xiol/tinycache"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

// Helper to create temp dir with files
func setupTestDir(t *testing.T, fs afero.Fs) string {

	dir, err := afero.TempDir(fs, "", "testdir")
	if err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := fs.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, filepath.Join(dir, "file1.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(fs, filepath.Join(subdir, "file2.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGetFiles_CacheMiss(t *testing.T) {
	fs := afero.NewMemMapFs()
	dir := setupTestDir(t, fs)
	cache := tinycache.New[FileInfo]()
	handler := NewFileHandler(dir, fs, cache)

	// Cache miss: should scan and populate cache
	files, err := handler.GetFiles()

	assert.NoError(t, err, "unexpected error")
	assert.Equal(t, files.Name, dir, "unexpected root FileInfo name")
	assert.Equal(t, files.Type, TYPE_FOLDER, "unexpected root FileInfo type")
	assert.Len(t, files.Items, 2, "expected 2 items in root directory")
}

func TestGetFiles_ScanError(t *testing.T) {
	cache := tinycache.New[FileInfo]()
	handler := NewFileHandler("/nonexistent/path", afero.NewMemMapFs(), cache)
	_, err := handler.GetFiles()
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestHandle_Success(t *testing.T) {
	fs := afero.NewMemMapFs()
	dir := setupTestDir(t, fs)
	cache := tinycache.New[FileInfo]()
	handler := NewFileHandler(dir, fs, cache)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "expected 200 OK")
	var fi FileInfo
	err := json.NewDecoder(resp.Body).Decode(&fi)
	assert.NoError(t, err, "unexpected error decoding response")

	assert.Equal(t, dir, fi.Name, "unexpected root FileInfo name")
	assert.Equal(t, TYPE_FOLDER, fi.Type, "unexpected root FileInfo type")

}

func TestHandle_ScanError(t *testing.T) {
	cache := tinycache.New[FileInfo]()
	handler := NewFileHandler("/nonexistent/path", afero.NewMemMapFs(), cache)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}
