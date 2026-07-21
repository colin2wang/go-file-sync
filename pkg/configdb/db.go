package configdb

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type ConfigDB struct {
	db *sql.DB
}

type SyncTask struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	SourcePath         string    `json:"source_path"`
	TargetPath         string    `json:"target_path"`
	MonitorInterval    int       `json:"monitor_interval"`
	SyncDirection      string    `json:"sync_direction"`
	Enabled            bool      `json:"enabled"`
	Backup             bool      `json:"backup"`
	Verify             bool      `json:"verify"`
	PreservePerms      bool      `json:"preserve_perms"`
	PreserveOwner      bool      `json:"preserve_owner"`
	PreserveTimes      bool      `json:"preserve_times"`
	Symlinks           string    `json:"symlinks"`
	BandwidthLimit     int64     `json:"bandwidth_limit"`
	ConflictDetection  bool      `json:"conflict_detection"`
	ConflictResolution string    `json:"conflict_resolution"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type SyncLog struct {
	ID        int64     `json:"id"`
	TaskID    int64     `json:"task_id"`
	TaskName  string    `json:"task_name"`
	Action    string    `json:"action"`
	FilePath  string    `json:"file_path"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

func Open(dbPath string) (*ConfigDB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &ConfigDB{db: db}, nil
}

func (c *ConfigDB) Close() error {
	return c.db.Close()
}

func migrate(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sync_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			source_path TEXT NOT NULL,
			target_path TEXT NOT NULL,
			monitor_interval INTEGER DEFAULT 5,
			sync_direction TEXT DEFAULT 'one_way_upload',
			enabled INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sync_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER,
			task_name TEXT,
			action TEXT,
			file_path TEXT,
			status TEXT,
			message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES sync_tasks(id)
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("exec %s: %w", q[:50], err)
		}
	}

	// Add new sync-option columns to existing databases without breaking them.
	addColumn(db, "backup", "INTEGER DEFAULT 0")
	addColumn(db, "verify", "INTEGER DEFAULT 0")
	addColumn(db, "preserve_perms", "INTEGER DEFAULT 0")
	addColumn(db, "preserve_owner", "INTEGER DEFAULT 0")
	addColumn(db, "preserve_times", "INTEGER DEFAULT 0")
	addColumn(db, "symlinks", "TEXT DEFAULT 'follow'")
	addColumn(db, "bandwidth_limit", "INTEGER DEFAULT 0")
	addColumn(db, "conflict_detection", "INTEGER DEFAULT 0")
	addColumn(db, "conflict_resolution", "TEXT DEFAULT 'warn'")

	return nil
}

// addColumn adds a column if it does not already exist (idempotent migration).
func addColumn(db *sql.DB, name, colType string) {
	_, err := db.Exec(fmt.Sprintf("ALTER TABLE sync_tasks ADD COLUMN %s %s", name, colType))
	if err != nil {
		// SQLite errors with "duplicate column" when the column already exists.
		if strings.Contains(err.Error(), "duplicate column") {
			return
		}
		log.Printf("migrate: add column %s failed: %v", name, err)
	}
}

