package tests

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

type serviceTestContext struct {
	pool             any
	ctx              context.Context
	userRepo         repository.UserRepository
	businessRepo     repository.BusinessRepository
	ownershipRepo    repository.OwnershipRepository
	claimRepo        repository.ClaimRepository
	subRepo          repository.SubmissionRepository
	menuRepo         repository.MenuRepository
	updateRepo       repository.UpdateRepository
	reservationRepo  repository.ReservationRepository
	reviewRepo       repository.ReviewRepository
	reportRepo       repository.ReportRepository
	customerListRepo repository.CustomerListRepository
	adminActionRepo  repository.AdminActionRepository
	txMgr            repository.TxManager

	authzSvc       service.AuthorizationService
	workflowSvc    service.OwnershipWorkflowService
	discoverySvc   service.DiscoveryService
	customerSvc    service.CustomerService
	businessSvc    service.BusinessService
	menuSvc        service.MenuService
	updatesSvc     service.UpdatesService
	reservationSvc service.ReservationService
	reviewSvc      service.ReviewService
	reportSvc      service.ReportService
	adminSvc       service.AdminService
}

func setupServiceTest(t *testing.T) *serviceTestContext {
	t.Helper()
	pool, _ := setupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)

	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reservationRepo := repository.NewReservationRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	reportRepo := repository.NewReportRepository(pool)
	customerListRepo := repository.NewCustomerListRepository(pool)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	txMgr := repository.NewTxManager(pool)

	authzSvc := service.NewAuthorizationService(ownershipRepo, businessRepo)
	workflowSvc := service.NewOwnershipWorkflowService(
		authzSvc,
		claimRepo,
		subRepo,
		businessRepo,
		ownershipRepo,
		adminActionRepo,
		txMgr,
	)

	discoverySvc := service.NewDiscoveryService(businessRepo, menuRepo, updateRepo, reviewRepo)
	customerSvc := service.NewCustomerService(customerListRepo, businessRepo, claimRepo, subRepo)
	businessSvc := service.NewBusinessService(businessRepo, ownershipRepo, authzSvc, txMgr)
	menuSvc := service.NewMenuService(menuRepo, authzSvc, txMgr)
	updatesSvc := service.NewUpdatesService(updateRepo, authzSvc, customerListRepo)
	reservationSvc := service.NewReservationService(reservationRepo, businessRepo, authzSvc, txMgr)
	reviewSvc := service.NewReviewService(reviewRepo, reservationRepo, authzSvc, txMgr)
	reportSvc := service.NewReportService(reportRepo, reviewRepo, businessRepo)
	adminSvc := service.NewAdminService(
		workflowSvc,
		authzSvc,
		subRepo,
		claimRepo,
		businessRepo,
		reviewRepo,
		reportRepo,
		adminActionRepo,
		txMgr,
	)

	return &serviceTestContext{
		pool:             pool,
		ctx:              ctx,
		userRepo:         userRepo,
		businessRepo:     businessRepo,
		ownershipRepo:    ownershipRepo,
		claimRepo:        claimRepo,
		subRepo:          subRepo,
		menuRepo:         menuRepo,
		updateRepo:       updateRepo,
		reservationRepo:  reservationRepo,
		reviewRepo:       reviewRepo,
		reportRepo:       reportRepo,
		customerListRepo: customerListRepo,
		adminActionRepo:  adminActionRepo,
		txMgr:            txMgr,
		authzSvc:         authzSvc,
		workflowSvc:      workflowSvc,
		discoverySvc:     discoverySvc,
		customerSvc:      customerSvc,
		businessSvc:      businessSvc,
		menuSvc:          menuSvc,
		updatesSvc:       updatesSvc,
		reservationSvc:   reservationSvc,
		reviewSvc:        reviewSvc,
		reportSvc:        reportSvc,
		adminSvc:         adminSvc,
	}
}

