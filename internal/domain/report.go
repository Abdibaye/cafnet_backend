package domain

import (
	"time"
)

// ReviewReport represents community reporting of suspicious or abusive reviews.
type ReviewReport struct {
	ID             ReportID           `json:"id"`
	ReviewID       ReviewID           `json:"review_id"`
	ReporterUserID UserID             `json:"reporter_user_id"`
	Reason         ReviewReportReason `json:"reason"`
	Explanation    string             `json:"explanation"`
	Status         ReportStatus       `json:"status"`
	ReviewedAt     *time.Time         `json:"reviewed_at,omitempty"`
	ReviewedBy     *UserID            `json:"reviewed_by,omitempty"`
	AdminAction    *string            `json:"admin_action,omitempty"` // "dismissed" or "removed"
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// BusinessReport represents community reporting of inaccurate business information.
type BusinessReport struct {
	ID             ReportID             `json:"id"`
	BusinessID     BusinessID           `json:"business_id"`
	ReporterUserID UserID               `json:"reporter_user_id"`
	Reason         BusinessReportReason `json:"reason"`
	Details        *string              `json:"details,omitempty"`
	Status         ReportStatus         `json:"status"`
	ReviewedAt     *time.Time           `json:"reviewed_at,omitempty"`
	ReviewedBy     *UserID              `json:"reviewed_by,omitempty"`
	AdminNotes     *string              `json:"admin_notes,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}
