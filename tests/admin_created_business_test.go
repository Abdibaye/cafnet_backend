package tests

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

func TestAdminCreateBusinessWorkflow(t *testing.T) {
	tc := setupServiceTest(t)

	admin := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Super Admin",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, admin); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	customer := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Normal Diner",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, customer); err != nil {
		t.Fatalf("create customer failed: %v", err)
	}

	// 1. Non-admin forbidden from creating business via AdminService
	_, err := tc.adminSvc.CreateBusiness(tc.ctx, customer, domain.AdminCreateBusinessInput{
		Name:         "Unauthorized Cafe",
		Category:     domain.CategoryCafe,
		Neighborhood: "Bole",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-admin create business, got: %v", err)
	}

	// 2. Admin creates canonical business
	bizName := fmt.Sprintf("Admin Bistro %d", time.Now().UnixNano()%1000000)
	addr := "Cameroon St, Bole"
	desc := "Specialty coffee roastery added by platform curator"
	phone := "+251911998877"
	website := "https://adminbistro.et"

	biz, err := tc.adminSvc.CreateBusiness(tc.ctx, admin, domain.AdminCreateBusinessInput{
		Name:              bizName,
		Category:          domain.CategoryCafe,
		Neighborhood:      "Bole",
		AddressLine:       &addr,
		Phone:             phone,
		Website:           &website,
		Description:       desc,
		PriceRange:        domain.PriceRangeModerate,
		PublicationStatus: domain.PublicationStatusPublished,
	})
	if err != nil {
		t.Fatalf("admin CreateBusiness failed: %v", err)
	}

	// 3. Verify invariants
	if biz.CreationSource != domain.CreationSourceAdminCreated {
		t.Errorf("expected CreationSource ADMIN_CREATED, got: %s", biz.CreationSource)
	}
	if biz.ClaimStatus != domain.ClaimStatusUnclaimed {
		t.Errorf("expected ClaimStatus UNCLAIMED, got: %s", biz.ClaimStatus)
	}
	if biz.PublicationStatus != domain.PublicationStatusPublished {
		t.Errorf("expected PublicationStatus PUBLISHED, got: %s", biz.PublicationStatus)
	}
	if biz.VerificationStatus != domain.VerificationStatusUnverified {
		t.Errorf("expected VerificationStatus UNVERIFIED, got: %s", biz.VerificationStatus)
	}
	if biz.CreatedByUserID == nil || *biz.CreatedByUserID != admin.ID {
		t.Errorf("expected CreatedByUserID to match admin.ID, got: %v", biz.CreatedByUserID)
	}

	// 4. Admin is NOT an owner / manager of the business
	isMgr, err := tc.ownershipRepo.IsManager(tc.ctx, biz.ID, admin.ID)
	if err != nil {
		t.Fatalf("IsManager check failed: %v", err)
	}
	if isMgr {
		t.Errorf("admin must NOT be a manager of an unclaimed business")
	}

	managers, err := tc.ownershipRepo.GetManagers(tc.ctx, biz.ID)
	if err != nil {
		t.Fatalf("GetManagers failed: %v", err)
	}
	if len(managers) != 0 {
		t.Errorf("expected 0 managers for unclaimed business, got: %d", len(managers))
	}

	// 5. Audit log record was created
	actions, err := tc.adminActionRepo.List(tc.ctx, 10, 0)
	if err != nil {
		t.Fatalf("adminActionRepo.List failed: %v", err)
	}
	if len(actions) == 0 {
		t.Fatalf("expected at least 1 admin action logged")
	}
	var foundAudit bool
	for _, a := range actions {
		if a.AdminUserID == admin.ID && a.ActionType == domain.AdminActionBusinessAdminCreated && a.TargetEntityType == "business" && a.TargetEntityID == fmt.Sprintf("%d", biz.ID) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Errorf("audit log missing ADMIN_CREATED action for business %d", biz.ID)
	}

	// 6. Public discovery: Business profile can be viewed by anyone
	profile, err := tc.discoverySvc.GetBusinessBySlug(tc.ctx, biz.Slug)
	if err != nil {
		t.Fatalf("discovery GetBusinessBySlug failed: %v", err)
	}
	if profile.ID != biz.ID {
		t.Errorf("profile ID mismatch: expected %d, got %d", biz.ID, profile.ID)
	}
	if profile.ClaimStatus != domain.ClaimStatusUnclaimed {
		t.Errorf("public profile must reflect UNCLAIMED, got: %s", profile.ClaimStatus)
	}
}