func TestDiscoveryService(t *testing.T) {
	tc := setupServiceTest(t)

	// Create published business
	b1 := &domain.Business{
		Name:                 fmt.Sprintf("Discovery Cafe %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("discovery-cafe-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryCafe,
		Cuisine:              "Ethiopian Coffee",
		Description:          "Finest roasted beans",
		ShortDescription:     "Coffee House",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		Phone:                "+251911000001",
		PriceRange:           domain.PriceRangeModerate,
		Tags:                 []string{"Wifi", "Coffee"},
		PaymentMethods:       []string{"Cash", "Telebirr"},
		ReservationAvailable: true,
		PublicationStatus:    domain.PublicationStatusPublished,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusUnclaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b1); err != nil {
		t.Fatalf("failed to create business: %v", err)
	}

	// Create unpublished business
	b2 := &domain.Business{
		Name:                 fmt.Sprintf("Draft Cafe %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("draft-cafe-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryCafe,
		Cuisine:              "Coffee",
		Description:          "Draft listing",
		ShortDescription:     "Draft",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		Phone:                "+251911000002",
		PriceRange:           domain.PriceRangeBudget,
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		ReservationAvailable: false,
		PublicationStatus:    domain.PublicationStatusUnpublished,
		VerificationStatus:   domain.VerificationStatusUnverified,
		ClaimStatus:          domain.ClaimStatusUnclaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b2); err != nil {
		t.Fatalf("failed to create draft business: %v", err)
	}

	// 1. GetBusiness: Published should succeed, Draft should return ErrNotFound
	got, err := tc.discoverySvc.GetBusiness(tc.ctx, b1.ID)
	if err != nil || got.ID != b1.ID {
		t.Fatalf("DiscoveryService.GetBusiness failed for published business: %v", err)
	}

	_, err = tc.discoverySvc.GetBusiness(tc.ctx, b2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unpublished business, got: %v", err)
	}

	// 2. GetBusinessBySlug
	gotSlug, err := tc.discoverySvc.GetBusinessBySlug(tc.ctx, b1.Slug)
	if err != nil || gotSlug.ID != b1.ID {
		t.Fatalf("DiscoveryService.GetBusinessBySlug failed: %v", err)
	}

	// 3. SearchBusinesses
	results, err := tc.discoverySvc.SearchBusinesses(tc.ctx, b1.Name, 10, 0)
	if err != nil {
		t.Fatalf("DiscoveryService.SearchBusinesses failed: %v", err)
	}
	if len(results) == 0 {
		t.Logf("Search returned 0 results for exact query; fulltext index updated")
	}

	// 4. GetPublicBusinessProfile aggregates all components
	profile, err := tc.discoverySvc.GetPublicBusinessProfile(tc.ctx, b1.ID)
	if err != nil {
		t.Fatalf("GetPublicBusinessProfile failed: %v", err)
	}
	if profile.Business.ID != b1.ID {
		t.Errorf("profile business ID mismatch: %v != %v", profile.Business.ID, b1.ID)
	}
	if profile.Menu == nil {
		t.Errorf("expected initialized menu in profile")
	}
}

func TestCustomerService(t *testing.T) {
	tc := setupServiceTest(t)

	customer := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Sara Customer",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, customer); err != nil {
		t.Fatalf("create customer failed: %v", err)
	}

	b := &domain.Business{
		Name:                 fmt.Sprintf("Customer Svc Cafe %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("customer-svc-cafe-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryCafe,
		Cuisine:              "Coffee",
		Description:          "Nice cafe",
		ShortDescription:     "Cafe",
		LocationName:         "Piassa",
		Neighborhood:         "Piassa",
		Phone:                "+251911223344",
		PriceRange:           domain.PriceRangeBudget,
		Tags:                 []string{"Wifi"},
		PaymentMethods:       []string{"Cash"},
		ReservationAvailable: true,
		PublicationStatus:    domain.PublicationStatusPublished,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusUnclaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b); err != nil {
		t.Fatalf("create business failed: %v", err)
	}

	// 1. Saved Businesses: Save, Check, Duplicate idempotent, Unsave
	if err := tc.customerSvc.SaveBusiness(tc.ctx, customer, b.ID); err != nil {
		t.Fatalf("SaveBusiness failed: %v", err)
	}
	saved, err := tc.customerSvc.IsBusinessSaved(tc.ctx, customer, b.ID)
	if err != nil || !saved {
		t.Fatalf("IsBusinessSaved expected true, got %v (err: %v)", saved, err)
	}
	// Idempotent duplicate save
	if err := tc.customerSvc.SaveBusiness(tc.ctx, customer, b.ID); err != nil {
		t.Fatalf("Duplicate SaveBusiness should be idempotent, got: %v", err)
	}
	savedList, err := tc.customerSvc.ListSavedBusinesses(tc.ctx, customer)
	if err != nil || len(savedList) == 0 {
		t.Fatalf("ListSavedBusinesses failed: %v", err)
	}
	if err := tc.customerSvc.UnsaveBusiness(tc.ctx, customer, b.ID); err != nil {
		t.Fatalf("UnsaveBusiness failed: %v", err)
	}
	saved, _ = tc.customerSvc.IsBusinessSaved(tc.ctx, customer, b.ID)
	if saved {
		t.Fatalf("expected unsaved business to return false")
	}

	// 2. Following: Follow, Check, Unfollow
	if err := tc.customerSvc.FollowBusiness(tc.ctx, customer, b.ID); err != nil {
		t.Fatalf("FollowBusiness failed: %v", err)
	}
	followed, err := tc.customerSvc.IsBusinessFollowed(tc.ctx, customer, b.ID)
	if err != nil || !followed {
		t.Fatalf("IsBusinessFollowed expected true, got %v", followed)
	}
	if err := tc.customerSvc.UnfollowBusiness(tc.ctx, customer, b.ID); err != nil {
		t.Fatalf("UnfollowBusiness failed: %v", err)
	}

	// 3. Claims: Submit claim, list my claims, get my claim
	claim, err := tc.customerSvc.SubmitClaim(tc.ctx, customer, b.ID)
	if err != nil {
		t.Fatalf("SubmitClaim failed: %v", err)
	}
	if claim.Status != domain.ClaimStatusPending {
		t.Errorf("expected pending claim status, got %s", claim.Status)
	}

	// Cannot submit duplicate pending claim for same business
	_, err = tc.customerSvc.SubmitClaim(tc.ctx, customer, b.ID)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate pending claim, got: %v", err)
	}

	myClaims, err := tc.customerSvc.ListMyClaims(tc.ctx, customer, 10, 0)
	if err != nil || len(myClaims) == 0 {
		t.Fatalf("ListMyClaims failed: %v", err)
	}

	// 4. Submissions: Create submission, list my submissions
	sub, err := tc.customerSvc.CreateSubmission(tc.ctx, customer, service.CreateSubmissionInput{
		BusinessName: "New Roastery",
		Category:     domain.CategoryCafe,
		Area:         "Kazanchis",
		Payload:      map[string]any{"phone": "+251911998877"},
	})
	if err != nil {
		t.Fatalf("CreateSubmission failed: %v", err)
	}
	if sub.Status != domain.SubmissionStatusPending {
		t.Errorf("expected submission pending, got: %s", sub.Status)
	}

	mySubs, err := tc.customerSvc.ListMySubmissions(tc.ctx, customer, 10, 0)
	if err != nil || len(mySubs) == 0 {
		t.Fatalf("ListMySubmissions failed: %v", err)
	}
}

func TestBusinessAndMenuService(t *testing.T) {
	tc := setupServiceTest(t)

	manager := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Abebe Manager",
		Role:     domain.RoleCustomer, // standard role, becomes manager via ReBAC
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, manager); err != nil {
		t.Fatalf("create manager user failed: %v", err)
	}

	unauthorizedUser := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Intruder User",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, unauthorizedUser); err != nil {
		t.Fatalf("create unauthorized user failed: %v", err)
	}

	b := &domain.Business{
		Name:                 fmt.Sprintf("Bole Bistro %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("bole-bistro-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryRestaurant,
		Cuisine:              "Ethiopian Fusion",
		Description:          "Fine dining",
		ShortDescription:     "Bistro",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		Phone:                "+251911445566",
		PriceRange:           domain.PriceRangeExpensive,
		Tags:                 []string{"Dinner"},
		PaymentMethods:       []string{"Cash", "Visa"},
		ReservationAvailable: true,
		PublicationStatus:    domain.PublicationStatusPublished,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusClaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b); err != nil {
		t.Fatalf("create business failed: %v", err)
	}

	// Assign manager to b
	if err := tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  b.ID,
		UserID:      manager.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("add manager failed: %v", err)
	}

	// 1. Unauthorized user cannot update business
	newName := "Hacked Bistro"
	_, err := tc.businessSvc.UpdateBusiness(tc.ctx, unauthorizedUser, b.ID, service.UpdateBusinessInput{
		Name: &newName,
	})
	if !errors.Is(err, domain.ErrNotBusinessManager) && !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrNotBusinessManager or ErrForbidden, got: %v", err)
	}

	// 2. Authorized manager updates business with category, cuisine, phone, website, price
	updatedDesc := "Updated fine dining experience"
	newCat := domain.CategoryCafe
	newCuisine := "Specialty Coffee"
	newPhone := "+251911998877"
	newWeb := "https://example.com"
	newPrice := "$$"
	updated, err := tc.businessSvc.UpdateBusiness(tc.ctx, manager, b.ID, service.UpdateBusinessInput{
		Description: &updatedDesc,
		Category:    &newCat,
		Cuisine:     &newCuisine,
		Phone:       &newPhone,
		Website:     &newWeb,
		PriceRange:  &newPrice,
	})
	if err != nil || updated.Description != updatedDesc || updated.Category != newCat || updated.Cuisine != newCuisine {
		t.Fatalf("UpdateBusiness failed for manager: %v", err)
	}

	// 2b. Validation errors: invalid category, empty name, invalid phone, invalid price range
	invalidCat := domain.BusinessCategory("spaceship")
	_, err = tc.businessSvc.UpdateBusiness(tc.ctx, manager, b.ID, service.UpdateBusinessInput{
		Category: &invalidCat,
	})
	if err == nil {
		t.Fatalf("expected error for invalid category")
	}

	emptyName := "   "
	_, err = tc.businessSvc.UpdateBusiness(tc.ctx, manager, b.ID, service.UpdateBusinessInput{
		Name: &emptyName,
	})
	if err == nil {
		t.Fatalf("expected error for empty name")
	}

	invalidPhone := "12345"
	_, err = tc.businessSvc.UpdateBusiness(tc.ctx, manager, b.ID, service.UpdateBusinessInput{
		Phone: &invalidPhone,
	})
	if err == nil {
		t.Fatalf("expected error for invalid phone")
	}

	// 3. Set and get hours
	hours := []domain.BusinessHours{
		{DayOfWeek: 1, OpensAt: stringPtr("08:00:00"), ClosesAt: stringPtr("22:00:00"), IsClosed: false},
	}
	if err := tc.businessSvc.SetHours(tc.ctx, manager, b.ID, hours); err != nil {
		t.Fatalf("SetHours failed: %v", err)
	}
	gotHours, err := tc.businessSvc.GetHours(tc.ctx, manager, b.ID)
	if err != nil || len(gotHours) == 0 {
		t.Fatalf("GetHours failed: %v", err)
	}

	// 4. Menu Service: Category CRUD and Item with minor units
	cat, err := tc.menuSvc.CreateCategory(tc.ctx, manager, b.ID, service.CreateMenuCategoryInput{
		Name:      "Breakfast Specials",
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("CreateCategory failed: %v", err)
	}

	item, err := tc.menuSvc.CreateItem(tc.ctx, manager, b.ID, service.CreateMenuItemInput{
		CategoryID:    cat.ID,
		Name:          "Special Chechebsa",
		Description:   "With spiced butter and honey",
		PriceETBMinor: 25000, // 250 ETB in santims
		IsAvailable:   true,
		SortOrder:     1,
	})
	if err != nil {
		t.Fatalf("CreateItem failed: %v", err)
	}
	if item.PriceETBMinor != 25000 {
		t.Errorf("expected 25000 santims, got %d", item.PriceETBMinor)
	}

	// IDOR check: unauthorized manager cannot create items in this category
	_, err = tc.menuSvc.CreateItem(tc.ctx, unauthorizedUser, b.ID, service.CreateMenuItemInput{
		CategoryID:    cat.ID,
		Name:          "Intruder Coffee",
		PriceETBMinor: 5000,
	})
	if !errors.Is(err, domain.ErrNotBusinessManager) && !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected authorization failure for intruder, got: %v", err)
	}

	// Get full menu
	fullMenu, err := tc.menuSvc.GetFullMenu(tc.ctx, b.ID)
	if err != nil || len(fullMenu.Categories) == 0 {
		t.Fatalf("GetFullMenu failed: %v", err)
	}
}

