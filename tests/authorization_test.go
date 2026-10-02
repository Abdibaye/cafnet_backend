package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

type authzTestContext struct {
	pool          any
	ctx           context.Context
	userRepo      repository.UserRepository
	businessRepo  repository.BusinessRepository
	ownershipRepo repository.OwnershipRepository
	claimRepo     repository.ClaimRepository
	subRepo       repository.SubmissionRepository
	adminRepo     repository.AdminActionRepository
	mediaRepo     repository.MediaRepository
	storageClient storage.ObjectStorage
	txMgr         repository.TxManager
	authzSvc      service.AuthorizationService
	workflowSvc   service.OwnershipWorkflowService
}

func setupAuthzTest(t *testing.T) *authzTestContext {
	t.Helper()
	pool, ctx := setupTestDB(t)

	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	adminRepo := repository.NewAdminActionRepository(pool)
	mediaRepo := repository.NewMediaRepository(pool)
	storageClient := storage.NewMemoryStorage("http://localhost:9000/test-bucket")
	txMgr := repository.NewTxManager(pool)

	authzSvc := service.NewAuthorizationService(ownershipRepo, businessRepo)
	workflowSvc := service.NewOwnershipWorkflowService(
		authzSvc,
		claimRepo,
		subRepo,
		businessRepo,
		ownershipRepo,
		adminRepo,
		mediaRepo,
		storageClient,
		txMgr,
	)

	return &authzTestContext{
		pool:          pool,
		ctx:           ctx,
		userRepo:      userRepo,
		businessRepo:  businessRepo,
		ownershipRepo: ownershipRepo,
		claimRepo:     claimRepo,
		subRepo:       subRepo,
		adminRepo:     adminRepo,
		txMgr:         txMgr,
		authzSvc:      authzSvc,
		workflowSvc:   workflowSvc,
	}
}

func createTestUserWithRole(t *testing.T, tc *authzTestContext, role domain.UserRole, name string) *domain.User {
	t.Helper()
	u := &domain.User{
		Phone:    uniquePhone(),
		FullName: name,
		Role:     role,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, u); err != nil {
		t.Fatalf("Failed to create user %s: %v", name, err)
	}
	return u
}

func createTestVenue(t *testing.T, tc *authzTestContext, name string) *domain.Business {
	t.Helper()
	b := &domain.Business{
		Name:                 name,
		Slug:                 uniqueSlug("venue"),
		Category:             domain.CategoryRestaurant,
		Cuisine:              "Ethiopian",
		Description:          "Test venue for authorization",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		PriceRange:           "$$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	if err := tc.businessRepo.Create(tc.ctx, b); err != nil {
		t.Fatalf("Failed to create venue %s: %v", name, err)
	}
	return b
}

