package domain

import (
	"time"
)

// BusinessManager represents an authorized management relationship between a user and a business.
type BusinessManager struct {
	ID          string      `json:"id"`
	BusinessID  BusinessID  `json:"business_id"`
	UserID      UserID      `json:"user_id"`
	ManagerRole ManagerRole `json:"manager_role"`
	GrantedBy   *UserID     `json:"granted_by,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// BusinessClaim represents a formal petition to claim management rights for an existing business.
type BusinessClaim struct {
	ID              ClaimID             `json:"id"`
	BusinessID      BusinessID          `json:"business_id"`
	ClaimantUserID  UserID              `json:"claimant_user_id"`
	Status          ClaimWorkflowStatus `json:"status"`
	SubmittedAt     time.Time           `json:"submitted_at"`
	ReviewedAt      *time.Time          `json:"reviewed_at,omitempty"`
	ReviewedBy      *UserID             `json:"reviewed_by,omitempty"`
	RejectionReason *string             `json:"rejection_reason,omitempty"`
	CreatedAt       time.Time           `json:"created_at"`
	UpdatedAt       time.Time           `json:"updated_at"`
}

// BusinessSubmission represents a proposed new venue for inclusion in the Cafnet directory.
type BusinessSubmission struct {
	ID                SubmissionID             `json:"id"`
	SubmitterUserID   UserID                   `json:"submitter_user_id"`
	BusinessName      string                   `json:"business_name"`
	Category          BusinessCategory         `json:"category"`
	Area              string                   `json:"area"`
	SubmittedPayload  map[string]any           `json:"submitted_payload"`
	Status            SubmissionWorkflowStatus `json:"status"`
	CreatedBusinessID *BusinessID              `json:"created_business_id,omitempty"`
	SubmittedAt       time.Time                `json:"submitted_at"`
	ReviewedAt        *time.Time               `json:"reviewed_at,omitempty"`
	ReviewedBy        *UserID                  `json:"reviewed_by,omitempty"`
	RejectionReason   *string                  `json:"rejection_reason,omitempty"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}
