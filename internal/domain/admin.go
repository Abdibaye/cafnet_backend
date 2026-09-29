package domain

import (
	"time"
)

// AdminAction represents an immutable audit log entry recording a governance decision.
type AdminAction struct {
	ID               AdminActionID   `json:"id"`
	AdminUserID      UserID          `json:"admin_user_id"`
	ActionType       AdminActionType `json:"action_type"`
	TargetEntityType string          `json:"target_entity_type"`
	TargetEntityID   string          `json:"target_entity_id"`
	Metadata         map[string]any  `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
}
