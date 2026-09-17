package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var tmpl *template.Template
var staticFiles fs.FS

func registerHandlers() {
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/md", handleMarkdown)
	http.HandleFunc("/getExpiryOptions", handleGetExpiryOptions)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	http.HandleFunc("/style.css", serveStaticAsset("style.css", "text/css", "Style not found"))
	http.HandleFunc("/manifest.json", serveStaticAsset("manifest.json", "application/json", "Manifest not found"))
	http.HandleFunc("/sw.js", serveStaticAsset("sw.js", "application/javascript", "Service worker not found"))
	http.HandleFunc("/md.js", serveStaticAsset("md.js", "application/javascript", "JavaScript not found"))
	http.HandleFunc("/favicon.ico", serveStaticAsset("favicon.ico", "image/x-icon", "Favicon not found"))
	http.HandleFunc("/icon-192.png", serveStaticAsset("icon-192.png", "image/png", "Icon not found"))
	http.HandleFunc("/icon-512.png", serveStaticAsset("icon-512.png", "image/png", "Icon not found"))

	http.HandleFunc("/notepad/", handleNotepad)
	http.HandleFunc("/submit", handleSubmit)
	http.HandleFunc("/rename/", handleRename)
	http.HandleFunc("/raw/", handleRaw)
	http.HandleFunc("/download/", handleDownload)
	http.HandleFunc("/view/", handleView)
	http.HandleFunc("/delete/", handleDelete)
	http.HandleFunc("/edit/", handleEdit)

	// SSE Updates for content refresh
	http.HandleFunc("/api/updates", handleContentUpdates)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	// Clean up expired files on page load
	expirationTracker.CleanupExpired()
	entries := []Entry{}
	// Read text snippets
	textFiles, _ := os.ReadDir(filepath.Join("data", "text"))
	for _, file := range textFiles {
		if file.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join("data", "text", file.Name()))
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			ID:       filepath.Join("text", file.Name()),
			Type:     "text",
			Content:  string(data),
			Filename: file.Name(),
		})
	}
	// Read files
	files, _ := os.ReadDir(filepath.Join("data", "files"))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		entries = append(entries, Entry{
			ID:       filepath.Join("files", file.Name()),
			Type:     "file",
			Filename: file.Name(),
		})
	}
	// Read links
	data, err := os.ReadFile(filepath.Join("data", "links.file"))
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			entries = append(entries, Entry{
				ID:       "link/" + url.PathEscape(line),
				Type:     "link",
				Content:  line,
				Filename: line,
			})
		}
	}
	tmpl.ExecuteTemplate(w, "index.html", entries)
}

func handleMarkdown(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "md.html", nil)
}

func handleGetExpiryOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expirationOptions)
}

