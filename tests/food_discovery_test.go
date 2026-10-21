package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

func TestCanonicalDish_DatabaseConstraints(t *testing.T) {
	pool, ctx := setupTestDB(t)
	dishRepo := repository.NewDishRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)

	suffix := time.Now().UnixNano()
	slug := fmt.Sprintf("test-dish-%d", suffix)
	name := fmt.Sprintf("Test Dish %d", suffix)

	// 1. Create canonical dish
	dish := &domain.CanonicalDish{
		Name:        name,
		Slug:        slug,
		Description: "A delicious test dish",
		Cuisine:     "Ethiopian",
	}
	err := dishRepo.Create(ctx, dish)
	if err != nil {
		t.Fatalf("Failed to create canonical dish: %v", err)
	}
	if dish.ID == "" {
		t.Fatalf("Expected non-empty dish ID")
	}

	// 2. Unique slug constraint
	duplicateSlugDish := &domain.CanonicalDish{
		Name:    fmt.Sprintf("Different Name %d", suffix),
		Slug:    slug, // Same slug
		Cuisine: "Ethiopian",
	}
	err = dishRepo.Create(ctx, duplicateSlugDish)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Expected ErrConflict for duplicate slug, got: %v", err)
	}

	// 3. Add alias and verify uniqueness constraint
	alias := &domain.DishAlias{
		CanonicalDishID: dish.ID,
		Alias:           "Testy",
	}
	err = dishRepo.AddAlias(ctx, alias)
	if err != nil {
		t.Fatalf("Failed to add alias: %v", err)
	}

	// Duplicate alias for same dish should fail
	duplicateAlias := &domain.DishAlias{
		CanonicalDishID: dish.ID,
		Alias:           "TESTY", // Normalizes to "testy"
	}
	err = dishRepo.AddAlias(ctx, duplicateAlias)
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("Expected ErrConflict for duplicate normalized alias, got: %v", err)
	}

	// 4. Nullable menu item mapping
	bizSlug := uniqueSlug("test-biz-menu")
	bizID := createTestBusiness(t, ctx, pool, bizSlug)

	cat := &domain.MenuCategory{
		BusinessID: domain.BusinessID(bizID),
		Name:       "Mains",
	}
	if err := menuRepo.CreateCategory(ctx, cat); err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create unmapped menu item (canonical_dish_id is nil)
	item := &domain.MenuItem{
		CategoryID:    cat.ID,
		BusinessID:    domain.BusinessID(bizID),
		Name:          "Special Chef Item",
		PriceETBMinor: 25000,
		IsAvailable:   true,
	}
	if err := menuRepo.CreateItem(ctx, item); err != nil {
		t.Fatalf("Failed to create menu item without canonical mapping: %v", err)
	}
	if item.CanonicalDishID != nil {
		t.Fatalf("Expected nil CanonicalDishID for unmapped item, got: %v", item.CanonicalDishID)
	}

	// Map item to canonical dish
	err = menuRepo.MapCanonicalDish(ctx, item.ID, &dish.ID)
	if err != nil {
		t.Fatalf("Failed to map canonical dish: %v", err)
	}

	fetchedItem, err := menuRepo.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("Failed to get mapped item: %v", err)
	}
	if fetchedItem.CanonicalDishID == nil || *fetchedItem.CanonicalDishID != dish.ID {
		t.Fatalf("Expected CanonicalDishID %v, got %v", dish.ID, fetchedItem.CanonicalDishID)
	}
	if fetchedItem.CanonicalDish == nil || fetchedItem.CanonicalDish.Name != dish.Name {
		t.Fatalf("Expected populated CanonicalDish %v, got %+v", dish.Name, fetchedItem.CanonicalDish)
	}

	// 5. Safe deletion: deleting canonical dish must set item.canonical_dish_id to null and NOT delete item
	if err := dishRepo.Delete(ctx, dish.ID); err != nil {
		t.Fatalf("Failed to delete canonical dish: %v", err)
	}

	itemAfterDishDelete, err := menuRepo.GetItemByID(ctx, item.ID)
	if err != nil {
		t.Fatalf("MenuItem was accidentally deleted when canonical dish was removed: %v", err)
	}
	if itemAfterDishDelete.CanonicalDishID != nil {
		t.Fatalf("Expected CanonicalDishID to be NULL after dish deletion, got %v", itemAfterDishDelete.CanonicalDishID)
	}
	if itemAfterDishDelete.Name != "Special Chef Item" {
		t.Fatalf("Expected item name to remain unchanged, got %q", itemAfterDishDelete.Name)
	}
}