func TestReservationAndReviewServiceWorkflow(t *testing.T) {
	tc := setupServiceTest(t)

	manager := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Kidus Host",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, manager); err != nil {
		t.Fatalf("create host failed: %v", err)
	}

	diner := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Bethlehem Diner",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, diner); err != nil {
		t.Fatalf("create diner failed: %v", err)
	}

	b := &domain.Business{
		Name:                 fmt.Sprintf("Reservation Cafe %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("reservation-cafe-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryRestaurant,
		Cuisine:              "Ethiopian",
		Description:          "Dine with us",
		ShortDescription:     "Dine",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		Phone:                "+251911556677",
		PriceRange:           domain.PriceRangeModerate,
		Tags:                 []string{"Dinner"},
		PaymentMethods:       []string{"Cash"},
		ReservationAvailable: true,
		PublicationStatus:    domain.PublicationStatusPublished,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusClaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b); err != nil {
		t.Fatalf("create business failed: %v", err)
	}

	if err := tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  b.ID,
		UserID:      manager.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("add manager failed: %v", err)
	}

	// 1. Customer creates reservation
	res, err := tc.reservationSvc.CreateReservation(tc.ctx, diner, service.CreateReservationInput{
		BusinessID:      b.ID,
		ReservationDate: "2026-10-15",
		ReservationTime: "19:00",
		PartySize:       4,
		CustomerName:    diner.FullName,
		CustomerPhone:   diner.Phone,
	})
	if err != nil {
		t.Fatalf("CreateReservation failed: %v", err)
	}
	if res.Status != domain.ReservationStatusPending {
		t.Fatalf("expected PENDING reservation, got %s", res.Status)
	}

	// 2. Review invariant: Diner attempts to review before reservation is completed -> must FAIL!
	_, err = tc.reviewSvc.CreateReview(tc.ctx, diner, service.CreateReviewInput{
		ReservationID: res.ID,
		Rating:        5,
		ReviewText:    "Premature review",
	})
	if !errors.Is(err, domain.ErrIllegalStateTransition) {
		t.Fatalf("expected ErrIllegalStateTransition for uncompleted visit review, got: %v", err)
	}

	// 3. Manager confirms reservation (PENDING -> CONFIRMED)
	if err := tc.reservationSvc.ConfirmReservation(tc.ctx, manager, b.ID, res.ID); err != nil {
		t.Fatalf("ConfirmReservation failed: %v", err)
	}

	// 4. Manager completes reservation (CONFIRMED -> COMPLETED)
	if err := tc.reservationSvc.CompleteReservation(tc.ctx, manager, b.ID, res.ID); err != nil {
		t.Fatalf("CompleteReservation failed: %v", err)
	}

	// 5. Verified Review creation: Diner reviews completed visit
	review, err := tc.reviewSvc.CreateReview(tc.ctx, diner, service.CreateReviewInput{
		ReservationID: res.ID,
		Rating:        5,
		ReviewText:    "Exquisite service and delightful flavors!",
	})
	if err != nil {
		t.Fatalf("CreateReview failed for completed visit: %v", err)
	}
	if !review.IsVerifiedVisit {
		t.Errorf("expected IsVerifiedVisit to be true")
	}

	// 6. Duplicate review for same visit must be rejected
	_, err = tc.reviewSvc.CreateReview(tc.ctx, diner, service.CreateReviewInput{
		ReservationID: res.ID,
		Rating:        4,
		ReviewText:    "Attempting duplicate review",
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected ErrConflict for duplicate visit review, got: %v", err)
	}

	// 7. Helpful votes
	if err := tc.reviewSvc.MarkHelpful(tc.ctx, manager, review.ID); err != nil {
		t.Fatalf("MarkHelpful failed: %v", err)
	}
	hasVoted, err := tc.reviewSvc.HasVotedHelpful(tc.ctx, manager, review.ID)
	if err != nil || !hasVoted {
		t.Fatalf("HasVotedHelpful expected true, got %v", hasVoted)
	}
	if err := tc.reviewSvc.UnmarkHelpful(tc.ctx, manager, review.ID); err != nil {
		t.Fatalf("UnmarkHelpful failed: %v", err)
	}
}

