package db

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/devakxhay/lighthouse/models"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

// ---- Setup ----

func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

//go:embed migrations/*.sql
var migrationFS embed.FS

func (d *DB) migrate() error {
	migrations := []string{
		"migrations/0001_init.sql",
		"migrations/0002_unique_domain.sql",
		"migrations/0003_git_url.sql",
		"migrations/0004_unique_port.sql",
	}
	for _, m := range migrations {
		schema, err := migrationFS.ReadFile(m)
		if err != nil {
			return err
		}
		if _, err = d.conn.Exec(string(schema)); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("run migration %s: %w", m, err)
		}
	}
	return nil
}

// ---- Apps ----

func (d *DB) CreateApp(a *models.App) error {
	res, err := d.conn.Exec(`
		INSERT INTO apps (name, type, domain, port, binary_path, app_dir, status, git_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.Type, a.Domain, a.Port, a.BinaryPath, a.AppDir, models.StatusPending, a.GitURL,
	)
	if err != nil {
		return err
	}
	a.ID, _ = res.LastInsertId()
	return nil
}

func (d *DB) GetApp(name string) (*models.App, error) {
	a := &models.App{}
	err := d.conn.QueryRow(`
		SELECT id, name, type, domain, port, binary_path, app_dir, status, created_at, git_url
		FROM apps WHERE name = ?`, name).
		Scan(&a.ID, &a.Name, &a.Type, &a.Domain, &a.Port,
			&a.BinaryPath, &a.AppDir, &a.Status, &a.CreatedAt, &a.GitURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (d *DB) ListApps() ([]models.App, error) {
	rows, err := d.conn.Query(`
		SELECT id, name, type, domain, port, binary_path, app_dir, status, created_at, git_url
		FROM apps ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []models.App
	for rows.Next() {
		a := models.App{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Domain, &a.Port,
			&a.BinaryPath, &a.AppDir, &a.Status, &a.CreatedAt, &a.GitURL); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, nil
}

func (d *DB) UpdateAppStatus(name string, status models.AppStatus) error {
	_, err := d.conn.Exec(`UPDATE apps SET status = ? WHERE name = ?`, status, name)
	return err
}

func (d *DB) UpdateAppPaths(name string, appDir string, binaryPath string) error {
	_, err := d.conn.Exec(`UPDATE apps SET app_dir = ?, binary_path = ? WHERE name = ?`, appDir, binaryPath, name)
	return err
}

func (d *DB) DeleteApp(name string) error {
	_, err := d.conn.Exec(`DELETE FROM apps WHERE name = ?`, name)
	return err
}

// ---- Certs ----

func (d *DB) SaveCert(c *models.Cert) error {
	// upsert: delete old, insert new
	d.conn.Exec(`DELETE FROM certs WHERE app_id = ?`, c.AppID)
	_, err := d.conn.Exec(`
		INSERT INTO certs (app_id, domain, issued_at, expires_at, cert_path, key_path)
		VALUES (?, ?, ?, ?, ?, ?)`,
		c.AppID, c.Domain, c.IssuedAt, c.ExpiresAt, c.CertPath, c.KeyPath,
	)
	return err
}

func (d *DB) GetCert(appID int64) (*models.Cert, error) {
	c := &models.Cert{}
	err := d.conn.QueryRow(`
		SELECT id, app_id, domain, issued_at, expires_at, cert_path, key_path
		FROM certs WHERE app_id = ?`, appID).
		Scan(&c.ID, &c.AppID, &c.Domain, &c.IssuedAt, &c.ExpiresAt, &c.CertPath, &c.KeyPath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// ---- Audit ----

func (d *DB) Log(entry models.AuditLog) {
	d.conn.Exec(`
		INSERT INTO audit_log (app_name, action, status, triggered_by, details)
		VALUES (?, ?, ?, ?, ?)`,
		entry.AppName, entry.Action, entry.Status, entry.TriggeredBy, entry.Details,
	)
}

func (d *DB) GetAuditLogs(appName string, limit int) ([]models.AuditLog, error) {
	q := `SELECT id, timestamp, app_name, action, status, triggered_by, details
		  FROM audit_log WHERE app_name = ? ORDER BY timestamp DESC LIMIT ?`
	rows, err := d.conn.Query(q, appName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		l := models.AuditLog{}
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.AppName, &l.Action,
			&l.Status, &l.TriggeredBy, &l.Details); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}