func TestCrossBusinessAuthorization(t *testing.T) {
	tc := setupAuthzTest(t)

	// Create 3 distinct businesses
	bizA := createTestVenue(t, tc, "Restaurant Alpha")
	bizB := createTestVenue(t, tc, "Restaurant Beta")
	bizC := createTestVenue(t, tc, "Restaurant Gamma")

	// Create 2 normal users
	userA := createTestUserWithRole(t, tc, domain.RoleCustomer, "Manager Alpha")
	userB := createTestUserWithRole(t, tc, domain.RoleCustomer, "Manager Beta")

	// User A is granted management of Biz A
	err := tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  bizA.ID,
		UserID:      userA.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})
	if err != nil {
		t.Fatalf("AddManager failed: %v", err)
	}

	// User B is granted management of Biz B
	err = tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  bizB.ID,
		UserID:      userB.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})
	if err != nil {
		t.Fatalf("AddManager failed: %v", err)
	}

	// 1. User A -> Biz A: ALLOW
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, userA, bizA.ID); err != nil {
		t.Errorf("User A should manage Biz A, got err: %v", err)
	}
	if err := tc.authzSvc.CanManageMenu(tc.ctx, userA, bizA.ID); err != nil {
		t.Errorf("User A should manage menu on Biz A, got err: %v", err)
	}

	// 2. User A -> Biz B: DENY (Cross-business isolation)
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, userA, bizB.ID); !errors.Is(err, domain.ErrNotBusinessManager) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: User A allowed to edit Biz B! Got: %v", err)
	}
	if err := tc.authzSvc.CanManageReservations(tc.ctx, userA, bizB.ID); !errors.Is(err, domain.ErrNotBusinessManager) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: User A allowed to manage reservations on Biz B! Got: %v", err)
	}

	// 3. User B -> Biz B: ALLOW
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, userB, bizB.ID); err != nil {
		t.Errorf("User B should manage Biz B, got err: %v", err)
	}

	// 4. User B -> Biz A: DENY
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, userB, bizA.ID); !errors.Is(err, domain.ErrNotBusinessManager) {
		t.Fatalf("CRITICAL SECURITY VIOLATION: User B allowed to edit Biz A! Got: %v", err)
	}

	// 5. Multi-business management: Grant User A management over Biz C as well
	err = tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  bizC.ID,
		UserID:      userA.ID,
		ManagerRole: domain.ManagerRoleManager,
	})
	if err != nil {
		t.Fatalf("AddManager Biz C failed: %v", err)
	}

	// User A -> Biz A: ALLOW
	if err := tc.authzSvc.CanPublishUpdate(tc.ctx, userA, bizA.ID); err != nil {
		t.Errorf("User A should manage Biz A, got: %v", err)
	}
	// User A -> Biz C: ALLOW
	if err := tc.authzSvc.CanPublishUpdate(tc.ctx, userA, bizC.ID); err != nil {
		t.Errorf("User A should manage Biz C, got: %v", err)
	}
	// User A -> Biz B: STILL DENIED
	if err := tc.authzSvc.CanPublishUpdate(tc.ctx, userA, bizB.ID); !errors.Is(err, domain.ErrNotBusinessManager) {
		t.Fatalf("User A must still be denied access to Biz B, got: %v", err)
	}
}

func TestUnauthenticatedAndFailClosed(t *testing.T) {
	tc := setupAuthzTest(t)
	biz := createTestVenue(t, tc, "Fail Closed Cafe")

	// Nil user must fail closed with ErrUnauthenticated
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, nil, biz.ID); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("Expected ErrUnauthenticated for nil user, got: %v", err)
	}
	if err := tc.authzSvc.AuthorizeAdmin(tc.ctx, nil, domain.PermissionAdminReviewClaims); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("Expected ErrUnauthenticated for nil admin, got: %v", err)
	}
	if err := tc.authzSvc.AuthorizeUserResource(nil, domain.UserID(uuid.New().String())); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("Expected ErrUnauthenticated for nil user resource check, got: %v", err)
	}

	// Non-existent business ID must return ErrNotFound
	customer := createTestUserWithRole(t, tc, domain.RoleCustomer, "Regular Guy")
	if err := tc.authzSvc.CanEditBusiness(tc.ctx, customer, domain.BusinessID(99999999)); !errors.Is(err, domain.ErrNotBusinessManager) {
		t.Fatalf("Expected ErrNotBusinessManager for unmanaged business, got: %v", err)
	}
}

func TestCustomerResourceOwnership(t *testing.T) {
	tc := setupAuthzTest(t)

	user1 := createTestUserWithRole(t, tc, domain.RoleCustomer, "Customer One")
	user2 := createTestUserWithRole(t, tc, domain.RoleCustomer, "Customer Two")

	// 1. User 1 can access own data
	if err := tc.authzSvc.AuthorizeUserResource(user1, user1.ID); err != nil {
		t.Errorf("User 1 should access own resource, got: %v", err)
	}

	// 2. User 1 cannot access User 2's private data
	if err := tc.authzSvc.AuthorizeUserResource(user1, user2.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when User 1 accesses User 2 data, got: %v", err)
	}

	// 3. Review modification authorization (own review only)
	if err := tc.authzSvc.AuthorizeReviewModification(user1, user1.ID); err != nil {
		t.Errorf("User 1 should edit own review, got: %v", err)
	}
	if err := tc.authzSvc.AuthorizeReviewModification(user1, user2.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when User 1 attempts to edit User 2 review, got: %v", err)
	}

	// 4. Reservation cancellation authorization
	if err := tc.authzSvc.AuthorizeReservationCancellation(user1, user1.ID, domain.ReservationStatusPending); err != nil {
		t.Errorf("User 1 should cancel own pending reservation, got: %v", err)
	}
	if err := tc.authzSvc.AuthorizeReservationCancellation(user1, user2.ID, domain.ReservationStatusPending); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("User 1 must not cancel User 2 reservation, got: %v", err)
	}
	if err := tc.authzSvc.AuthorizeReservationCancellation(user1, user1.ID, domain.ReservationStatusCompleted); !errors.Is(err, domain.ErrIllegalStateTransition) {
		t.Fatalf("Cannot cancel COMPLETED reservation, expected ErrIllegalStateTransition, got: %v", err)
	}
}

