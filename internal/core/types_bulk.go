package core

import "time"

type BulkDeleteMode string

const (
	BulkDeleteModeNone BulkDeleteMode = ""
	BulkDeleteModeSoft BulkDeleteMode = "soft"
	BulkDeleteModeHard BulkDeleteMode = "hard"
)

type BulkUserPatch struct {
	IDs               []string   `json:"ids"`
	Enabled           *bool      `json:"enabled,omitempty"`
	ExtendSeconds     int64      `json:"extend_seconds,omitempty"`
	SetExpireAt       *time.Time `json:"set_expire_at,omitempty"`
	ClearExpire       bool       `json:"clear_expire,omitempty"`
	TrafficLimitBytes *int64     `json:"traffic_limit_bytes,omitempty"`
	ClientProfileID   *string    `json:"client_profile_id,omitempty"`
	InboundID         *string    `json:"inbound_id,omitempty"`
	RotateTokens      bool       `json:"rotate_tokens,omitempty"`
	RegenerateArtifacts bool     `json:"regenerate_artifacts,omitempty"`
	DeleteMode        BulkDeleteMode `json:"delete_mode,omitempty"`
}

type BulkAccessPatch struct {
	IDs               []string   `json:"ids"`
	Enabled           *bool      `json:"enabled,omitempty"`
	ExtendSeconds     int64      `json:"extend_seconds,omitempty"`
	SetExpireAt       *time.Time `json:"set_expire_at,omitempty"`
	ClearExpire       bool       `json:"clear_expire,omitempty"`
	TrafficLimitBytes *int64     `json:"traffic_limit_bytes,omitempty"`
	ClientProfileID   *string    `json:"client_profile_id,omitempty"`
	InboundID         *string    `json:"inbound_id,omitempty"`
	RotateCredentials bool       `json:"rotate_credentials,omitempty"`
	RegenerateArtifacts bool     `json:"regenerate_artifacts,omitempty"`
	DeleteMode        BulkDeleteMode `json:"delete_mode,omitempty"`
}