func (c *ConfigDB) CreateTask(t *SyncTask) (int64, error) {
	result, err := c.db.Exec(
		`INSERT INTO sync_tasks
			(name, source_path, target_path, monitor_interval, sync_direction, enabled,
			 backup, verify, preserve_perms, preserve_owner, preserve_times, symlinks,
			 bandwidth_limit, conflict_detection, conflict_resolution)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.SourcePath, t.TargetPath, t.MonitorInterval, t.SyncDirection, t.Enabled,
		t.Backup, t.Verify, t.PreservePerms, t.PreserveOwner, t.PreserveTimes, t.Symlinks,
		t.BandwidthLimit, t.ConflictDetection, t.ConflictResolution,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (c *ConfigDB) GetTask(id int64) (*SyncTask, error) {
	t := &SyncTask{}
	err := c.db.QueryRow(
		`SELECT id, name, source_path, target_path, monitor_interval, sync_direction, enabled,
			backup, verify, preserve_perms, preserve_owner, preserve_times, symlinks,
			bandwidth_limit, conflict_detection, conflict_resolution, created_at, updated_at
		 FROM sync_tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.SourcePath, &t.TargetPath, &t.MonitorInterval, &t.SyncDirection, &t.Enabled,
		&t.Backup, &t.Verify, &t.PreservePerms, &t.PreserveOwner, &t.PreserveTimes, &t.Symlinks,
		&t.BandwidthLimit, &t.ConflictDetection, &t.ConflictResolution, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (c *ConfigDB) ListTasks() ([]SyncTask, error) {
	rows, err := c.db.Query(
		`SELECT id, name, source_path, target_path, monitor_interval, sync_direction, enabled,
			backup, verify, preserve_perms, preserve_owner, preserve_times, symlinks,
			bandwidth_limit, conflict_detection, conflict_resolution, created_at, updated_at
		 FROM sync_tasks ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []SyncTask
	for rows.Next() {
		var t SyncTask
		if err := rows.Scan(&t.ID, &t.Name, &t.SourcePath, &t.TargetPath, &t.MonitorInterval, &t.SyncDirection, &t.Enabled,
			&t.Backup, &t.Verify, &t.PreservePerms, &t.PreserveOwner, &t.PreserveTimes, &t.Symlinks,
			&t.BandwidthLimit, &t.ConflictDetection, &t.ConflictResolution, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (c *ConfigDB) UpdateTask(id int64, t *SyncTask) error {
	_, err := c.db.Exec(
		`UPDATE sync_tasks SET
			name=?, source_path=?, target_path=?, monitor_interval=?, sync_direction=?, enabled=?,
			backup=?, verify=?, preserve_perms=?, preserve_owner=?, preserve_times=?, symlinks=?,
			bandwidth_limit=?, conflict_detection=?, conflict_resolution=?, updated_at=CURRENT_TIMESTAMP
		 WHERE id=?`,
		t.Name, t.SourcePath, t.TargetPath, t.MonitorInterval, t.SyncDirection, t.Enabled,
		t.Backup, t.Verify, t.PreservePerms, t.PreserveOwner, t.PreserveTimes, t.Symlinks,
		t.BandwidthLimit, t.ConflictDetection, t.ConflictResolution, id,
	)
	return err
}

func (c *ConfigDB) DeleteTask(id int64) error {
	_, err := c.db.Exec(`DELETE FROM sync_tasks WHERE id = ?`, id)
	return err
}

func (c *ConfigDB) LogSync(taskID int64, taskName, action, filePath, status, message string) error {
	_, err := c.db.Exec(
		`INSERT INTO sync_logs (task_id, task_name, action, file_path, status, message) VALUES (?, ?, ?, ?, ?, ?)`,
		taskID, taskName, action, filePath, status, message,
	)
	return err
}

func (c *ConfigDB) GetLogs(limit int) ([]SyncLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := c.db.Query(
		`SELECT l.id, l.task_id, l.task_name, l.action, l.file_path, l.status, l.message, l.created_at
		 FROM sync_logs l ORDER BY l.id DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []SyncLog
	for rows.Next() {
		var l SyncLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.TaskName, &l.Action, &l.FilePath, &l.Status, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (c *ConfigDB) GetLogsByTask(taskID int64, limit int) ([]SyncLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := c.db.Query(
		`SELECT l.id, l.task_id, l.task_name, l.action, l.file_path, l.status, l.message, l.created_at
		 FROM sync_logs l WHERE l.task_id = ? ORDER BY l.id DESC LIMIT ?`, taskID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []SyncLog
	for rows.Next() {
		var l SyncLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.TaskName, &l.Action, &l.FilePath, &l.Status, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