func TestAdminAuthorizationAndPrivilegeBoundary(t *testing.T) {
	tc := setupAuthzTest(t)

	admin := createTestUserWithRole(t, tc, domain.RoleAdmin, "Platform Governor")
	customer := createTestUserWithRole(t, tc, domain.RoleCustomer, "Regular User")

	// 1. Admin checks pass
	if err := tc.authzSvc.AuthorizeAdmin(tc.ctx, admin, domain.PermissionAdminReviewClaims); err != nil {
		t.Errorf("Admin check should pass for admin user, got: %v", err)
	}
	if !tc.authzSvc.CanReviewClaims(admin) || !tc.authzSvc.CanReviewBusinessSubmissions(admin) {
		t.Errorf("Admin predicates must return true for admin")
	}

	// 2. Normal customer cannot access admin operations
	if err := tc.authzSvc.AuthorizeAdmin(tc.ctx, customer, domain.PermissionAdminReviewClaims); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden for normal user attempting admin operation, got: %v", err)
	}
	if tc.authzSvc.CanReviewClaims(customer) {
		t.Errorf("CanReviewClaims must return false for normal user")
	}

	// 3. Business manager does NOT automatically become admin
	biz := createTestVenue(t, tc, "Manager Admin Isolation Venue")
	_ = tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  biz.ID,
		UserID:      customer.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})
	if err := tc.authzSvc.AuthorizeAdmin(tc.ctx, customer, domain.PermissionAdminReviewSubmissions); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Business manager must NOT automatically have admin authority, got: %v", err)
	}
}

func TestClaimApprovalWorkflowAndSelfEscalationPrevention(t *testing.T) {
	tc := setupAuthzTest(t)

	admin := createTestUserWithRole(t, tc, domain.RoleAdmin, "System Admin")
	claimant := createTestUserWithRole(t, tc, domain.RoleCustomer, "Venue Claimant")
	biz := createTestVenue(t, tc, "Claimable Bistro")

	// Create pending claim
	claim := &domain.BusinessClaim{
		BusinessID:     biz.ID,
		ClaimantUserID: claimant.ID,
		Status:         domain.ClaimStatusPending,
	}
	if err := tc.claimRepo.Create(tc.ctx, claim); err != nil {
		t.Fatalf("Create claim failed: %v", err)
	}

	// 1. Pending claimant must NOT have management access!
	canManage, err := tc.authzSvc.CanManageBusiness(tc.ctx, claimant, biz.ID)
	if err != nil || canManage {
		t.Fatalf("Pending claimant must NOT have management access! canManage: %v, err: %v", canManage, err)
	}

	// 2. Self-approval escalation prevention: Claimant who is also admin cannot approve own claim
	adminClaimant := createTestUserWithRole(t, tc, domain.RoleAdmin, "Admin Who Owns Bistro")
	selfVenue := createTestVenue(t, tc, "Self Venue")
	selfClaimRecord := &domain.BusinessClaim{
		BusinessID:     selfVenue.ID,
		ClaimantUserID: adminClaimant.ID,
		Status:         domain.ClaimStatusPending,
	}
	if err := tc.claimRepo.Create(tc.ctx, selfClaimRecord); err != nil {
		t.Fatalf("Create selfClaimRecord failed: %v", err)
	}

	err = tc.workflowSvc.ApproveClaim(tc.ctx, adminClaimant, selfClaimRecord.ID, strPtr("Self approve attempt"))
	if !errors.Is(err, domain.ErrSelfApprovalForbidden) {
		t.Fatalf("Expected ErrSelfApprovalForbidden when admin approves their own claim, got: %v", err)
	}

	// 3. Normal user cannot approve claim
	err = tc.workflowSvc.ApproveClaim(tc.ctx, claimant, claim.ID, strPtr("Unauthorized approve"))
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when non-admin approves claim, got: %v", err)
	}

	// 4. Admin approves valid claim atomically
	notes := "Approved after verifying business registration."
	err = tc.workflowSvc.ApproveClaim(tc.ctx, admin, claim.ID, &notes)
	if err != nil {
		t.Fatalf("ApproveClaim failed: %v", err)
	}

	// 5. Claimant NOW has management access
	canManageNow, err := tc.authzSvc.CanManageBusiness(tc.ctx, claimant, biz.ID)
	if err != nil || !canManageNow {
		t.Fatalf("Approved claimant MUST have management access! canManage: %v, err: %v", canManageNow, err)
	}

	// Verify business claim status is CLAIMED
	updatedBiz, _ := tc.businessRepo.GetByID(tc.ctx, biz.ID)
	if updatedBiz.ClaimStatus != domain.ClaimStatusClaimed {
		t.Errorf("Expected business claim_status to be CLAIMED, got: %s", updatedBiz.ClaimStatus)
	}
}

