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

// AdminStats aggregates operational platform counts across submissions, claims, reports, and venues.
type AdminStats struct {
	PendingClaimsCount            int `json:"pending_claims_count"`
	ApprovedClaimsCount           int `json:"approved_claims_count"`
	RejectedClaimsCount           int `json:"rejected_claims_count"`
	PendingSubmissionsCount       int `json:"pending_submissions_count"`
	ApprovedSubmissionsCount      int `json:"approved_submissions_count"`
	RejectedSubmissionsCount      int `json:"rejected_submissions_count"`
	TotalBusinessesCount          int `json:"total_businesses_count"`
	VerifiedBusinessesCount       int `json:"verified_businesses_count"`
	PendingReviewReportsCount     int `json:"pending_review_reports_count"`
	DismissedReviewReportsCount   int `json:"dismissed_review_reports_count"`
	ActionedReviewReportsCount    int `json:"actioned_review_reports_count"`
	PendingBusinessReportsCount   int `json:"pending_business_reports_count"`
	DismissedBusinessReportsCount int `json:"dismissed_business_reports_count"`
	ActionedBusinessReportsCount  int `json:"actioned_business_reports_count"`
}
