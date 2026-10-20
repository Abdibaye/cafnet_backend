package domain

// Permission represents an explicit operation authority within Cafnet.
type Permission string

const (
	// Business management permissions (Requires approved BusinessManager relationship or Admin override)
	PermissionBusinessViewManagement     Permission = "business.view_management"
	PermissionBusinessEdit               Permission = "business.edit"
	PermissionBusinessManageMenu         Permission = "business.manage_menu"
	PermissionBusinessManageUpdates      Permission = "business.manage_updates"
	PermissionBusinessManageReservations Permission = "business.manage_reservations"
	PermissionBusinessManageMedia        Permission = "business.manage_media"

	// Platform admin permissions (Requires UserRole == ADMIN)
	PermissionAdminViewDashboard     Permission = "admin.view_dashboard"
	PermissionAdminReviewSubmissions Permission = "admin.business_submission_review"
	PermissionAdminReviewClaims      Permission = "admin.claim_review"
	PermissionAdminModerateReviews   Permission = "admin.review_moderation"
	PermissionAdminModerateReports   Permission = "admin.business_report_moderation"
	PermissionAdminManageBadges      Permission = "admin.badge_and_publication"
	PermissionAdminCreateBusiness    Permission = "admin.create_business"
	PermissionAdminDeleteBusiness    Permission = "admin.delete_business"
	PermissionAdminManageTaxonomy    Permission = "admin.manage_taxonomy"
)
