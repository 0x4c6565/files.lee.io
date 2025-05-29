package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Xiol/tinycache"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	flag "github.com/spf13/pflag"
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

func main() {
	path := flag.String("path", "files", "relative path to the files directory")
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	r := mux.NewRouter()

	cache := tinycache.New[FileInfo](
		tinycache.WithTTL(24*time.Hour),
		tinycache.WithReapInterval(1*time.Hour),
	)
	defer cache.Close()

	handler := NewFileHandler(*path, afero.NewOsFs(), cache)
	// Warm cache with initial scan
	log.Info().Msg("Warming file handler cache...")
	_, err := handler.GetFiles()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to warm file handler cache")
	}

	r.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
	r.HandleFunc("/files.json", handler.Handle).Methods("GET")
	r.PathPrefix("/files/").Handler(http.StripPrefix("/files/", http.FileServer(http.Dir(*path))))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))

	ctx, cancel := context.WithCancel(context.Background())
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-stop
		log.Info().Msg("Caught signal, shutting down..")
		cancel()
	}()
	server := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: r}

	go func() {
		err = server.ListenAndServe()
		cancel()
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