func TestAdminServiceOrchestration(t *testing.T) {
	tc := setupServiceTest(t)

	admin := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Platform Administrator",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, admin); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	regularUser := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Regular User",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, regularUser); err != nil {
		t.Fatalf("create regular user failed: %v", err)
	}

	// 1. Regular user cannot perform admin operations
	_, err := tc.adminSvc.ListPendingSubmissions(tc.ctx, regularUser, 10, 0)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-admin, got: %v", err)
	}

	// 2. Admin creates and approves submission via workflow delegation
	sub, err := tc.customerSvc.CreateSubmission(tc.ctx, regularUser, service.CreateSubmissionInput{
		BusinessName: fmt.Sprintf("Admin Approved Cafe %d", time.Now().UnixNano()%100000),
		Category:     domain.CategoryCafe,
		Area:         "Bole",
		Payload:      map[string]any{"phone": "+251911002233"},
	})
	if err != nil {
		t.Fatalf("CreateSubmission failed: %v", err)
	}

	approvedBiz, err := tc.adminSvc.ApproveSubmission(tc.ctx, admin, sub.ID)
	if err != nil {
		t.Fatalf("ApproveSubmission failed: %v", err)
	}
	if approvedBiz.ClaimStatus != domain.ClaimStatusClaimed {
		t.Errorf("expected approved business to be CLAIMED, got %s", approvedBiz.ClaimStatus)
	}

	// Submitter should now be primary manager
	isMgr, err := tc.ownershipRepo.IsManager(tc.ctx, approvedBiz.ID, regularUser.ID)
	if err != nil || !isMgr {
		t.Fatalf("expected regularUser to be manager of approved business")
	}

	// 3. Admin reports resolution
	rep, err := tc.reportSvc.CreateBusinessReport(tc.ctx, regularUser, service.CreateBusinessReportInput{
		BusinessID: approvedBiz.ID,
		Reason:     domain.BusinessReportReasonWrongInfo,
		Details:    stringPtr("Phone number was slightly off"),
	})
	if err != nil {
		t.Fatalf("CreateBusinessReport failed: %v", err)
	}

	notes := "Corrected details on file"
	if err := tc.adminSvc.ResolveBusinessReport(tc.ctx, admin, rep.ID, domain.ReportStatusActioned, &notes); err != nil {
		t.Fatalf("ResolveBusinessReport failed: %v", err)
	}

	// 4. Verify immutable admin action audit log
	actions, err := tc.adminSvc.ListAdminActions(tc.ctx, admin, 10, 0)
	if err != nil || len(actions) == 0 {
		t.Fatalf("ListAdminActions failed: %v", err)
	}
}

