package models

import "time"

type AppType string

const (
	AppTypeSpringBoot AppType = "spring-boot"
	AppTypeNextJS     AppType = "nextjs"
	AppTypeGo         AppType = "go"
)

type AppStatus string

const (
	StatusRunning AppStatus = "running"
	StatusStopped AppStatus = "stopped"
	StatusFailed  AppStatus = "failed"
	StatusPending AppStatus = "pending"
)

type App struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Type       AppType   `json:"type"`
	Domain     string    `json:"domain"`
	Port       int       `json:"port"`
	BinaryPath string    `json:"binary_path"`
	AppDir     string    `json:"app_dir"`
	Status     AppStatus `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	GitURL     string    `json:"git_url"`
	EntryPoint string    `json:"entry_point"`
}

type Cert struct {
	ID        int64     `json:"id"`
	AppID     int64     `json:"app_id"`
	Domain    string    `json:"domain"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
	CertPath  string    `json:"cert_path"`
	KeyPath   string    `json:"key_path"`
}

type AuditAction string

const (
	ActionDeploy      AuditAction = "DEPLOY"
	ActionStart       AuditAction = "START"
	ActionStop        AuditAction = "STOP"
	ActionRestart     AuditAction = "RESTART"
	ActionCertGen     AuditAction = "CERT_GEN"
	ActionEnvChange   AuditAction = "ENV_CHANGE"
	ActionNginxReload AuditAction = "NGINX_RELOAD"
	ActionRollback    AuditAction = "ROLLBACK"
)

type AuditLog struct {
	ID          int64       `json:"id"`
	Timestamp   time.Time   `json:"timestamp"`
	AppName     string      `json:"app_name"`
	Action      AuditAction `json:"action"`
	Status      string      `json:"status"` // SUCCESS | FAILED
	TriggeredBy string      `json:"triggered_by"`
	Details     string      `json:"details"`
}

type Runtime struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	BinPath    string    `json:"bin_path"`
	DetectedAt time.Time `json:"detected_at"`
	Overridden bool      `json:"overridden"`
}