func serveStaticAsset(name, contentType, notFoundMessage string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFiles.Open(name)
		if err != nil {
			http.Error(w, notFoundMessage, http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", contentType)
		io.Copy(w, file)
	}
}

func handleNotepad(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		filename := strings.TrimPrefix(r.URL.Path, "/notepad/")
		if filename != "md.file" { // && filename != "rtext.file" {
			http.Error(w, "Invalid notepad file", http.StatusBadRequest)
			return
		}
		content, err := os.ReadFile(filepath.Join("data", "notepad", filename))
		if err != nil {
			http.Error(w, "Error reading notepad file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(content)
		return
	case "POST":
		filename := strings.TrimPrefix(r.URL.Path, "/notepad/")
		if filename != "md.file" { // && filename != "rtext.file" {
			http.Error(w, "Invalid notepad file", http.StatusBadRequest)
			return
		}
		content, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		err = os.WriteFile(filepath.Join("data", "notepad", filename), content, 0644)
		if err != nil {
			http.Error(w, "Error saving notepad file", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Saved"))
		log.Printf("Saved notepad content to %s\n", filename)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entryType := r.FormValue("type")
	expiryOption := r.FormValue("expiry")
	content := r.FormValue("content")
	name := r.FormValue("name")
	if entryType == "link" {
		// Handle link submission
		if content == "" {
			http.Error(w, "URL content cannot be empty", http.StatusBadRequest)
			return
		}
		u, err := url.ParseRequestURI(content)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "Invalid URL format. Must start with http:// or https://", http.StatusBadRequest)
			return
		}
		linksFilePath := filepath.Join("data", "links.file")
		f, err := os.OpenFile(linksFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		if _, err := f.WriteString(content + "\n"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Saved link %s\n", content)
	} else {
		// Handle file and text submission
		files := r.MultipartForm.File["file-upload"]
		if len(files) > 0 {
			// File submission
			for _, fileHeader := range files {
				err := func() error {
					file, err := fileHeader.Open()
					if err != nil {
						return err
					}
					defer file.Close()
					fileName := name
					if fileName == "" {
						fileName = fileHeader.Filename
					}
					uniqueFileName := generateUniqueFilename("data/files", fileName)
					f, err := os.Create(filepath.Join("data/files", uniqueFileName))
					if err != nil {
						return err
					}
					defer f.Close()
					if _, err := io.Copy(f, file); err != nil {
						return err
					}
					if expiryOption != "Never" {
						fileID := filepath.Join("files", uniqueFileName)
						expirationTracker.SetExpiration(fileID, expiryOption)
					}
					log.Printf("Saved file %s with expiry %s\n", uniqueFileName, expiryOption)
					return nil
				}()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else if content != "" {
			// Text snippet submission
			filename := name
			if filename == "" {
				filename = time.Now().Format("Jan-02 15-04-05")
			}
			uniqueFileName := generateUniqueFilename("data/text", filename)
			err := os.WriteFile(filepath.Join("data/text", uniqueFileName), []byte(content), 0644)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if expiryOption != "Never" {
				fileID := filepath.Join("text", uniqueFileName)
				expirationTracker.SetExpiration(fileID, expiryOption)
			}
			log.Printf("Saved text snippet %s with expiry %s\n", uniqueFileName, expiryOption)
		}
	}
	notifyContentChange()
	// Send succes for AJAX
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	oldPath := strings.TrimPrefix(r.URL.Path, "/rename/")
	newName := r.FormValue("newname")
	if newName == "" {
		http.Error(w, "New name cannot be empty", http.StatusBadRequest)
		return
	}
	baseDir := filepath.Dir(filepath.Join("data", oldPath))
	newName = generateUniqueFilename(baseDir, newName)

	// Get the new full path
	newPath := filepath.Join(baseDir, newName)
	oldFullPath := filepath.Join("data", oldPath)
	// Check if there's an expiration for this file
	expirationTracker.mu.Lock()
	expiryTime, hasExpiry := expirationTracker.Expirations[oldPath]
	if hasExpiry {
		// Remove old entry and add new one
		delete(expirationTracker.Expirations, oldPath)
		relNewPath := strings.TrimPrefix(newPath, "data/")
		relNewPath = strings.ReplaceAll(relNewPath, "\\", "/") // Ensure cross-platform path separators
		expirationTracker.Expirations[relNewPath] = expiryTime
		expirationTracker.saveToFile()
	}
	expirationTracker.mu.Unlock()
	// Rename the file
	err := os.Rename(oldFullPath, newPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	notifyContentChange()
	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.Printf("Renamed %s to %s\n", oldPath, newName)
}

func handleRaw(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/raw/")
	if !strings.HasPrefix(id, "text/") {
		http.Error(w, "Only text files can be accessed", http.StatusBadRequest)
		return
	}
	content, err := os.ReadFile(filepath.Join("data", id))
	if err != nil {
		http.Error(w, "File not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(content)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	filePath := filepath.Join("data", filename)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Brute force method to determine content type
	ext := strings.ToLower(filepath.Ext(filename))
	var contentType string
	switch ext {
	case ".pdf":
		contentType = "application/pdf"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".svg":
		contentType = "image/svg+xml"
	default:
		buffer := make([]byte, 512)
		_, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		contentType = http.DetectContentType(buffer)
		_, err = file.Seek(0, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	baseFilename := filepath.Base(filename)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", baseFilename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = io.Copy(w, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Served %s for download\n", filename)
}

func handleView(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/view/")
	http.ServeFile(w, r, filepath.Join("data", filename))
	log.Printf("Served %s for viewing\n", filename)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/delete/")
	// Handle link deletion
	if after, ok := strings.CutPrefix(id, "link/"); ok {
		linkToDelete := after
		linksFilePath := filepath.Join("data", "links.file")
		data, err := os.ReadFile(linksFilePath)
		if err != nil {
			http.Error(w, "Failed to read links file for deletion", http.StatusInternalServerError)
			return
		}
		lines := strings.Split(string(data), "\n")
		var newLines []string
		var found bool
		for _, line := range lines {
			if strings.TrimSpace(line) == strings.TrimSpace(linkToDelete) && !found {
				found = true // Remove only the first occurrence
				continue
			}
			if strings.TrimSpace(line) != "" {
				newLines = append(newLines, line)
			}
		}
		output := strings.Join(newLines, "\n")
		// Add newline for correctness
		if output != "" {
			output += "\n"
		}
		err = os.WriteFile(linksFilePath, []byte(output), 0644)
		if err != nil {
			http.Error(w, "Failed to write links file after deletion", http.StatusInternalServerError)
			return
		}
		notifyContentChange()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
		log.Printf("Deleted link %s\n", linkToDelete)
		return
	}
	// Handle file and snippet deletion
	err := os.Remove(filepath.Join("data", id))
	if err != nil {
		log.Printf("Failed to delete %s: %v", id, err)
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}
	expirationTracker.mu.Lock()
	delete(expirationTracker.Expirations, id)
	expirationTracker.saveToFile()
	expirationTracker.mu.Unlock()
	notifyContentChange()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
	log.Printf("Deleted %s\n", id)
}

func handleEdit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/edit/")
	if !strings.HasPrefix(id, "text/") {
		http.Error(w, "Can only edit text snippets", http.StatusBadRequest)
		return
	}
	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Content cannot be empty", http.StatusBadRequest)
		return
	}
	err := os.WriteFile(filepath.Join("data", id), []byte(content), 0644)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	notifyContentChange()
	http.Redirect(w, r, "/", http.StatusSeeOther)
	log.Printf("Edited %s\n", id)
}
