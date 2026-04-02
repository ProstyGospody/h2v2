package panel

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Protocol string

const (
	ProtocolHY2   Protocol = "hy2"
	ProtocolVLESS Protocol = "vless"
)

type CredentialType string

const (
	CredentialTypeUserPass CredentialType = "userpass"
	CredentialTypeUUID     CredentialType = "uuid"
)

type User struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	NameNormalized     string     `json:"name_normalized"`
	Enabled            bool       `json:"enabled"`
	TrafficLimitBytes  int64      `json:"traffic_limit_bytes"`
	TrafficUsedTxBytes int64      `json:"traffic_used_tx_bytes"`
	TrafficUsedRxBytes int64      `json:"traffic_used_rx_bytes"`
	ExpireAt           *time.Time `json:"expire_at,omitempty"`
	Note               *string    `json:"note,omitempty"`
	Subject            string     `json:"subject"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
}

type Credential struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	Protocol  Protocol       `json:"protocol"`
	Type      CredentialType `json:"type"`
	Identity  string         `json:"identity"`
	Secret    string         `json:"secret,omitempty"`
	DataJSON  string         `json:"data_json,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type UserWithCredentials struct {
	User
	Credentials []Credential `json:"credentials"`
	OnlineCount int          `json:"online_count"`
	DownloadBps float64      `json:"download_bps"`
	UploadBps   float64      `json:"upload_bps"`
}

type Node struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Inbound struct {
	ID          string    `json:"id"`
	NodeID      string    `json:"node_id"`
	Name        string    `json:"name"`
	Protocol    Protocol  `json:"protocol"`
	Transport   string    `json:"transport"`
	Security    string    `json:"security"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Enabled     bool      `json:"enabled"`
	ParamsJSON  string    `json:"params_json,omitempty"`
	RuntimeJSON string    `json:"runtime_json,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SubscriptionToken struct {
	UserID    string     `json:"user_id"`
	Subject   string     `json:"subject"`
	Version   int        `json:"version"`
	Revoked   bool       `json:"revoked"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type TrafficCounter struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"user_id"`
	Protocol   Protocol  `json:"protocol"`
	TxBytes    int64     `json:"tx_bytes"`
	RxBytes    int64     `json:"rx_bytes"`
	Online     int       `json:"online_count"`
	SnapshotAt time.Time `json:"snapshot_at"`
}

var userNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,62}[a-z0-9]$`)

func NormalizeUserName(input string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("name is required")
	}
	if !userNamePattern.MatchString(value) {
		return "", fmt.Errorf("name must be 3-64 chars and use a-z, 0-9, dot, dash, or underscore")
	}
	return value, nil
}

func NormalizeNote(input *string) *string {
	if input == nil {
		return nil
	}
	value := strings.TrimSpace(*input)
	if value == "" {
		return nil
	}
	return &value
}

func ValidateLifecycle(enabled bool, limitBytes int64, usedTx int64, usedRx int64, expireAt *time.Time, now time.Time) error {
	if limitBytes < 0 {
		return fmt.Errorf("traffic limit must be greater or equal to zero")
	}
	if usedTx < 0 || usedRx < 0 {
		return fmt.Errorf("traffic counters must be greater or equal to zero")
	}
	if !enabled {
		return nil
	}
	if limitBytes > 0 && (usedTx+usedRx) >= limitBytes {
		return fmt.Errorf("traffic limit reached")
	}
	if expireAt != nil && !expireAt.UTC().After(now.UTC()) {
		return fmt.Errorf("user expired")
	}
	return nil
}

func ValidateProtocol(protocol Protocol) error {
	switch protocol {
	case ProtocolHY2, ProtocolVLESS:
		return nil
	default:
		return fmt.Errorf("unsupported protocol")
	}
}
