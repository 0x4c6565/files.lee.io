package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Xiol/tinycache"
	"github.com/rs/zerolog/log"
)

func scan(dir string) ([]FileInfo, error) {
	var files []FileInfo

	entries, err := os.ReadDir(dir)
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
			subItems, err := scan(fullPath)
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
			info, err := entry.Info()
			if err != nil {
				return nil, err
			}
			files = append(files, FileInfo{
				Name: entry.Name(),
				Type: TYPE_FILE,
				Path: fullPath,
				Size: info.Size(),
			})
		}
	}

	return files, nil
}

type Handler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}

type FileHandler struct {
	dir   string
	cache *tinycache.Cache[FileInfo]
}

func NewFileHandler(dir string, cache *tinycache.Cache[FileInfo]) *FileHandler {
	return &FileHandler{
		dir:   dir,
		cache: cache,
	}
}

func (h *FileHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var root FileInfo
	root, ok := h.cache.Get("files")
	if !ok {
		log.Debug().Msg("Cache miss, scanning directory")

		items, err := scan(h.dir)
		if err != nil {
			http.Error(w, "Error scanning directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		root = FileInfo{
			Name:  h.dir,
			Type:  TYPE_FOLDER,
			Path:  h.dir,
			Items: items,
		}

		h.cache.Set("files", root)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(root); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}
