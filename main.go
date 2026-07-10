package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"go-file-sync/pkg/configdb"
	"go-file-sync/pkg/syncmanager"
	"go-file-sync/pkg/web"
)

func main() {
	// Get executable directory for database
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(filepath.Dir(execPath), "go-file-sync.db")

	// Open database
	db, err := configdb.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create sync manager
	syncMgr := syncmanager.NewManager(db)
	syncMgr.Start()
	defer syncMgr.Stop()

	// Create API handler
	api := web.NewAPI(db)
	api.SetTaskUpdateFunc(func(taskID int64, enabled bool) {
		syncMgr.TaskUpdated(taskID, enabled)
	})
	api.SetSyncManager(syncMgr)

	// Create HTTP mux
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	web.RegisterFileRoutes(mux)

	// Serve static files (Vue3 frontend)
	staticFS, err := web.StaticFiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load static files: %v\n", err)
		os.Exit(1)
	}
	fs := http.FileServer(http.FS(staticFS))
	mux.Handle("/", fs)

	// Start server
	listen := ":8080"
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			listen = ":" + os.Args[i+1]
		}
	}

	fmt.Printf("Starting go-file-sync on http://localhost%s\n", listen)

	go func() {
		if err := http.ListenAndServe(listen, mux); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	// Open browser
	openBrowser("http://localhost" + listen)

	fmt.Println("Press Ctrl+C to stop")

	// Wait for shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "msedge", url)
	case "darwin":
		cmd = exec.Command("open", "-a", "Microsoft Edge", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
		fmt.Printf("Please open %s manually\n", url)
	}
}
