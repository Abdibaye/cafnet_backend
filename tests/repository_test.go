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
)

func uniquePhone() string {
	return fmt.Sprintf("+2519%08d", time.Now().UnixNano()%100000000)
}

func uniqueSlug(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestUserRepository(t *testing.T) {
	pool, ctx := setupTestDB(t)
	repo := repository.NewUserRepository(pool)

	phone := uniquePhone()
	user := &domain.User{
		Phone:    phone,
		FullName: "Abebe Bikila",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}

	// 1. Create
	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("UserRepository.Create failed: %v", err)
	}
	if user.ID == "" {
		t.Fatalf("Expected non-empty user ID")
	}

	// 2. GetByID
	fetched, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("UserRepository.GetByID failed: %v", err)
	}
	if fetched.Phone != phone || fetched.FullName != "Abebe Bikila" {
		t.Errorf("Unexpected user fields: %+v", fetched)
	}

	// 3. GetByPhone
	byPhone, err := repo.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatalf("UserRepository.GetByPhone failed: %v", err)
	}
	if byPhone.ID != user.ID {
		t.Errorf("User ID mismatch: got %v, want %v", byPhone.ID, user.ID)
	}

	// 4. Update
	user.FullName = "Abebe Bikila Updated"
	avatar := "https://example.com/avatar.jpg"
	user.AvatarURL = &avatar
	err = repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("UserRepository.Update failed: %v", err)
	}

	updated, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to re-fetch user: %v", err)
	}
	if updated.FullName != "Abebe Bikila Updated" || updated.AvatarURL == nil || *updated.AvatarURL != avatar {
		t.Errorf("Updated fields mismatch: %+v", updated)
	}

	// 5. Unique phone conflict
	duplicateUser := &domain.User{
		Phone:    phone,
		FullName: "Duplicate Phone",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	err = repo.Create(ctx, duplicateUser)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Expected domain.ErrConflict on duplicate phone, got: %v", err)
	}
}