func TestSubmissionApprovalWorkflowAndSelfEscalationPrevention(t *testing.T) {
	tc := setupAuthzTest(t)

	admin := createTestUserWithRole(t, tc, domain.RoleAdmin, "Platform Reviewer")
	submitter := createTestUserWithRole(t, tc, domain.RoleCustomer, "Entrepreneur")

	uniqueName1 := fmt.Sprintf("Atomic Approved Cafe %d", time.Now().UnixNano())
	sub := &domain.BusinessSubmission{
		SubmitterUserID: submitter.ID,
		BusinessName:    uniqueName1,
		Category:        domain.CategoryCafe,
		Area:            "Bole",
		SubmittedPayload: map[string]any{
			"cuisine": "Artisanal Coffee",
			"phone":   "+251911998877",
		},
		Status: domain.SubmissionStatusPending,
	}
	if err := tc.subRepo.Create(tc.ctx, sub); err != nil {
		t.Fatalf("Create submission failed: %v", err)
	}

	// 1. Non-admin cannot approve submission
	_, err := tc.workflowSvc.ApproveSubmission(tc.ctx, submitter, sub.ID)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when customer approves submission, got: %v", err)
	}

	// 2. Self-approval prevention
	adminSubmitter := createTestUserWithRole(t, tc, domain.RoleAdmin, "Admin Submitter")
	uniqueName2 := fmt.Sprintf("Self Submitted Cafe %d", time.Now().UnixNano())
	selfSub := &domain.BusinessSubmission{
		SubmitterUserID: adminSubmitter.ID,
		BusinessName:    uniqueName2,
		Category:        domain.CategoryCafe,
		Area:            "Kazanchis",
		Status:          domain.SubmissionStatusPending,
	}
	_ = tc.subRepo.Create(tc.ctx, selfSub)

	_, err = tc.workflowSvc.ApproveSubmission(tc.ctx, adminSubmitter, selfSub.ID)
	if !errors.Is(err, domain.ErrSelfApprovalForbidden) {
		t.Fatalf("Expected ErrSelfApprovalForbidden when admin approves own submission, got: %v", err)
	}

	// 3. Admin approves valid submission atomically
	createdBiz, err := tc.workflowSvc.ApproveSubmission(tc.ctx, admin, sub.ID)
	if err != nil {
		t.Fatalf("ApproveSubmission failed: %v", err)
	}
	if createdBiz == nil || createdBiz.ID == 0 {
		t.Fatalf("Expected non-nil created business")
	}

	// 4. Submitter NOW has management rights over the new business
	canManage, err := tc.authzSvc.CanManageBusiness(tc.ctx, submitter, createdBiz.ID)
	if err != nil || !canManage {
		t.Fatalf("Submitter MUST have management rights over approved business, got: %v", err)
	}

	// 5. Submission status is APPROVED
	updatedSub, _ := tc.subRepo.GetByID(tc.ctx, sub.ID)
	if updatedSub.Status != domain.SubmissionStatusApproved {
		t.Errorf("Expected submission status APPROVED, got: %s", updatedSub.Status)
	}
}