func TestFoodDiscovery_SearchAndRelevance(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	reportRepo := repository.NewReportRepository(pool)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	mediaRepo := repository.NewMediaRepository(pool)
	dishRepo := repository.NewDishRepository(pool)
	storageClient := storage.NewMemoryStorage("http://localhost:9000/test-bucket")
	txMgr := repository.NewTxManager(pool)

	authzSvc := service.NewAuthorizationService(ownershipRepo, businessRepo)
	workflowSvc := service.NewOwnershipWorkflowService(
		authzSvc, claimRepo, subRepo, businessRepo, ownershipRepo, adminActionRepo, mediaRepo, storageClient, txMgr,
	)
	adminSvc := service.NewAdminService(
		workflowSvc, authzSvc, subRepo, claimRepo, businessRepo, reviewRepo, reportRepo,
		adminActionRepo, mediaRepo, dishRepo, menuRepo, storageClient, txMgr,
	)
	discoverySvc := service.NewDiscoveryService(
		businessRepo, menuRepo, updateRepo, reviewRepo, mediaRepo, dishRepo, storageClient,
	)

	// Admin user
	adminUser := &domain.User{
		ID:       domain.UserID(fmt.Sprintf("%08d-0000-0000-0000-000000000001", time.Now().UnixNano()%100000000)),
		Phone:    uniquePhone(),
		FullName: "Platform Admin",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := userRepo.Create(ctx, adminUser); err != nil {
		t.Fatalf("Failed to create admin user: %v", err)
	}

	suffix := time.Now().UnixNano()
	// Create canonical dish "Tibs" with test-specific unique slug
	tibsSlug := fmt.Sprintf("tibs-%d", suffix)
	tibsDish, err := adminSvc.CreateCanonicalDish(ctx, adminUser, service.CreateCanonicalDishInput{
		Name:        fmt.Sprintf("Tibs %d", suffix),
		Slug:        tibsSlug,
		Description: "Sautéed savory beef",
		Cuisine:     "Ethiopian",
	})
	if err != nil {
		t.Fatalf("Failed to create canonical dish Tibs: %v", err)
	}

	// Add aliases: "tibse", "tibbs", "ጥብስ"
	uniqueAliasEnglish := fmt.Sprintf("tibse%d", suffix%100000)
	uniqueAliasAmharic := fmt.Sprintf("ጥብስ%d", suffix%100000)
	if _, err := adminSvc.AddDishAlias(ctx, adminUser, tibsDish.ID, uniqueAliasEnglish, nil); err != nil {
		t.Fatalf("Failed to add alias: %v", err)
	}
	if _, err := adminSvc.AddDishAlias(ctx, adminUser, tibsDish.ID, uniqueAliasAmharic, nil); err != nil {
		t.Fatalf("Failed to add Amharic alias: %v", err)
	}

	// Create Restaurant 1 (Gojo Restaurant)
	biz1 := &domain.Business{
		Name:              fmt.Sprintf("Gojo Restaurant %d", suffix),
		Slug:              fmt.Sprintf("gojo-%d", suffix),
		Category:          domain.CategoryRestaurant,
		Cuisine:           "Ethiopian",
		LocationName:      "Bole",
		Neighborhood:      "Bole",
		PublicationStatus: domain.PublicationStatusPublished,
	}
	if err := businessRepo.Create(ctx, biz1); err != nil {
		t.Fatalf("Failed to create biz1: %v", err)
	}

	// Create Restaurant 2 (ABC Restaurant)
	biz2 := &domain.Business{
		Name:              fmt.Sprintf("ABC Restaurant %d", suffix),
		Slug:              fmt.Sprintf("abc-%d", suffix),
		Category:          domain.CategoryRestaurant,
		Cuisine:           "Ethiopian",
		LocationName:      "Kazanchis",
		Neighborhood:      "Kazanchis",
		PublicationStatus: domain.PublicationStatusPublished,
	}
	if err := businessRepo.Create(ctx, biz2); err != nil {
		t.Fatalf("Failed to create biz2: %v", err)
	}

	// Create Category & Items for Biz 1
	cat1 := &domain.MenuCategory{BusinessID: biz1.ID, Name: "Traditional"}
	if err := menuRepo.CreateCategory(ctx, cat1); err != nil {
		t.Fatalf("Failed to create cat1: %v", err)
	}

	// Gojo Special Tibs (280 ETB = 28000 Santims)
	item1 := &domain.MenuItem{
		CategoryID:      cat1.ID,
		BusinessID:      biz1.ID,
		Name:            "Gojo Special Tibs",
		Description:     "Prime beef cubes tossed with peppers and clarified butter",
		PriceETBMinor:   28000,
		IsAvailable:     true,
		CanonicalDishID: &tibsDish.ID,
	}
	if err := menuRepo.CreateItem(ctx, item1); err != nil {
		t.Fatalf("Failed to create item1: %v", err)
	}

	// Unmapped Chef Special at Biz 1
	unmappedItem := &domain.MenuItem{
		CategoryID:    cat1.ID,
		BusinessID:    biz1.ID,
		Name:          fmt.Sprintf("Gojo Chef Surprise %d", suffix),
		Description:   "Chef secret recipe",
		PriceETBMinor: 35000,
		IsAvailable:   true,
	}
	if err := menuRepo.CreateItem(ctx, unmappedItem); err != nil {
		t.Fatalf("Failed to create unmappedItem: %v", err)
	}

	// Create Category & Item for Biz 2: Siga Tibs (220 ETB = 22000 Santims)
	cat2 := &domain.MenuCategory{BusinessID: biz2.ID, Name: "Main Courses"}
	if err := menuRepo.CreateCategory(ctx, cat2); err != nil {
		t.Fatalf("Failed to create cat2: %v", err)
	}

	item2 := &domain.MenuItem{
		CategoryID:      cat2.ID,
		BusinessID:      biz2.ID,
		Name:            "Siga Tibs",
		Description:     "Tender sautéed meat",
		PriceETBMinor:   22000,
		IsAvailable:     true,
		CanonicalDishID: &tibsDish.ID,
	}
	if err := menuRepo.CreateItem(ctx, item2); err != nil {
		t.Fatalf("Failed to create item2: %v", err)
	}

	// 1. Search by canonical dish name
	t.Run("Search by Canonical Dish Name", func(t *testing.T) {
		resp, err := discoverySvc.SearchFood(ctx, tibsDish.Name, 20, 0)
		if err != nil {
			t.Fatalf("SearchFood failed: %v", err)
		}
		if resp.CanonicalDish == nil || resp.CanonicalDish.ID != tibsDish.ID {
			t.Fatalf("Expected CanonicalDish %v, got %+v", tibsDish.Name, resp.CanonicalDish)
		}
		if len(resp.Results) < 2 {
			t.Fatalf("Expected at least 2 results for Tibs, got %d", len(resp.Results))
		}

		// Verify restaurant context and that original names and prices are preserved
		foundGojo := false
		foundABC := false
		for _, r := range resp.Results {
			if r.MenuItem.ID == item1.ID {
				foundGojo = true
				if r.MenuItem.Name != "Gojo Special Tibs" {
					t.Errorf("Expected original menu item name 'Gojo Special Tibs', got %q", r.MenuItem.Name)
				}
				if r.MenuItem.PriceETBMinor != 28000 {
					t.Errorf("Expected price 28000 santims, got %d", r.MenuItem.PriceETBMinor)
				}
				if r.Business.ID != biz1.ID {
					t.Errorf("Expected business Gojo (%d), got %d", biz1.ID, r.Business.ID)
				}
			}
			if r.MenuItem.ID == item2.ID {
				foundABC = true
				if r.MenuItem.Name != "Siga Tibs" {
					t.Errorf("Expected original menu item name 'Siga Tibs', got %q", r.MenuItem.Name)
				}
				if r.MenuItem.PriceETBMinor != 22000 {
					t.Errorf("Expected price 22000 santims, got %d", r.MenuItem.PriceETBMinor)
				}
				if r.Business.ID != biz2.ID {
					t.Errorf("Expected business ABC (%d), got %d", biz2.ID, r.Business.ID)
				}
			}
		}

		if !foundGojo || !foundABC {
			t.Errorf("Did not find both Gojo (%v) and ABC (%v)", foundGojo, foundABC)
		}
	})

	// 2. Search by alias
	t.Run("Search by English Alias", func(t *testing.T) {
		resp, err := discoverySvc.SearchFood(ctx, uniqueAliasEnglish, 20, 0)
		if err != nil {
			t.Fatalf("SearchFood failed: %v", err)
		}
		if resp.CanonicalDish == nil || resp.CanonicalDish.ID != tibsDish.ID {
			t.Fatalf("Expected CanonicalDish to match through alias, got %+v", resp.CanonicalDish)
		}
		if len(resp.Results) < 2 {
			t.Fatalf("Expected results through alias match, got %d", len(resp.Results))
		}
	})

	// 3. Search case insensitive
	t.Run("Search Case Insensitive", func(t *testing.T) {
		resp, err := discoverySvc.SearchFood(ctx, "  "+tibsDish.Name+"  ", 20, 0)
		if err != nil {
			t.Fatalf("SearchFood failed: %v", err)
		}
		if len(resp.Results) < 2 {
			t.Fatalf("Expected results for padded/case-variant query, got %d", len(resp.Results))
		}
	})

	// 4. Search by Amharic alias
	t.Run("Search by Amharic Alias", func(t *testing.T) {
		resp, err := discoverySvc.SearchFood(ctx, uniqueAliasAmharic, 20, 0)
		if err != nil {
			t.Fatalf("SearchFood failed: %v", err)
		}
		if resp.CanonicalDish == nil || resp.CanonicalDish.ID != tibsDish.ID {
			t.Fatalf("Expected CanonicalDish to match through Amharic alias, got %+v", resp.CanonicalDish)
		}
		if len(resp.Results) < 2 {
			t.Fatalf("Expected results through Amharic alias match, got %d", len(resp.Results))
		}
	})

	// 5. Search unmapped item directly by name
	t.Run("Search Unmapped Menu Item By Exact Name", func(t *testing.T) {
		resp, err := discoverySvc.SearchFood(ctx, unmappedItem.Name, 20, 0)
		if err != nil {
			t.Fatalf("SearchFood failed: %v", err)
		}
		if len(resp.Results) == 0 {
			t.Fatalf("Expected to find unmapped menu item by name")
		}
		found := false
		for _, r := range resp.Results {
			if r.MenuItem.ID == unmappedItem.ID {
				found = true
				if r.CanonicalDish != nil {
					t.Errorf("Expected nil CanonicalDish for unmapped item, got %+v", r.CanonicalDish)
				}
				if r.MenuItem.PriceETBMinor != 35000 {
					t.Errorf("Expected price 35000, got %d", r.MenuItem.PriceETBMinor)
				}
			}
		}
		if !found {
			t.Errorf("Unmapped item %s not found in results", unmappedItem.Name)
		}
	})

	// 6. Search with Max Price filter
	t.Run("Search with Max Price Filter", func(t *testing.T) {
		maxPrice := int64(25000) // 250 ETB, should only include item2 (220 ETB), not item1 (280 ETB)
		resp, err := discoverySvc.SearchFoodWithFilter(ctx, domain.FoodSearchFilter{
			Query:            tibsDish.Name,
			MaxPriceETBMinor: &maxPrice,
			Limit:            20,
		})
		if err != nil {
			t.Fatalf("SearchFoodWithFilter failed: %v", err)
		}
		if len(resp.Results) == 0 {
			t.Fatalf("Expected results under 250 ETB, got 0")
		}
		foundItem2 := false
		for _, r := range resp.Results {
			if r.MenuItem.PriceETBMinor > maxPrice {
				t.Errorf("Expected price <= %d, got %d for item %s", maxPrice, r.MenuItem.PriceETBMinor, r.MenuItem.ID)
			}
			if r.MenuItem.ID == item2.ID {
				foundItem2 = true
			}
			if r.MenuItem.ID == item1.ID {
				t.Errorf("Expected expensive item1 (280 ETB) to be excluded by 250 ETB filter")
			}
		}
		if !foundItem2 {
			t.Errorf("Expected item2 (220 ETB) in results, but was not found")
		}
		if len(resp.Items) != len(resp.Results) {
			t.Errorf("Expected Items slice to match Results slice")
		}
	})

	// 7. Search with Area / Neighborhood filter
	t.Run("Search with Area Filter", func(t *testing.T) {
		boleArea := "Bole"
		respBole, err := discoverySvc.SearchFoodWithFilter(ctx, domain.FoodSearchFilter{
			Query:        tibsDish.Name,
			Neighborhood: &boleArea,
			Limit:        20,
		})
		if err != nil {
			t.Fatalf("SearchFoodWithFilter failed: %v", err)
		}
		for _, r := range respBole.Results {
			if r.Business.Neighborhood != "Bole" {
				t.Errorf("Expected only Bole restaurants, got %q", r.Business.Neighborhood)
			}
		}

		kazanchisArea := "Kazanchis"
		respKaz, err := discoverySvc.SearchFoodWithFilter(ctx, domain.FoodSearchFilter{
			Query:        tibsDish.Name,
			Neighborhood: &kazanchisArea,
			Limit:        20,
		})
		if err != nil {
			t.Fatalf("SearchFoodWithFilter failed: %v", err)
		}
		for _, r := range respKaz.Results {
			if r.Business.Neighborhood != "Kazanchis" {
				t.Errorf("Expected only Kazanchis restaurants, got %q", r.Business.Neighborhood)
			}
		}
	})

	// 8. Search without dish query (area + budget discovery)
	t.Run("Search by Area and Budget without Dish Name", func(t *testing.T) {
		boleArea := "Bole"
		maxPrice := int64(30000)
		resp, err := discoverySvc.SearchFoodWithFilter(ctx, domain.FoodSearchFilter{
			Neighborhood:     &boleArea,
			MaxPriceETBMinor: &maxPrice,
			Limit:            20,
		})
		if err != nil {
			t.Fatalf("SearchFoodWithFilter failed: %v", err)
		}
		if len(resp.Results) == 0 {
			t.Fatalf("Expected results for area + budget search without dish query")
		}
		for _, r := range resp.Results {
			if r.Business.Neighborhood != "Bole" {
				t.Errorf("Expected Bole venue, got %q", r.Business.Neighborhood)
			}
			if r.MenuItem.PriceETBMinor > maxPrice {
				t.Errorf("Expected price <= %d, got %d", maxPrice, r.MenuItem.PriceETBMinor)
			}
		}
	})
}

func TestFoodDiscovery_AuthorizationAndIDOR(t *testing.T) {
	pool, ctx := setupTestDB(t)
	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	reportRepo := repository.NewReportRepository(pool)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	mediaRepo := repository.NewMediaRepository(pool)
	dishRepo := repository.NewDishRepository(pool)
	storageClient := storage.NewMemoryStorage("http://localhost:9000/test-bucket")
	txMgr := repository.NewTxManager(pool)

	authzSvc := service.NewAuthorizationService(ownershipRepo, businessRepo)
	workflowSvc := service.NewOwnershipWorkflowService(
		authzSvc, claimRepo, subRepo, businessRepo, ownershipRepo, adminActionRepo, mediaRepo, storageClient, txMgr,
	)
	adminSvc := service.NewAdminService(
		workflowSvc, authzSvc, subRepo, claimRepo, businessRepo, reviewRepo, reportRepo,
		adminActionRepo, mediaRepo, dishRepo, menuRepo, storageClient, txMgr,
	)

	// 1. Customer
	customer := &domain.User{
		ID:       domain.UserID(fmt.Sprintf("%08d-0000-0000-0000-000000000010", time.Now().UnixNano()%100000000)),
		Phone:    uniquePhone(),
		FullName: "Normal Customer",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatalf("Failed to create customer: %v", err)
	}

	// 2. Owner of Business 1
	owner1 := &domain.User{
		ID:       domain.UserID(fmt.Sprintf("%08d-0000-0000-0000-000000000011", time.Now().UnixNano()%100000000)),
		Phone:    uniquePhone(),
		FullName: "Owner 1",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := userRepo.Create(ctx, owner1); err != nil {
		t.Fatalf("Failed to create owner1: %v", err)
	}

	// 3. Owner of Business 2
	owner2 := &domain.User{
		ID:       domain.UserID(fmt.Sprintf("%08d-0000-0000-0000-000000000012", time.Now().UnixNano()%100000000)),
		Phone:    uniquePhone(),
		FullName: "Owner 2",
		Role:     domain.RoleCustomer,
		IsActive: true,
	}
	if err := userRepo.Create(ctx, owner2); err != nil {
		t.Fatalf("Failed to create owner2: %v", err)
	}

	// 4. Admin
	admin := &domain.User{
		ID:       domain.UserID(fmt.Sprintf("%08d-0000-0000-0000-000000000013", time.Now().UnixNano()%100000000)),
		Phone:    uniquePhone(),
		FullName: "Admin User",
		Role:     domain.RoleAdmin,
		IsActive: true,
	}
	if err := userRepo.Create(ctx, admin); err != nil {
		t.Fatalf("Failed to create admin: %v", err)
	}

	// Create Business 1 and assign owner1
	biz1ID := domain.BusinessID(createTestBusiness(t, ctx, pool, uniqueSlug("owner1-biz")))
	if err := ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  biz1ID,
		UserID:      owner1.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("Failed to assign owner1 manager: %v", err)
	}

	// Create Business 2 and assign owner2
	biz2ID := domain.BusinessID(createTestBusiness(t, ctx, pool, uniqueSlug("owner2-biz")))
	if err := ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  biz2ID,
		UserID:      owner2.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("Failed to assign owner2 manager: %v", err)
	}

	// Create menu category and item for Business 2
	cat2 := &domain.MenuCategory{BusinessID: biz2ID, Name: "Biz2 Mains"}
	if err := menuRepo.CreateCategory(ctx, cat2); err != nil {
		t.Fatalf("Failed to create cat2: %v", err)
	}
	itemBiz2 := &domain.MenuItem{
		CategoryID:    cat2.ID,
		BusinessID:    biz2ID,
		Name:          "Biz 2 Private Dish",
		PriceETBMinor: 10000,
		IsAvailable:   true,
	}
	if err := menuRepo.CreateItem(ctx, itemBiz2); err != nil {
		t.Fatalf("Failed to create itemBiz2: %v", err)
	}

	// Create canonical dish via Admin
	dish, err := adminSvc.CreateCanonicalDish(ctx, admin, service.CreateCanonicalDishInput{
		Name:    fmt.Sprintf("Taxonomy Dish %d", time.Now().UnixNano()),
		Slug:    uniqueSlug("tax-dish"),
		Cuisine: "Ethiopian",
	})
	if err != nil {
		t.Fatalf("Admin failed to create canonical dish: %v", err)
	}

	// A. Customer cannot create canonical dish
	_, err = adminSvc.CreateCanonicalDish(ctx, customer, service.CreateCanonicalDishInput{
		Name: "Customer Dish",
		Slug: "customer-dish",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when customer creates canonical dish, got: %v", err)
	}

	// B. Business owner cannot create canonical dish
	_, err = adminSvc.CreateCanonicalDish(ctx, owner1, service.CreateCanonicalDishInput{
		Name: "Owner Dish",
		Slug: "owner-dish",
	})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when owner creates canonical dish, got: %v", err)
	}

	// C. Customer cannot add alias
	_, err = adminSvc.AddDishAlias(ctx, customer, dish.ID, "customer-alias", nil)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected ErrForbidden when customer adds alias, got: %v", err)
	}

	// D. Owner 1 cannot map Business 2's menu item (IDOR Prevention!)
	err = adminSvc.MapMenuItemCanonicalDish(ctx, owner1, itemBiz2.ID, &dish.ID)
	if !errors.Is(err, domain.ErrNotBusinessManager) && !errors.Is(err, domain.ErrResourceNotManaged) && !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("Expected forbidden/not managed when owner1 attempts IDOR mapping of owner2's item, got: %v", err)
	}

	// E. Owner 2 CAN map their own item
	err = adminSvc.MapMenuItemCanonicalDish(ctx, owner2, itemBiz2.ID, &dish.ID)
	if err != nil {
		t.Fatalf("Owner2 should be able to map their own item: %v", err)
	}

	// F. Admin CAN map any business's item
	err = adminSvc.MapMenuItemCanonicalDish(ctx, admin, itemBiz2.ID, nil) // unmap
	if err != nil {
		t.Fatalf("Admin should be able to map/unmap any item: %v", err)
	}
}

