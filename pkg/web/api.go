package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"go-file-sync/pkg/configdb"
	"go-file-sync/pkg/syncmanager"
)

type TaskUpdateFunc func(taskID int64, enabled bool)

type API struct {
	db           *configdb.ConfigDB
	onTaskUpdate TaskUpdateFunc
	syncMgr      *syncmanager.Manager
}

func NewAPI(db *configdb.ConfigDB) *API {
	return &API{db: db}
}

func (a *API) SetTaskUpdateFunc(fn TaskUpdateFunc) {
	a.onTaskUpdate = fn
}

func (a *API) SetSyncManager(mgr *syncmanager.Manager) {
	a.syncMgr = mgr
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/tasks", a.handleTasks)
	mux.HandleFunc("/api/tasks/", a.handleTaskByID)
	mux.HandleFunc("/api/logs", a.handleLogs)
	mux.HandleFunc("/api/logs/", a.handleLogsByTask)
	mux.HandleFunc("/api/stats", a.handleStats)
	mux.HandleFunc("/api/sync-stats", a.handleSyncStats)
}

func (a *API) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listTasks(w, r)
	case http.MethodPost:
		a.createTask(w, r)
	default:
		writeErrorJSON(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := a.db.ListTasks()
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, tasks)
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	var t configdb.SyncTask
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeErrorJSON(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if t.Name == "" || t.SourcePath == "" || t.TargetPath == "" {
		writeErrorJSON(w, "name, source_path, target_path required", http.StatusBadRequest)
		return
	}
	if t.MonitorInterval <= 0 {
		t.MonitorInterval = 5
	}
	if t.SyncDirection == "" {
		t.SyncDirection = "one_way_upload"
	}
	t.Enabled = true

	id, err := a.db.CreateTask(&t)
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.ID = id
	if a.onTaskUpdate != nil {
		a.onTaskUpdate(id, t.Enabled)
	}
	writeJSON(w, t)
}

func (a *API) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErrorJSON(w, "invalid id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := a.db.GetTask(id)
		if err != nil {
			writeErrorJSON(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, task)

	case http.MethodPut:
		var t configdb.SyncTask
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeErrorJSON(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := a.db.UpdateTask(id, &t); err != nil {
			writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if a.onTaskUpdate != nil {
			a.onTaskUpdate(id, t.Enabled)
		}
		writeJSON(w, map[string]string{"status": "updated"})

	case http.MethodDelete:
		if err := a.db.DeleteTask(id); err != nil {
			writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if a.onTaskUpdate != nil {
			a.onTaskUpdate(id, false)
		}
		writeJSON(w, map[string]string{"status": "deleted"})

	default:
		writeErrorJSON(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	logs, err := a.db.GetLogs(limit)
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, logs)
}

func (a *API) handleLogsByTask(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	taskID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeErrorJSON(w, "invalid task id", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	logs, err := a.db.GetLogsByTask(taskID, limit)
	if err != nil {
		writeErrorJSON(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, logs)
}

func (a *API) handleStats(w http.ResponseWriter, r *http.Request) {
	tasks, _ := a.db.ListTasks()
	logs, _ := a.db.GetLogs(1000)

	enabledCount := 0
	for _, t := range tasks {
		if t.Enabled {
			enabledCount++
		}
	}

	successCount := 0
	failCount := 0
	for _, l := range logs {
		if l.Status == "synced" || l.Status == "success" {
			successCount++
		} else if l.Status == "failed" {
			failCount++
		}
	}

	writeJSON(w, map[string]interface{}{
		"total_tasks":   len(tasks),
		"enabled_tasks": enabledCount,
		"total_logs":    len(logs),
		"success_count": successCount,
		"fail_count":    failCount,
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErrorJSON(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (a *API) handleSyncStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErrorJSON(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if a.syncMgr == nil {
		writeJSON(w, map[string]interface{}{
			"monitored_files": 0,
			"synced_files":    0,
		})
		return
	}

	stats := a.syncMgr.GetStats()
	writeJSON(w, stats)
}