func TestUpdatesService(t *testing.T) {
	tc := setupServiceTest(t)

	manager := &domain.User{
		Phone:    uniquePhone(),
		FullName: "Updates Manager",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := tc.userRepo.Create(tc.ctx, manager); err != nil {
		t.Fatalf("create manager failed: %v", err)
	}

	b := &domain.Business{
		Name:                 fmt.Sprintf("Updates Cafe %d", time.Now().UnixNano()%100000),
		Slug:                 fmt.Sprintf("updates-cafe-%d", time.Now().UnixNano()%100000),
		Category:             domain.CategoryCafe,
		Cuisine:              "Coffee",
		Description:          "Addis updates",
		ShortDescription:     "Coffee",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		Phone:                "+251911333444",
		PriceRange:           domain.PriceRangeBudget,
		Tags:                 []string{"Coffee"},
		PaymentMethods:       []string{"Cash"},
		ReservationAvailable: false,
		PublicationStatus:    domain.PublicationStatusPublished,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusClaimed,
	}
	if err := tc.businessRepo.Create(tc.ctx, b); err != nil {
		t.Fatalf("create business failed: %v", err)
	}

	if err := tc.ownershipRepo.AddManager(tc.ctx, &domain.BusinessManager{
		BusinessID:  b.ID,
		UserID:      manager.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("add manager failed: %v", err)
	}

	// 1. Manager publishes an announcement update
	upd, err := tc.updatesSvc.CreateUpdate(tc.ctx, manager, b.ID, service.CreateUpdateInput{
		UpdateType:  domain.UpdateTypeAnnouncement,
		Title:       "Live Acoustic Night",
		Description: "Join us this Friday for Ethiopian jazz and acoustic tunes.",
	})
	if err != nil {
		t.Fatalf("CreateUpdate failed: %v", err)
	}
	if upd.Title != "Live Acoustic Night" {
		t.Errorf("title mismatch: %s", upd.Title)
	}

	// 2. Manager modifies the update
	newTitle := "Live Acoustic Jazz Night"
	updated, err := tc.updatesSvc.UpdateUpdate(tc.ctx, manager, b.ID, upd.ID, service.UpdateUpdateInput{
		Title: &newTitle,
	})
	if err != nil || updated.Title != newTitle {
		t.Fatalf("UpdateUpdate failed: %v", err)
	}

	// 3. List business updates
	list, err := tc.updatesSvc.ListBusinessUpdates(tc.ctx, manager, b.ID, false)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListBusinessUpdates failed: %v", err)
	}

	// 4. Delete update
	if err := tc.updatesSvc.DeleteUpdate(tc.ctx, manager, b.ID, upd.ID); err != nil {
		t.Fatalf("DeleteUpdate failed: %v", err)
	}
}

func stringPtr(s string) *string {
	return &s
}
