package repository

import "time"

type Admin struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Session struct {
	ID               string    `json:"id"`
	AdminID          string    `json:"admin_id"`
	SessionTokenHash string    `json:"session_token_hash"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	IP               string    `json:"ip"`
	UserAgent        string    `json:"user_agent"`
}

type SystemSnapshot struct {
	ID                int64     `json:"id"`
	SnapshotAt        time.Time `json:"snapshot_at"`
	CPUUsagePercent   float64   `json:"cpu_usage_percent"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	NetworkRxBps      float64   `json:"network_rx_bps"`
	NetworkTxBps      float64   `json:"network_tx_bps"`
}

type ServiceState struct {
	ID          int64      `json:"id"`
	ServiceName string     `json:"service_name"`
	Status      string     `json:"status"`
	Version     *string    `json:"version"`
	LastCheckAt time.Time  `json:"last_check_at"`
	RawJSON     *string    `json:"raw_json"`
}
