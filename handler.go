package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

type Cacher[T any] interface {
	Delete(key string)
	Set(key string, value T)
	SetTTL(key string, value T, ttl time.Duration)
	SetPermanent(key string, value T)
	Get(key string) (T, bool)
	Reap()
	Close()
}

type Handler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}

type FileHandler struct {
	dir   string
	cache Cacher[FileInfo]
	fs    afero.Fs
}

func NewFileHandler(dir string, fs afero.Fs, cache Cacher[FileInfo]) *FileHandler {
	return &FileHandler{
		dir:   dir,
		cache: cache,
		fs:    fs,
	}
}

func (h *FileHandler) scan(dir string) ([]FileInfo, error) {
	var files []FileInfo

	entries, err := afero.ReadDir(h.fs, dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Skip hidden files and folders
		if entry.Name()[0] == '.' {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			subItems, err := h.scan(fullPath)
			if err != nil {
				return nil, err
			}
			files = append(files, FileInfo{
				Name:  entry.Name(),
				Type:  TYPE_FOLDER,
				Path:  fullPath,
				Items: subItems,
			})
		} else {
			files = append(files, FileInfo{
				Name: entry.Name(),
				Type: TYPE_FILE,
				Path: fullPath,
				Size: entry.Size(),
			})
		}
	}

	return files, nil
}

func (h *FileHandler) GetFiles() (FileInfo, error) {
	files, ok := h.cache.Get("files")
	if !ok {
		log.Debug().Msg("Cache miss, scanning directory")

		items, err := h.scan(h.dir)
		if err != nil {
			return FileInfo{}, fmt.Errorf("error scanning directory: %w", err)
		}

		files = FileInfo{
			Name:  h.dir,
			Type:  TYPE_FOLDER,
			Path:  h.dir,
			Items: items,
		}

		h.cache.Set("files", files)
	}

	return files, nil
}

func (h *FileHandler) Handle(w http.ResponseWriter, r *http.Request) {
	files, err := h.GetFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
