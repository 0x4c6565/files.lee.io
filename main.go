package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Xiol/tinycache"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

var (
	TYPE_FILE   = "file"
	TYPE_FOLDER = "folder"
)

type FileInfo struct {
	Name  string     `json:"name"`
	Type  string     `json:"type"`
	Path  string     `json:"path"`
	Size  int64      `json:"size"`
	Items []FileInfo `json:"items,omitempty"`
}

var cache = tinycache.New[FileInfo](
	tinycache.WithTTL(24*time.Hour),
	tinycache.WithReapInterval(1*time.Hour),
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

func handler(w http.ResponseWriter, r *http.Request) {
	var root FileInfo
	root, ok := cache.Get("files")
	if !ok {
		log.Debug().Msg("Cache miss, scanning directory")
		dir := "files"

		items, err := scan(dir)
		if err != nil {
			http.Error(w, "Error scanning directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

		root = FileInfo{
			Name:  "files",
			Type:  TYPE_FOLDER,
			Path:  "files",
			Items: items,
		}

		cache.Set("files", root)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(root); err != nil {
		http.Error(w, "Failed to encode JSON: "+err.Error(), http.StatusInternalServerError)
	}
}

func main() {
	defer cache.Close()
	r := mux.NewRouter()

	r.HandleFunc("/files.json", handler).Methods("GET")
	r.PathPrefix("/files/").Handler(http.StripPrefix("/files/", http.FileServer(http.Dir("./files"))))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		log.Info().Msg("Caught signal, shutting down..")
		cancel()
	}()
	server := &http.Server{Addr: ":8080", Handler: r}

	var err error
	go func() {
		err = server.ListenAndServe()
	}()

	log.Info().Msg("Server started")

	<-ctx.Done()
	log.Debug().Msg("Server shutting down..")
	server.Shutdown(context.Background())
	log.Debug().Msg("Server shut down complete")

	if err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server failed")
	}
}