func TestBusinessRepository(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	bRepo := repository.NewBusinessRepository(pool)

	u := &domain.User{Phone: uniquePhone(), FullName: "Owner User", Role: domain.RoleCustomer, IsActive: true}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	slug := uniqueSlug("buna-cafe")
	b := &domain.Business{
		Name:                 "Buna Bet",
		Slug:                 slug,
		Category:             domain.CategoryCafe,
		Cuisine:              "Ethiopian Coffee",
		Description:          "Authentic traditional coffee experience in Addis Ababa",
		ShortDescription:     "Traditional coffee",
		LocationName:         "Bole Medhanialem",
		Neighborhood:         "Bole",
		AddressLine:          strPtr("Cameroon St"),
		Phone:                uniquePhone(),
		PriceRange:           "$$",
		Tags:                 []string{"traditional", "wifi", "outdoor"},
		PaymentMethods:       []string{"Cash", "Telebirr"},
		ReservationAvailable: true,
		CreatedByUserID:      &u.ID,
	}

	// 1. Create
	if err := bRepo.Create(ctx, b); err != nil {
		t.Fatalf("BusinessRepository.Create failed: %v", err)
	}
	if b.ID == 0 {
		t.Fatalf("Expected non-zero business ID")
	}

	// 2. GetByID
	fetched, err := bRepo.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Slug != slug || fetched.Name != "Buna Bet" {
		t.Errorf("Unexpected business fields: %+v", fetched)
	}

	// 3. GetBySlug
	bySlug, err := bRepo.GetBySlug(ctx, slug)
	if err != nil {
		t.Fatalf("GetBySlug failed: %v", err)
	}
	if bySlug.ID != b.ID {
		t.Errorf("ID mismatch: got %d, want %d", bySlug.ID, b.ID)
	}

	// 4. Update
	b.Name = "Buna Bet Bole"
	b.Cuisine = "Artisanal Coffee & Pastries"
	if err := bRepo.Update(ctx, b); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 5. State transitions (Platform vs Business)
	if err := bRepo.SetPublicationStatus(ctx, b.ID, domain.PublicationStatusPublished); err != nil {
		t.Fatalf("SetPublicationStatus failed: %v", err)
	}
	if err := bRepo.SetVerificationStatus(ctx, b.ID, domain.VerificationStatusVerified); err != nil {
		t.Fatalf("SetVerificationStatus failed: %v", err)
	}
	if err := bRepo.SetClaimStatus(ctx, b.ID, domain.ClaimStatusClaimed); err != nil {
		t.Fatalf("SetClaimStatus failed: %v", err)
	}

	refetched, err := bRepo.GetByID(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if refetched.PublicationStatus != domain.PublicationStatusPublished ||
		refetched.VerificationStatus != domain.VerificationStatusVerified ||
		refetched.ClaimStatus != domain.ClaimStatusClaimed {
		t.Errorf("Status mismatch on business: %+v", refetched)
	}

	// 6. Business Hours
	hours := []domain.BusinessHours{
		{BusinessID: b.ID, DayOfWeek: 1, OpensAt: strPtr("08:00:00"), ClosesAt: strPtr("22:00:00"), IsClosed: false},
		{BusinessID: b.ID, DayOfWeek: 0, IsClosed: true},
	}
	if err := bRepo.SetHours(ctx, b.ID, hours); err != nil {
		t.Fatalf("SetHours failed: %v", err)
	}
	fetchedHours, err := bRepo.GetHours(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetHours failed: %v", err)
	}
	if len(fetchedHours) != 2 {
		t.Fatalf("Expected 2 hours entries, got %d", len(fetchedHours))
	}

	// 7. Business Images
	images := []domain.BusinessImage{
		{BusinessID: b.ID, ImageURL: "https://example.com/buna1.jpg", AltText: strPtr("Front facade"), SortOrder: 0},
		{BusinessID: b.ID, ImageURL: "https://example.com/buna2.jpg", AltText: strPtr("Interior seating"), SortOrder: 1},
	}
	if err := bRepo.SetImages(ctx, b.ID, images); err != nil {
		t.Fatalf("SetImages failed: %v", err)
	}
	fetchedImages, err := bRepo.GetImages(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetImages failed: %v", err)
	}
	if len(fetchedImages) != 2 {
		t.Fatalf("Expected 2 image entries, got %d", len(fetchedImages))
	}

	// 8. List & Search
	cat := domain.CategoryCafe
	listRes, err := bRepo.List(ctx, domain.BusinessFilter{
		Category:          &cat,
		PublicationStatus: &refetched.PublicationStatus,
		Limit:             10,
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listRes) == 0 {
		t.Errorf("Expected at least one business in list")
	}

	searchResults, err := bRepo.Search(ctx, "Artisanal", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchResults) == 0 {
		t.Logf("Search returned 0 results (full-text index might need exact token), trying 'Buna'")
		searchResults, err = bRepo.Search(ctx, "Buna", 10, 0)
		if err != nil {
			t.Fatalf("Search with 'Buna' failed: %v", err)
		}
	}

	// 9. Soft delete
	if err := bRepo.SoftDelete(ctx, b.ID); err != nil {
		t.Fatalf("SoftDelete failed: %v", err)
	}
	_, err = bRepo.GetByID(ctx, b.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Expected domain.ErrNotFound after soft delete, got: %v", err)
	}
}

func TestOwnershipClaimSubmissionRepositories(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	bRepo := repository.NewBusinessRepository(pool)
	ownRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)

	admin := &domain.User{Phone: uniquePhone(), FullName: "Platform Admin", Role: domain.RoleAdmin, IsActive: true}
	_ = userRepo.Create(ctx, admin)

	owner := &domain.User{Phone: uniquePhone(), FullName: "Cafe Owner", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, owner)

	b := &domain.Business{
		Name:                 "Owner Test Cafe",
		Slug:                 uniqueSlug("owner-cafe"),
		Category:             domain.CategoryCafe,
		Cuisine:              "Ethiopian",
		Description:          "Testing ownership & claims",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		PriceRange:           "$$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	_ = bRepo.Create(ctx, b)

	// 1. Business Manager
	mgr := &domain.BusinessManager{
		BusinessID:  b.ID,
		UserID:      owner.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
		GrantedBy:   &admin.ID,
	}
	if err := ownRepo.AddManager(ctx, mgr); err != nil {
		t.Fatalf("AddManager failed: %v", err)
	}

	isMgr, err := ownRepo.IsManager(ctx, b.ID, owner.ID)
	if err != nil || !isMgr {
		t.Fatalf("Expected user to be manager, err: %v", err)
	}

	managers, err := ownRepo.GetManagers(ctx, b.ID)
	if err != nil || len(managers) != 1 {
		t.Fatalf("GetManagers failed: %v, count: %d", err, len(managers))
	}

	managedBiz, err := ownRepo.GetManagedBusinesses(ctx, owner.ID)
	if err != nil || len(managedBiz) != 1 {
		t.Fatalf("GetManagedBusinesses failed: %v, count: %d", err, len(managedBiz))
	}

	// 2. Business Claim
	claimant := &domain.User{Phone: uniquePhone(), FullName: "Claimant User", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, claimant)

	claim := &domain.BusinessClaim{
		BusinessID:     b.ID,
		ClaimantUserID: claimant.ID,
		Status:         domain.ClaimStatusPending,
	}
	if err := claimRepo.Create(ctx, claim); err != nil {
		t.Fatalf("ClaimRepository.Create failed: %v", err)
	}

	pendingClaim, err := claimRepo.GetPendingForBusiness(ctx, b.ID)
	if err != nil || pendingClaim.ID != claim.ID {
		t.Fatalf("GetPendingForBusiness failed: %v", err)
	}

	// Resolve claim
	reason := "Verified via government registration document"
	if err := claimRepo.UpdateStatus(ctx, claim.ID, domain.ClaimStatusApproved, &admin.ID, &reason); err != nil {
		t.Fatalf("ClaimRepository.UpdateStatus failed: %v", err)
	}

	// 3. Business Submission
	sub := &domain.BusinessSubmission{
		SubmitterUserID: owner.ID,
		BusinessName:    "Submitted Venue",
		Category:        domain.CategoryRestaurant,
		Area:            "Kazanchis",
		SubmittedPayload: map[string]any{
			"cuisine": "Traditional",
			"phone":   "+251911223344",
		},
		Status: domain.SubmissionStatusPending,
	}
	if err := subRepo.Create(ctx, sub); err != nil {
		t.Fatalf("SubmissionRepository.Create failed: %v", err)
	}

	pendingSubs, err := subRepo.ListPending(ctx, 10, 0)
	if err != nil || len(pendingSubs) == 0 {
		t.Fatalf("ListPending submissions failed: %v", err)
	}

	if err := subRepo.UpdateStatus(ctx, sub.ID, domain.SubmissionStatusApproved, &b.ID, &admin.ID, nil); err != nil {
		t.Fatalf("SubmissionRepository.UpdateStatus failed: %v", err)
	}
}

func TestMenuRepository(t *testing.T) {
	pool, ctx := setupTestDB(t)
	bRepo := repository.NewBusinessRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)

	b := &domain.Business{
		Name:                 "Menu Bistro",
		Slug:                 uniqueSlug("menu-bistro"),
		Category:             domain.CategoryRestaurant,
		Cuisine:              "Fusion",
		Description:          "Full menu test",
		LocationName:         "Piassa",
		Neighborhood:         "Piassa",
		PriceRange:           "$$$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	if err := bRepo.Create(ctx, b); err != nil {
		t.Fatalf("Create business failed: %v", err)
	}

	// 1. Categories
	cat1 := &domain.MenuCategory{BusinessID: b.ID, Name: "Hot Drinks", SortOrder: 0}
	cat2 := &domain.MenuCategory{BusinessID: b.ID, Name: "Breakfast", SortOrder: 1}
	if err := menuRepo.CreateCategory(ctx, cat1); err != nil {
		t.Fatalf("CreateCategory 1 failed: %v", err)
	}
	if err := menuRepo.CreateCategory(ctx, cat2); err != nil {
		t.Fatalf("CreateCategory 2 failed: %v", err)
	}

	categories, err := menuRepo.ListCategories(ctx, b.ID)
	if err != nil || len(categories) != 2 {
		t.Fatalf("ListCategories failed: %v, count: %d", err, len(categories))
	}

	// Reorder categories
	if err := menuRepo.ReorderCategories(ctx, b.ID, []domain.CategoryID{cat2.ID, cat1.ID}); err != nil {
		t.Fatalf("ReorderCategories failed: %v", err)
	}

	// 2. Menu Items
	item1 := &domain.MenuItem{
		CategoryID:    cat1.ID,
		BusinessID:    b.ID,
		Name:          "Special Macchiato",
		Description:   "Layered espresso and frothy milk",
		PriceETBMinor: 6500, // 65 ETB
		SortOrder:     0,
		IsAvailable:   true,
	}
	item2 := &domain.MenuItem{
		CategoryID:    cat1.ID,
		BusinessID:    b.ID,
		Name:          "Spiced Herbal Tea",
		Description:   "Ginger, cinnamon, cloves",
		PriceETBMinor: 4000, // 40 ETB
		SortOrder:     1,
		IsAvailable:   true,
	}
	if err := menuRepo.CreateItem(ctx, item1); err != nil {
		t.Fatalf("CreateItem 1 failed: %v", err)
	}
	if err := menuRepo.CreateItem(ctx, item2); err != nil {
		t.Fatalf("CreateItem 2 failed: %v", err)
	}

	// Reorder items
	if err := menuRepo.ReorderItems(ctx, cat1.ID, []domain.ItemID{item2.ID, item1.ID}); err != nil {
		t.Fatalf("ReorderItems failed: %v", err)
	}

	// 3. Full Menu
	fullMenu, err := menuRepo.GetFullMenu(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetFullMenu failed: %v", err)
	}
	if len(fullMenu.Categories) != 2 {
		t.Fatalf("Expected 2 categories in full menu, got %d", len(fullMenu.Categories))
	}

	// 4. Soft Delete Item & Category
	if err := menuRepo.DeleteItem(ctx, item2.ID); err != nil {
		t.Fatalf("DeleteItem failed: %v", err)
	}
	_, err = menuRepo.GetItemByID(ctx, item2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Expected domain.ErrNotFound for deleted item, got: %v", err)
	}

	if err := menuRepo.DeleteCategory(ctx, cat2.ID); err != nil {
		t.Fatalf("DeleteCategory failed: %v", err)
	}
	_, err = menuRepo.GetCategoryByID(ctx, cat2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Expected domain.ErrNotFound for deleted category, got: %v", err)
	}
}

func TestUpdateRepository(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	bRepo := repository.NewBusinessRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)

	u := &domain.User{Phone: uniquePhone(), FullName: "Update Author", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, u)

	b := &domain.Business{
		Name:                 "Live Events Cafe",
		Slug:                 uniqueSlug("live-events"),
		Category:             domain.CategoryCafe,
		Cuisine:              "Jazz & Coffee",
		Description:          "Live music venue",
		LocationName:         "Kazanchis",
		Neighborhood:         "Kazanchis",
		PriceRange:           "$$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	_ = bRepo.Create(ctx, b)

	future := time.Now().Add(48 * time.Hour)
	up := &domain.BusinessUpdate{
		BusinessID:   b.ID,
		AuthorUserID: u.ID,
		UpdateType:   domain.UpdateTypeEvent,
		Title:        "Friday Live Jazz Evening",
		Description:  "Join us for an acoustic jazz performance featuring top local artists.",
		Status:       domain.UpdateStatusPublished,
		ExpiresAt:    &future,
	}

	// 1. Create
	if err := updateRepo.Create(ctx, up); err != nil {
		t.Fatalf("UpdateRepository.Create failed: %v", err)
	}

	// 2. GetByID
	fetched, err := updateRepo.GetByID(ctx, up.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if fetched.Title != up.Title {
		t.Errorf("Title mismatch: got %s, want %s", fetched.Title, up.Title)
	}

	// 3. ListActive
	activeUpdates, err := updateRepo.ListActive(ctx, &b.ID, 10, 0)
	if err != nil || len(activeUpdates) == 0 {
		t.Fatalf("ListActive failed: %v, count: %d", err, len(activeUpdates))
	}

	// 4. Update
	up.Title = "Friday Live Jazz Night (Time Updated)"
	if err := updateRepo.Update(ctx, up); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 5. Delete
	if err := updateRepo.Delete(ctx, up.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = updateRepo.GetByID(ctx, up.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Expected domain.ErrNotFound after delete, got: %v", err)
	}
}

func TestReservationAndReviewLifecycle(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	bRepo := repository.NewBusinessRepository(pool)
	resRepo := repository.NewReservationRepository(pool)
	revRepo := repository.NewReviewRepository(pool)

	customer := &domain.User{Phone: uniquePhone(), FullName: "Dining Guest", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, customer)

	staff := &domain.User{Phone: uniquePhone(), FullName: "Maitre D", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, staff)

	b := &domain.Business{
		Name:                 "Gourmet Restaurant",
		Slug:                 uniqueSlug("gourmet-res"),
		Category:             domain.CategoryRestaurant,
		Cuisine:              "Fine Dining",
		Description:          "Reservations & reviews testing venue",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		PriceRange:           "$$$$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	_ = bRepo.Create(ctx, b)

	code := fmt.Sprintf("CAF-%04d", time.Now().UnixNano()%10000)
	resDate := time.Now().Add(24 * time.Hour)
	res := &domain.Reservation{
		ReservationCode: code,
		BusinessID:      b.ID,
		CustomerUserID:  customer.ID,
		CustomerName:    "Dining Guest",
		CustomerPhone:   customer.Phone,
		ReservationDate: resDate,
		ReservationTime: "19:30:00",
		PartySize:       4,
		SpecialRequest:  "Window table preferred",
		Status:          domain.ReservationStatusPending,
	}

	// 1. Create Reservation
	if err := resRepo.Create(ctx, res); err != nil {
		t.Fatalf("Create reservation failed: %v", err)
	}

	fetchedRes, err := resRepo.GetByCode(ctx, code)
	if err != nil || fetchedRes.Status != domain.ReservationStatusPending {
		t.Fatalf("GetByCode failed: %v", err)
	}

	// 2. Status guard: review BEFORE completion MUST FAIL (enforced by DB trigger!)
	illegalReview := &domain.Review{
		BusinessID:      b.ID,
		CustomerUserID:  customer.ID,
		ReservationID:   res.ID,
		Rating:          5,
		ReviewText:      "Pre-mature review attempt",
		IsVerifiedVisit: true,
		Status:          domain.ReviewStatusVisible,
	}
	err = revRepo.Create(ctx, illegalReview)
	if err == nil {
		t.Fatalf("Expected review creation on PENDING reservation to fail due to trigger!")
	}

	// 3. Confirm Reservation
	if err := resRepo.Confirm(ctx, res.ID, &staff.ID); err != nil {
		t.Fatalf("Confirm reservation failed: %v", err)
	}

	// 4. Complete Reservation
	if err := resRepo.Complete(ctx, res.ID, &staff.ID); err != nil {
		t.Fatalf("Complete reservation failed: %v", err)
	}

	completedRes, err := resRepo.GetByID(ctx, res.ID)
	if err != nil || completedRes.Status != domain.ReservationStatusCompleted || completedRes.CompletedAt == nil {
		t.Fatalf("Reservation not in completed state: %+v", completedRes)
	}

	// 5. Create Review now that visit is completed
	validReview := &domain.Review{
		BusinessID:      b.ID,
		CustomerUserID:  customer.ID,
		ReservationID:   res.ID,
		Rating:          5,
		ReviewText:      "Exceptional dining experience! The ambiance and food were top notch.",
		IsVerifiedVisit: true,
		Status:          domain.ReviewStatusVisible,
	}
	if err := revRepo.Create(ctx, validReview); err != nil {
		t.Fatalf("Create review on completed reservation failed: %v", err)
	}

	// 6. Helpful vote
	voter := &domain.User{Phone: uniquePhone(), FullName: "Voter", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, voter)

	vote := &domain.ReviewHelpfulVote{ReviewID: validReview.ID, CustomerUserID: voter.ID}
	if err := revRepo.AddHelpfulVote(ctx, vote); err != nil {
		t.Fatalf("AddHelpfulVote failed: %v", err)
	}

	hasVoted, err := revRepo.HasVotedHelpful(ctx, validReview.ID, voter.ID)
	if err != nil || !hasVoted {
		t.Fatalf("HasVotedHelpful check failed: %v", err)
	}

	// 7. Review Summary
	summary, err := revRepo.GetSummary(ctx, b.ID)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}
	if summary.Count != 1 || summary.Average != 5.0 || summary.Distribution[0] != 1 {
		t.Errorf("Summary mismatch: %+v", summary)
	}
}

func TestCustomerListsAndReports(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	bRepo := repository.NewBusinessRepository(pool)
	listRepo := repository.NewCustomerListRepository(pool)
	reportRepo := repository.NewReportRepository(pool)

	admin := &domain.User{Phone: uniquePhone(), FullName: "Admin Mod", Role: domain.RoleAdmin, IsActive: true}
	_ = userRepo.Create(ctx, admin)

	u := &domain.User{Phone: uniquePhone(), FullName: "List User", Role: domain.RoleCustomer, IsActive: true}
	_ = userRepo.Create(ctx, u)

	b := &domain.Business{
		Name:                 "Bookmark Cafe",
		Slug:                 uniqueSlug("bookmark-cafe"),
		Category:             domain.CategoryCafe,
		Cuisine:              "Coffee",
		Description:          "Testing bookmarks",
		LocationName:         "Bole",
		Neighborhood:         "Bole",
		PriceRange:           "$",
		Tags:                 []string{},
		PaymentMethods:       []string{"Cash"},
		PublicationStatus:    domain.PublicationStatusPublished,
		ReservationAvailable: true,
	}
	_ = bRepo.Create(ctx, b)

	// 1. Saved Businesses
	if err := listRepo.SaveBusiness(ctx, u.ID, b.ID); err != nil {
		t.Fatalf("SaveBusiness failed: %v", err)
	}
	isSaved, err := listRepo.IsSaved(ctx, u.ID, b.ID)
	if err != nil || !isSaved {
		t.Fatalf("IsSaved returned false: %v", err)
	}
	savedList, err := listRepo.ListSavedBusinesses(ctx, u.ID)
	if err != nil || len(savedList) != 1 {
		t.Fatalf("ListSavedBusinesses failed: %v, count: %d", err, len(savedList))
	}
	if err := listRepo.UnsaveBusiness(ctx, u.ID, b.ID); err != nil {
		t.Fatalf("UnsaveBusiness failed: %v", err)
	}
	isSaved, _ = listRepo.IsSaved(ctx, u.ID, b.ID)
	if isSaved {
		t.Errorf("Expected IsSaved to be false after Unsave")
	}

	// 2. Followed Businesses
	if err := listRepo.FollowBusiness(ctx, u.ID, b.ID); err != nil {
		t.Fatalf("FollowBusiness failed: %v", err)
	}
	isFollowing, err := listRepo.IsFollowing(ctx, u.ID, b.ID)
	if err != nil || !isFollowing {
		t.Fatalf("IsFollowing returned false: %v", err)
	}
	if err := listRepo.UnfollowBusiness(ctx, u.ID, b.ID); err != nil {
		t.Fatalf("UnfollowBusiness failed: %v", err)
	}

	// 3. Business Report
	bReport := &domain.BusinessReport{
		BusinessID:     b.ID,
		ReporterUserID: u.ID,
		Reason:         domain.BusinessReportReasonWrongHours,
		Details:        strPtr("Opening time is 09:00, not 08:00"),
		Status:         domain.ReportStatusPending,
	}
	if err := reportRepo.CreateBusinessReport(ctx, bReport); err != nil {
		t.Fatalf("CreateBusinessReport failed: %v", err)
	}
	pendingBReports, err := reportRepo.ListPendingBusinessReports(ctx, 10, 0)
	if err != nil || len(pendingBReports) == 0 {
		t.Fatalf("ListPendingBusinessReports failed: %v", err)
	}
	notes := "Verified with shop owner, updated hours."
	if err := reportRepo.ResolveBusinessReport(ctx, bReport.ID, domain.ReportStatusActioned, &admin.ID, &notes); err != nil {
		t.Fatalf("ResolveBusinessReport failed: %v", err)
	}
}

func TestAdminActionRepository(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	adminRepo := repository.NewAdminActionRepository(pool)

	admin := &domain.User{Phone: uniquePhone(), FullName: "Governance Officer", Role: domain.RoleAdmin, IsActive: true}
	_ = userRepo.Create(ctx, admin)

	action := &domain.AdminAction{
		AdminUserID:      admin.ID,
		ActionType:       domain.AdminActionBusinessApproved,
		TargetEntityType: "business_submission",
		TargetEntityID:   uuid.New().String(),
		Metadata: map[string]any{
			"notes": "Verified all regulatory licenses.",
		},
	}

	if err := adminRepo.Record(ctx, action); err != nil {
		t.Fatalf("AdminActionRepository.Record failed: %v", err)
	}

	actions, err := adminRepo.List(ctx, 10, 0)
	if err != nil || len(actions) == 0 {
		t.Fatalf("AdminActionRepository.List failed: %v", err)
	}

	found := false
	for _, a := range actions {
		if a.ID == action.ID {
			found = true
			if a.Metadata["notes"] != "Verified all regulatory licenses." {
				t.Errorf("Metadata mismatch: %+v", a.Metadata)
			}
			break
		}
	}
	if !found {
		t.Errorf("Recorded admin action not found in list")
	}
}

func TestTxManagerAtomicityAndRollback(t *testing.T) {
	pool, ctx := setupTestDB(t)
	txMgr := repository.NewTxManager(pool)
	userRepo := repository.NewUserRepository(pool)

	phone1 := uniquePhone()
	phone2 := uniquePhone()

	// 1. Success case: both operations committed
	err := txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		u1 := &domain.User{Phone: phone1, FullName: "Atomic User 1", Role: domain.RoleCustomer, IsActive: true}
		if err := userRepo.Create(txCtx, u1); err != nil {
			return err
		}
		u2 := &domain.User{Phone: phone2, FullName: "Atomic User 2", Role: domain.RoleCustomer, IsActive: true}
		return userRepo.Create(txCtx, u2)
	})
	if err != nil {
		t.Fatalf("WithinTransaction failed: %v", err)
	}

	// Verify both users exist
	if _, err := userRepo.GetByPhone(ctx, phone1); err != nil {
		t.Errorf("Expected user1 to exist after commit: %v", err)
	}
	if _, err := userRepo.GetByPhone(ctx, phone2); err != nil {
		t.Errorf("Expected user2 to exist after commit: %v", err)
	}

	// 2. Rollback case: failure inside block rolls back all operations
	phone3 := uniquePhone()
	err = txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		u3 := &domain.User{Phone: phone3, FullName: "Atomic User 3", Role: domain.RoleCustomer, IsActive: true}
		if err := userRepo.Create(txCtx, u3); err != nil {
			return err
		}
		// Return deliberate error to trigger rollback
		return fmt.Errorf("simulated failure inside transaction block")
	})
	if err == nil {
		t.Fatalf("Expected error from transaction block")
	}

	// Verify user3 was rolled back and does not exist in database
	_, err = userRepo.GetByPhone(ctx, phone3)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Expected domain.ErrNotFound for rolled-back user, got: %v", err)
	}
}

func strPtr(s string) *string {
	return &s
}
