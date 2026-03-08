// Command wayzn-server runs a web server for controlling your Wayzn Smart Pet Door.
//
// Usage:
//
//	wayzn-server -config wayzn.json -addr :8080
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/k15z/axiom/wayzn/pkg/wayzn"
)

//go:embed templates
var templateFS embed.FS

var (
	flagAddr   = flag.String("addr", ":8080", "HTTP listen address")
	flagConfig = flag.String("config", "wayzn.json", "Path to Wayzn config JSON file")
)

type server struct {
	client    *wayzn.Client
	mu        sync.Mutex
	lastState wayzn.DoorState
	lastErr   string
	tmpl      *template.Template
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := loadConfig(*flagConfig)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	client := wayzn.NewClient(cfg)
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	s := &server{
		client:    client,
		lastState: wayzn.DoorUnknown,
		tmpl:      tmpl,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/open", s.handleOpen)
	mux.HandleFunc("/api/close", s.handleClose)
	mux.HandleFunc("/api/status", s.handleStatus)

	srv := &http.Server{
		Addr:         *flagAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		log.Printf("Wayzn web server listening on %s", *flagAddr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func loadConfig(path string) (wayzn.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return wayzn.Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var cfg wayzn.Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return wayzn.Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return cfg, nil
}

type pageData struct {
	State   wayzn.DoorState
	Error   string
	Message string
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	data := pageData{
		State: s.lastState,
		Error: s.lastErr,
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Template error: %v", err)
	}
}

func (s *server) handleOpen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	err := s.client.Open(ctx)

	s.mu.Lock()
	if err != nil {
		s.lastErr = err.Error()
	} else {
		s.lastState = wayzn.DoorOpen
		s.lastErr = ""
	}
	s.mu.Unlock()

	respondJSON(w, err)
}

func (s *server) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	err := s.client.Close(ctx)

	s.mu.Lock()
	if err != nil {
		s.lastErr = err.Error()
	} else {
		s.lastState = wayzn.DoorClosed
		s.lastErr = ""
	}
	s.mu.Unlock()

	respondJSON(w, err)
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status, err := s.client.GetStatus(ctx)

	s.mu.Lock()
	if err != nil {
		s.lastErr = err.Error()
	} else {
		s.lastState = status.State
		s.lastErr = ""
	}
	s.mu.Unlock()

	if err != nil {
		respondJSON(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func respondJSON(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
