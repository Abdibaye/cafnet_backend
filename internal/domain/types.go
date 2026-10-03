package domain

// Strongly-typed identifiers providing compile-time type safety across the domain.
type (
	UserID        string
	BusinessID    int64
	ReservationID string
	ReviewID      string
	CategoryID    string
	ItemID        string
	UpdateID      string
	ClaimID       string
	SubmissionID  string
	ReportID      string
	AdminActionID string
)

// UserRole defines the platform security role of a user identity.
type UserRole string

const (
	RoleCustomer UserRole = "CUSTOMER"
	RoleAdmin    UserRole = "ADMIN"
)

// BusinessCategory defines top-level classification for venues.
type BusinessCategory string

const (
	CategoryRestaurant BusinessCategory = "restaurant"
	CategoryCafe       BusinessCategory = "cafe"
	CategoryDrinks     BusinessCategory = "drinks"
)

// PriceRange indicates the price level of a venue.
type PriceRange string

const (
	PriceRangeBudget        PriceRange = "$"
	PriceRangeModerate      PriceRange = "$$"
	PriceRangeExpensive     PriceRange = "$$$"
	PriceRangeVeryExpensive PriceRange = "$$$$"
)

// PublicationStatus indicates whether a business is visible in public discovery.
type PublicationStatus string

const (
	PublicationStatusPublished   PublicationStatus = "PUBLISHED"
	PublicationStatusUnpublished PublicationStatus = "UNPUBLISHED"
)

// VerificationStatus indicates whether a business holds an official authenticity badge.
type VerificationStatus string

const (
	VerificationStatusVerified   VerificationStatus = "VERIFIED"
	VerificationStatusUnverified VerificationStatus = "UNVERIFIED"
)

// ClaimStatus indicates whether a business has an active owner management relationship.
type ClaimStatus string

const (
	ClaimStatusUnclaimed ClaimStatus = "UNCLAIMED"
	ClaimStatusClaimed   ClaimStatus = "CLAIMED"
)

// CreationSource indicates whether a business was created by an admin or from an owner submission.
type CreationSource string

const (
	CreationSourceOwnerSubmission CreationSource = "OWNER_SUBMISSION"
	CreationSourceAdminCreated    CreationSource = "ADMIN_CREATED"
)

// ManagerRole specifies the privilege tier of an authorized business manager.
type ManagerRole string

const (
	ManagerRolePrimaryOwner ManagerRole = "PRIMARY_OWNER"
	ManagerRoleManager      ManagerRole = "MANAGER"
)

// ClaimWorkflowStatus represents the adjudication lifecycle of a business ownership claim.
type ClaimWorkflowStatus string

const (
	ClaimStatusPending  ClaimWorkflowStatus = "PENDING"
	ClaimStatusApproved ClaimWorkflowStatus = "APPROVED"
	ClaimStatusRejected ClaimWorkflowStatus = "REJECTED"
)

// SubmissionWorkflowStatus represents the adjudication lifecycle of a new venue submission.
type SubmissionWorkflowStatus string

const (
	SubmissionStatusPending  SubmissionWorkflowStatus = "PENDING"
	SubmissionStatusApproved SubmissionWorkflowStatus = "APPROVED"
	SubmissionStatusRejected SubmissionWorkflowStatus = "REJECTED"
)

// UpdateType defines the broadcast category of a business update.
type UpdateType string

const (
	UpdateTypeNewMenu      UpdateType = "new_menu"
	UpdateTypeOffer        UpdateType = "offer"
	UpdateTypeEvent        UpdateType = "event"
	UpdateTypePriceUpdate  UpdateType = "price_update"
	UpdateTypeAnnouncement UpdateType = "announcement"
)

// UpdateStatus represents the lifecycle state of a broadcast update.
type UpdateStatus string

const (
	UpdateStatusDraft     UpdateStatus = "DRAFT"
	UpdateStatusPublished UpdateStatus = "PUBLISHED"
	UpdateStatusExpired   UpdateStatus = "EXPIRED"
)

// ReservationStatus represents the canonical 5-state lifecycle of a dining reservation.
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "PENDING"
	ReservationStatusConfirmed ReservationStatus = "CONFIRMED"
	ReservationStatusDeclined  ReservationStatus = "DECLINED"
	ReservationStatusCancelled ReservationStatus = "CANCELLED"
	ReservationStatusCompleted ReservationStatus = "COMPLETED"
)

// ReviewStatus represents the visibility state of a customer review.
type ReviewStatus string

const (
	ReviewStatusVisible        ReviewStatus = "VISIBLE"
	ReviewStatusRemovedByAdmin ReviewStatus = "REMOVED_BY_ADMIN"
)

// ReviewReportReason specifies the community abuse category for reporting a review.
type ReviewReportReason string

const (
	ReviewReportReasonSpam      ReviewReportReason = "spam"
	ReviewReportReasonOffensive ReviewReportReason = "offensive"
	ReviewReportReasonFake      ReviewReportReason = "fake"
	ReviewReportReasonOther     ReviewReportReason = "other"
)

// ReportStatus represents the administrative triage status of a review or business report.
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "PENDING"
	ReportStatusDismissed ReportStatus = "DISMISSED"
	ReportStatusActioned  ReportStatus = "ACTIONED"
)

// BusinessReportReason specifies community flags for inaccurate business profile data.
type BusinessReportReason string

const (
	BusinessReportReasonWrongHours    BusinessReportReason = "WRONG_HOURS"
	BusinessReportReasonWrongPhone    BusinessReportReason = "WRONG_PHONE"
	BusinessReportReasonWrongLocation BusinessReportReason = "WRONG_LOCATION"
	BusinessReportReasonWrongInfo     BusinessReportReason = "WRONG_INFORMATION"
	BusinessReportReasonClosed        BusinessReportReason = "CLOSED"
	BusinessReportReasonDuplicate     BusinessReportReason = "DUPLICATE"
	BusinessReportReasonOther         BusinessReportReason = "OTHER"
)

// AdminActionType denotes the governance action executed by an administrator.
type AdminActionType string

const (
	AdminActionBusinessCreated           AdminActionType = "BUSINESS_CREATED"
	AdminActionBusinessAdminCreated      AdminActionType = "BUSINESS_ADMIN_CREATED"
	AdminActionBusinessApproved          AdminActionType = "BUSINESS_APPROVED"
	AdminActionBusinessRejected          AdminActionType = "BUSINESS_REJECTED"
	AdminActionClaimApproved             AdminActionType = "CLAIM_APPROVED"
	AdminActionClaimRejected             AdminActionType = "CLAIM_REJECTED"
	AdminActionReviewRemoved             AdminActionType = "REVIEW_REMOVED"
	AdminActionReviewReportDismissed     AdminActionType = "REVIEW_REPORT_DISMISSED"
	AdminActionBusinessReportActioned    AdminActionType = "BUSINESS_REPORT_ACTIONED"
	AdminActionBusinessReportDismissed   AdminActionType = "BUSINESS_REPORT_DISMISSED"
	AdminActionPublicationStatusChanged  AdminActionType = "PUBLICATION_STATUS_CHANGED"
	AdminActionVerificationStatusChanged AdminActionType = "VERIFICATION_STATUS_CHANGED"
)
