package hysteria

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{1,62}[a-z0-9])?$`)

type User struct {
	ID                 string     `json:"id"`
	Username           string     `json:"username"`
	UsernameNormalized string     `json:"username_normalized"`
	Password           string     `json:"password"`
	Enabled            bool       `json:"enabled"`
	Note               *string    `json:"note,omitempty"`
	ClientOverrides    *ClientOverrides `json:"client_overrides,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
}

type ClientOverrides struct {
	SNI          *string `json:"sni,omitempty"`
	Insecure     *bool   `json:"insecure,omitempty"`
	PinSHA256    *string `json:"pinSHA256,omitempty"`
	ObfsType     *string `json:"obfsType,omitempty"`
	ObfsPassword *string `json:"obfsPassword,omitempty"`
}

type UserView struct {
	User
	LastTxBytes  int64   `json:"last_tx_bytes"`
	LastRxBytes  int64   `json:"last_rx_bytes"`
	OnlineCount  int     `json:"online_count"`
	DownloadBps  float64 `json:"download_bps"`
	UploadBps    float64 `json:"upload_bps"`
}

type Snapshot struct {
	ID         int64     `json:"id"`
	UserID     string    `json:"hysteria_user_id"`
	TxBytes    int64     `json:"tx_bytes"`
	RxBytes    int64     `json:"rx_bytes"`
	Online     int       `json:"online_count"`
	SnapshotAt time.Time `json:"snapshot_at"`
}

type Overview struct {
	EnabledUsers int64 `json:"enabled_users"`
	TotalTxBytes int64 `json:"total_tx_bytes"`
	TotalRxBytes int64 `json:"total_rx_bytes"`
	OnlineCount  int64 `json:"online_count"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func NormalizeUsername(input string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("username is required")
	}
	if !usernamePattern.MatchString(value) {
		return "", fmt.Errorf("username must be 3-64 chars and use a-z, 0-9, dot, dash, or underscore")
	}
	return value, nil
}

func NormalizePassword(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", fmt.Errorf("password is required")
	}
	if len(value) < 8 {
		return "", fmt.Errorf("password must be at least 8 characters")
	}
	for _, ch := range value {
		if ch < 0x21 || ch == 0x7f {
			return "", fmt.Errorf("password must not contain spaces or control characters")
		}
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

func NormalizeClientOverrides(input *ClientOverrides) *ClientOverrides {
	if input == nil {
		return nil
	}

	out := ClientOverrides{}
	if input.SNI != nil {
		value := strings.TrimSpace(*input.SNI)
		if value != "" {
			out.SNI = &value
		}
	}
	if input.Insecure != nil {
		value := *input.Insecure
		out.Insecure = &value
	}
	if input.PinSHA256 != nil {
		value := strings.TrimSpace(*input.PinSHA256)
		if value != "" {
			out.PinSHA256 = &value
		}
	}
	if input.ObfsType != nil {
		value := strings.ToLower(strings.TrimSpace(*input.ObfsType))
		if value != "" {
			out.ObfsType = &value
		}
	}
	if input.ObfsPassword != nil {
		value := strings.TrimSpace(*input.ObfsPassword)
		if value != "" {
			out.ObfsPassword = &value
		}
	}

	if out.ObfsType == nil && out.ObfsPassword != nil {
		value := "salamander"
		out.ObfsType = &value
	}
	if out.ObfsType != nil && *out.ObfsType != "salamander" {
		out.ObfsPassword = nil
	}

	if out.SNI == nil && out.Insecure == nil && out.PinSHA256 == nil && out.ObfsType == nil && out.ObfsPassword == nil {
		return nil
	}
	return &out
}

func BuildCredential(user User) string {
	return strings.TrimSpace(user.Username) + ":" + strings.TrimSpace(user.Password)
}

func ValidateUserInput(username string, password string) []ValidationError {
	errors := make([]ValidationError, 0, 2)
	if _, err := NormalizeUsername(username); err != nil {
		errors = append(errors, ValidationError{Field: "username", Message: err.Error()})
	}
	if _, err := NormalizePassword(password); err != nil {
		errors = append(errors, ValidationError{Field: "password", Message: err.Error()})
	}
	return errors
}

func ValidateClientOverrides(input *ClientOverrides) []ValidationError {
	overrides := NormalizeClientOverrides(input)
	if overrides == nil {
		return nil
	}
	errors := make([]ValidationError, 0, 3)

	if overrides.SNI != nil && !isValidOverrideHost(*overrides.SNI) {
		errors = append(errors, ValidationError{Field: "overrides.sni", Message: "sni must be a valid host"})
	}
	if overrides.PinSHA256 != nil && !IsValidPinSHA256(*overrides.PinSHA256) {
		errors = append(errors, ValidationError{Field: "overrides.pinSHA256", Message: "pinSHA256 must be a valid SHA-256 certificate fingerprint"})
	}
	if overrides.ObfsType != nil && *overrides.ObfsType != "salamander" {
		errors = append(errors, ValidationError{Field: "overrides.obfsType", Message: "obfsType must be salamander"})
	}
	if overrides.ObfsType != nil && *overrides.ObfsType == "salamander" && (overrides.ObfsPassword == nil || strings.TrimSpace(*overrides.ObfsPassword) == "") {
		errors = append(errors, ValidationError{Field: "overrides.obfsPassword", Message: "obfsPassword is required when obfsType is salamander"})
	}

	return errors
}

func isValidOverrideHost(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, " /?#") {
		return false
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.Trim(value, "[]")
	}
	if ip := net.ParseIP(value); ip != nil {
		return true
	}
	if strings.Contains(value, ":") {
		return false
	}
	if parsed, err := url.Parse("https://" + value); err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return false
	}
	return true
}

func IsValidPinSHA256(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	normalized := strings.Builder{}
	normalized.Grow(len(value))
	for _, ch := range value {
		switch ch {
		case ':', '-', ' ':
			continue
		default:
			normalized.WriteRune(unicode.ToLower(ch))
		}
	}
	hex := normalized.String()
	if len(hex) != 64 {
		return false
	}
	for _, ch := range hex {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

