package main

import (
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func main() {
	flag.Parse()
	loadDotEnv(".env")
	authConfig = loadAuthConfigFromEnv()

	if err := os.MkdirAll(filepath.Join("data", "files"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("data", "text"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("data", "notepad"), 0755); err != nil {
		log.Fatal(err)
	}
	log.Println("Data directory created/reused without errors.")
	createFileIfNotExists("notepad/md.file", mdPlaceholder)
	createFileIfNotExists("links.file", "")

	// Initialize the expiration tracker
	expirationTracker = initExpirationTracker()
	customExpiry := os.Getenv("DEFAULT_EXPIRY")
	if customExpiry != "" {
		applyDefaultExpiry(customExpiry)
	}

	// Goroutine to periodically expire files
	go func() {
		ticker := time.NewTicker(3 * time.Minute) // 3 minutes is sparse enough, load is extremely minimal as the operation is fast (in memory tracker)
		defer ticker.Stop()
		for range ticker.C {
			expirationTracker.CleanupExpired()
		}
	}()

	tmpl = template.Must(template.ParseFS(content, "templates/*.html"))

	var err error
	staticFiles, err = fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}

	registerHandlers()

	handler := withAuth(http.DefaultServeMux)

	// Start server
	log.Fatal(http.ListenAndServe(*listenAddress, handler))
}