func TestAdminCreatedBusinessClaimAndApproval(t *testing.T) {
	tc := setupServiceTest(t)

	admin := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Platform Reviewer Admin",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, admin); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	claimant := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Legitimate Cafe Owner",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, claimant); err != nil {
		t.Fatalf("create claimant failed: %v", err)
	}

	// 1. Admin creates business
	bizName := fmt.Sprintf("Admin Harvest Cafe %d", time.Now().UnixNano()%1000000)
	biz, err := tc.adminSvc.CreateBusiness(tc.ctx, admin, domain.AdminCreateBusinessInput{
		Name:              bizName,
		Category:          domain.CategoryCafe,
		Neighborhood:      "Kazanchis",
		PublicationStatus: domain.PublicationStatusPublished,
	})
	if err != nil {
		t.Fatalf("CreateBusiness failed: %v", err)
	}
	originalBusinessID := biz.ID

	// 2. Claimant discovers the listing and submits a claim
	claim, err := tc.customerSvc.SubmitClaim(tc.ctx, claimant, biz.ID)
	if err != nil {
		t.Fatalf("SubmitClaim failed: %v", err)
	}
	if claim.Status != domain.ClaimStatusPending {
		t.Errorf("expected claim status PENDING, got: %s", claim.Status)
	}

	// Business remains UNCLAIMED while claim is pending
	currentBiz, err := tc.businessRepo.GetByID(tc.ctx, biz.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if currentBiz.ClaimStatus != domain.ClaimStatusUnclaimed {
		t.Errorf("business must remain UNCLAIMED while claim is pending, got: %s", currentBiz.ClaimStatus)
	}

	// 3. Admin approves the claim
	notes := "Verified trade license and identity documentation"
	if err := tc.adminSvc.ApproveClaim(tc.ctx, admin, claim.ID, &notes); err != nil {
		t.Fatalf("ApproveClaim failed: %v", err)
	}

	// 4. Critical invariant: Claiming MUST NOT create a duplicate business!
	approvedBiz, err := tc.businessRepo.GetByID(tc.ctx, originalBusinessID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if approvedBiz.ID != originalBusinessID {
		t.Fatalf("CRITICAL: Original business ID changed! Expected %d, got %d", originalBusinessID, approvedBiz.ID)
	}

	// 5. Verification of transitioned state
	if approvedBiz.ClaimStatus != domain.ClaimStatusClaimed {
		t.Errorf("expected approved business ClaimStatus CLAIMED, got: %s", approvedBiz.ClaimStatus)
	}
	if approvedBiz.CreationSource != domain.CreationSourceAdminCreated {
		t.Errorf("CreationSource should remain ADMIN_CREATED, got: %s", approvedBiz.CreationSource)
	}

	// 6. Claimant is now PRIMARY_OWNER
	isMgr, err := tc.ownershipRepo.IsManager(tc.ctx, originalBusinessID, claimant.ID)
	if err != nil || !isMgr {
		t.Fatalf("claimant is not manager after approval")
	}

	managers, err := tc.ownershipRepo.GetManagers(tc.ctx, originalBusinessID)
	if err != nil {
		t.Fatalf("GetManagers failed: %v", err)
	}
	if len(managers) != 1 {
		t.Fatalf("expected exactly 1 manager, got: %d", len(managers))
	}
	if managers[0].UserID != claimant.ID || managers[0].ManagerRole != domain.ManagerRolePrimaryOwner {
		t.Errorf("expected claimant to be PRIMARY_OWNER, got %+v", managers[0])
	}
}

func TestConcurrentClaimApprovalRaceCondition(t *testing.T) {
	tc := setupServiceTest(t)

	admin := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Approval Governor",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, admin); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	claimant1 := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Claimant One",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, claimant1); err != nil {
		t.Fatalf("create claimant1 failed: %v", err)
	}

	claimant2 := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Claimant Two",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, claimant2); err != nil {
		t.Fatalf("create claimant2 failed: %v", err)
	}

	// Admin creates business
	biz, err := tc.adminSvc.CreateBusiness(tc.ctx, admin, domain.AdminCreateBusinessInput{
		Name:              fmt.Sprintf("Contested Lounge %d", time.Now().UnixNano()%1000000),
		Category:          domain.CategoryDrinks,
		Neighborhood:      "Old Airport",
		PublicationStatus: domain.PublicationStatusPublished,
	})
	if err != nil {
		t.Fatalf("CreateBusiness failed: %v", err)
	}

	// Create claim 1
	claim1 := &domain.BusinessClaim{
		BusinessID:     biz.ID,
		ClaimantUserID: claimant1.ID,
		Status:         domain.ClaimStatusPending,
	}
	if err := tc.claimRepo.Create(tc.ctx, claim1); err != nil {
		t.Fatalf("Create claim1 failed: %v", err)
	}

	// 1. Approve claim 1: must succeed and mark business CLAIMED
	notes1 := "Approved for Claimant 1"
	if err := tc.adminSvc.ApproveClaim(tc.ctx, admin, claim1.ID, &notes1); err != nil {
		t.Fatalf("Approve claim1 failed: %v", err)
	}

	approvedBiz, err := tc.businessRepo.GetByID(tc.ctx, biz.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if approvedBiz.ClaimStatus != domain.ClaimStatusClaimed {
		t.Fatalf("expected business CLAIMED after claim1 approval")
	}

	// 2. Now create claim 2 (since claim 1 is no longer PENDING, DB partial index allows claim 2)
	claim2 := &domain.BusinessClaim{
		BusinessID:     biz.ID,
		ClaimantUserID: claimant2.ID,
		Status:         domain.ClaimStatusPending,
	}
	if err := tc.claimRepo.Create(tc.ctx, claim2); err != nil {
		t.Fatalf("Create claim2 failed: %v", err)
	}

	// 3. Attempting to approve claim 2 must FAIL safely because the business already has an active owner
	notes2 := "Attempting duplicate approval"
	err = tc.adminSvc.ApproveClaim(tc.ctx, admin, claim2.ID, &notes2)
	if err == nil {
		t.Fatalf("expected ApproveClaim for claim2 to fail, but it succeeded!")
	}
	if !errors.Is(err, domain.ErrConflict) && !errors.Is(err, domain.ErrBusinessAlreadyClaimed) {
		t.Errorf("expected ErrConflict or ErrBusinessAlreadyClaimed, got: %v", err)
	}

	// Verify only claimant 1 is manager
	managers, err := tc.ownershipRepo.GetManagers(tc.ctx, biz.ID)
	if err != nil {
		t.Fatalf("GetManagers failed: %v", err)
	}
	if len(managers) != 1 {
		t.Fatalf("expected exactly 1 manager, got: %d", len(managers))
	}
	if managers[0].UserID != claimant1.ID {
		t.Errorf("expected manager to be claimant1 (%s), got: %s", claimant1.ID, managers[0].UserID)
	}
}
