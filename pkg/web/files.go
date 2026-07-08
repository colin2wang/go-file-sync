package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

func RegisterFileRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/files", handleFileList)
	mux.HandleFunc("/api/files/read", handleFileRead)
	mux.HandleFunc("/api/drives", handleDrives)
}

func handleDrives(w http.ResponseWriter, r *http.Request) {
	if runtime.GOOS != "windows" {
		writeJSONResp(w, []string{"/"})
		return
	}

	var drives []string
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		drive := string(letter) + ":\\"
		if _, err := os.Stat(drive); err == nil {
			drives = append(drives, drive)
		}
	}
	if len(drives) == 0 {
		drives = []string{"C:\\"}
	}
	writeJSONResp(w, drives)
}

func handleFileList(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		if runtime.GOOS == "windows" {
			dirPath = "C:\\"
		} else {
			dirPath = "/"
		}
	}

	// Normalize path
	dirPath = filepath.FromSlash(dirPath)

	info, err := os.Stat(dirPath)
	if err != nil {
		writeJSONResp(w, map[string]string{"error": err.Error()})
		return
	}
	if !info.IsDir() {
		writeJSONResp(w, map[string]string{"error": "not a directory"})
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		writeJSONResp(w, map[string]string{"error": err.Error()})
		return
	}

	var files []FileInfo

	// Add parent directory link
	parent := filepath.Dir(dirPath)
	if parent != dirPath {
		files = append(files, FileInfo{
			Name:  "..",
			Path:  parent,
			IsDir: true,
		})
	}

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:  entry.Name(),
			Path:  filepath.Join(dirPath, entry.Name()),
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	writeJSONResp(w, files)
}

func handleFileRead(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSONResp(w, map[string]string{"error": "path required"})
		return
	}

	filePath = filepath.FromSlash(filePath)

	info, err := os.Stat(filePath)
	if err != nil {
		writeJSONResp(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSONResp(w, FileInfo{
		Name:  info.Name(),
		Path:  filePath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
	})
}

func writeJSONResp(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