func TestFoodDiscovery_HTTP_Endpoints(t *testing.T) {
	h := setupHTTPTest(t)

	// Create admin client and customer client
	adminClient, _ := h.loginClient(t, uniquePhone(), domain.RoleAdmin)
	customerClient, _ := h.loginClient(t, uniquePhone(), domain.RoleCustomer)

	suffix := time.Now().UnixNano()
	dishName := fmt.Sprintf("HTTP Tibs %d", suffix)
	dishSlug := fmt.Sprintf("http-tibs-%d", suffix)

	// 1. Customer attempts POST /api/v1/admin/dishes -> 403 Forbidden
	payloadCreate := map[string]string{
		"name":        dishName,
		"slug":        dishSlug,
		"cuisine":     "Ethiopian",
		"description": "Tasty food",
	}
	bodyCreate, _ := json.Marshal(payloadCreate)
	res, err := customerClient.Post(h.server.URL+"/api/v1/admin/dishes", "application/json", bytes.NewReader(bodyCreate))
	if err != nil {
		t.Fatalf("POST /api/v1/admin/dishes failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden for customer, got %d", res.StatusCode)
	}

	// 2. Admin creates canonical dish -> 201 Created
	res, err = adminClient.Post(h.server.URL+"/api/v1/admin/dishes", "application/json", bytes.NewReader(bodyCreate))
	if err != nil {
		t.Fatalf("Admin POST /api/v1/admin/dishes failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("Admin create dish failed (%d): %s", res.StatusCode, string(b))
	}

	var dishEnvelope struct {
		Data domain.CanonicalDish `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&dishEnvelope); err != nil {
		t.Fatalf("Failed to decode created dish: %v", err)
	}
	createdDish := dishEnvelope.Data

	// 3. Admin adds alias -> 201 Created
	aliasName := fmt.Sprintf("http-alias-%d", suffix%100000)
	aliasPayload := map[string]string{"alias": aliasName}
	bodyAlias, _ := json.Marshal(aliasPayload)
	res, err = adminClient.Post(fmt.Sprintf("%s/api/v1/admin/dishes/%s/aliases", h.server.URL, createdDish.ID), "application/json", bytes.NewReader(bodyAlias))
	if err != nil {
		t.Fatalf("Add alias failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("Add alias failed (%d): %s", res.StatusCode, string(b))
	}

	// 4. Public search via GET /api/v1/dishes/search?q=... -> 200 OK
	res, err = h.client.Get(fmt.Sprintf("%s/api/v1/dishes/search?q=%s", h.server.URL, aliasName))
	if err != nil {
		t.Fatalf("Public food search failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("Search food failed (%d): %s", res.StatusCode, string(b))
	}

	var searchEnvelope struct {
		Data domain.FoodSearchResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchEnvelope); err != nil {
		t.Fatalf("Failed to decode search response: %v", err)
	}
	searchResp := searchEnvelope.Data
	if searchResp.CanonicalDish == nil || searchResp.CanonicalDish.ID != createdDish.ID {
		t.Fatalf("Expected matched canonical dish %v, got %+v", createdDish.ID, searchResp.CanonicalDish)
	}

	// 5. Public get dish via GET /api/v1/dishes/{id} -> 200 OK
	res, err = h.client.Get(fmt.Sprintf("%s/api/v1/dishes/%s", h.server.URL, createdDish.ID))
	if err != nil {
		t.Fatalf("GET /api/v1/dishes/{id} failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("Get dish failed (%d): %s", res.StatusCode, string(b))
	}

	var getDishEnvelope struct {
		Data domain.CanonicalDish `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&getDishEnvelope); err != nil {
		t.Fatalf("Failed to decode get dish: %v", err)
	}
	if getDishEnvelope.Data.ID != createdDish.ID {
		t.Fatalf("Expected dish ID %s, got %s", createdDish.ID, getDishEnvelope.Data.ID)
	}
}
