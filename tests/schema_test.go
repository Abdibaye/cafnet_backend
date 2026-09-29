package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"

	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

func setupTestDB(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	_ = godotenv.Load("../.env")

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)

	cfg := db.LoadConfigFromEnv()
	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect to Neon PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool, ctx
}

// Helper to create test user
func createTestUser(t *testing.T, ctx context.Context, pool *db.Pool, phone string) string {
	t.Helper()
	var userID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (phone, full_name, role)
		VALUES ($1, 'Test User', 'CUSTOMER')
		RETURNING id;
	`, phone).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return userID
}

// Helper to create test business
func createTestBusiness(t *testing.T, ctx context.Context, pool *db.Pool, slug string) int64 {
	t.Helper()
	var businessID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO businesses (name, slug, category, cuisine, location_name, neighborhood, publication_status)
		VALUES ('Test Venue', $1, 'restaurant', 'Ethiopian', 'Bole', 'Bole', 'PUBLISHED')
		RETURNING id;
	`, slug).Scan(&businessID)
	if err != nil {
		t.Fatalf("Failed to create test business: %v", err)
	}
	return businessID
}

// Test 1: Duplicate Save Rejected
func TestInvariant_DuplicateSaveRejected(t *testing.T) {
	pool, ctx := setupTestDB(t)

	userID := createTestUser(t, ctx, pool, "+251911"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "save-venue-"+uuid.NewString()[:8])

	// First save should succeed
	_, err := pool.Exec(ctx, "INSERT INTO saved_businesses (user_id, business_id) VALUES ($1, $2)", userID, busID)
	if err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Duplicate save must be rejected by PRIMARY KEY
	_, err = pool.Exec(ctx, "INSERT INTO saved_businesses (user_id, business_id) VALUES ($1, $2)", userID, busID)
	if err == nil {
		t.Fatal("Expected error on duplicate save, got nil")
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" { // unique_violation
		t.Fatalf("Expected unique violation (23505), got: %v", err)
	}
}

// Test 2: Duplicate Follow Rejected
func TestInvariant_DuplicateFollowRejected(t *testing.T) {
	pool, ctx := setupTestDB(t)

	userID := createTestUser(t, ctx, pool, "+251912"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "follow-venue-"+uuid.NewString()[:8])

	// First follow should succeed
	_, err := pool.Exec(ctx, "INSERT INTO followed_businesses (user_id, business_id) VALUES ($1, $2)", userID, busID)
	if err != nil {
		t.Fatalf("First follow failed: %v", err)
	}

	// Duplicate follow must be rejected by PRIMARY KEY
	_, err = pool.Exec(ctx, "INSERT INTO followed_businesses (user_id, business_id) VALUES ($1, $2)", userID, busID)
	if err == nil {
		t.Fatal("Expected error on duplicate follow, got nil")
	}

	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("Expected unique violation (23505), got: %v", err)
	}
}

// Test 3 & 4: Review for Reservation (Verified Visit & Duplicate Review Rejected)
func TestInvariant_ReviewReservationIntegrity(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251913"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "res-venue-"+uuid.NewString()[:8])

	// Create reservation in PENDING state
	var resID string
	resCode := "TEST-" + uuid.NewString()[:6]
	err := pool.QueryRow(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size, status)
		VALUES ($1, $2, $3, 'Abebe', '+251913000000', CURRENT_DATE, '19:00', 2, 'PENDING')
		RETURNING id;
	`, resCode, busID, customerID).Scan(&resID)
	if err != nil {
		t.Fatalf("Failed to create reservation: %v", err)
	}

	// Invariant: Review CANNOT be submitted for PENDING reservation
	_, err = pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 5, 'Great food!')
	`, busID, customerID, resID)
	if err == nil {
		t.Fatal("Expected error submitting review for PENDING reservation, got nil")
	}
	if !strings.Contains(err.Error(), "COMPLETED reservation") {
		t.Fatalf("Expected trigger error mentioning COMPLETED reservation, got: %v", err)
	}

	// Transition reservation: PENDING -> CONFIRMED -> COMPLETED
	_, err = pool.Exec(ctx, "UPDATE reservations SET status = 'CONFIRMED' WHERE id = $1", resID)
	if err != nil {
		t.Fatalf("Failed to confirm reservation: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE reservations SET status = 'COMPLETED' WHERE id = $1", resID)
	if err != nil {
		t.Fatalf("Failed to complete reservation: %v", err)
	}

	// Now review for COMPLETED reservation should succeed
	_, err = pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 5, 'Exceptional experience!')
	`, busID, customerID, resID)
	if err != nil {
		t.Fatalf("Failed to submit review for COMPLETED reservation: %v", err)
	}

	// Duplicate review for the exact same reservation MUST be rejected
	_, err = pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 4, 'Second attempt review')
	`, busID, customerID, resID)
	if err == nil {
		t.Fatal("Expected error on duplicate review for reservation, got nil")
	}
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("Expected unique violation on reservation_id (23505), got: %v", err)
	}
}

// Test 5: Helpful Vote Uniqueness
func TestInvariant_ReviewHelpfulVoteUniqueness(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251914"+uuid.NewString()[:8])
	voterID := createTestUser(t, ctx, pool, "+251915"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "vote-venue-"+uuid.NewString()[:8])

	// Create completed reservation and review
	var resID, reviewID string
	_ = pool.QueryRow(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size, status)
		VALUES ($1, $2, $3, 'Abebe', '+251914000000', CURRENT_DATE, '19:00', 2, 'COMPLETED')
		RETURNING id;
	`, "VOTE-"+uuid.NewString()[:6], busID, customerID).Scan(&resID)

	_ = pool.QueryRow(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 5, 'Great!')
		RETURNING id;
	`, busID, customerID, resID).Scan(&reviewID)

	// First vote succeeds
	_, err := pool.Exec(ctx, "INSERT INTO review_helpful_votes (review_id, customer_user_id) VALUES ($1, $2)", reviewID, voterID)
	if err != nil {
		t.Fatalf("First helpful vote failed: %v", err)
	}

	// Duplicate vote must be rejected
	_, err = pool.Exec(ctx, "INSERT INTO review_helpful_votes (review_id, customer_user_id) VALUES ($1, $2)", reviewID, voterID)
	if err == nil {
		t.Fatal("Expected error on duplicate helpful vote, got nil")
	}
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("Expected unique violation (23505), got: %v", err)
	}
}

// Test 6: Invalid Rating Rejected
func TestInvariant_InvalidRatingRejected(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251916"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "rating-venue-"+uuid.NewString()[:8])

	var resID string
	_ = pool.QueryRow(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size, status)
		VALUES ($1, $2, $3, 'Sara', '+251916000000', CURRENT_DATE, '19:00', 2, 'COMPLETED')
		RETURNING id;
	`, "RATE-"+uuid.NewString()[:6], busID, customerID).Scan(&resID)

	// Rating 0 is invalid (must be 1-5)
	_, err := pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 0, 'Zero stars')
	`, busID, customerID, resID)
	if err == nil {
		t.Fatal("Expected error with rating 0, got nil")
	}

	// Rating 6 is invalid
	_, err = pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 6, 'Six stars')
	`, busID, customerID, resID)
	if err == nil {
		t.Fatal("Expected error with rating 6, got nil")
	}
}

// Test 7: Invalid Party Size Rejected
func TestInvariant_InvalidPartySizeRejected(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251917"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "party-venue-"+uuid.NewString()[:8])

	// Party size 0 must fail check constraint
	_, err := pool.Exec(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size)
		VALUES ('BAD-0', $1, $2, 'Sara', '+251917000000', CURRENT_DATE, '19:00', 0)
	`, busID, customerID)
	if err == nil {
		t.Fatal("Expected error with party_size 0, got nil")
	}

	// Party size 51 exceeds limit (max 50)
	_, err = pool.Exec(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size)
		VALUES ('BAD-51', $1, $2, 'Sara', '+251917000000', CURRENT_DATE, '19:00', 51)
	`, busID, customerID)
	if err == nil {
		t.Fatal("Expected error with party_size 51, got nil")
	}
}

// Test 8: Invalid Enum / State Value Rejected
func TestInvariant_InvalidStateValueRejected(t *testing.T) {
	pool, ctx := setupTestDB(t)

	// Invalid publication_status
	_, err := pool.Exec(ctx, `
		INSERT INTO businesses (name, slug, category, cuisine, location_name, neighborhood, publication_status)
		VALUES ('Invalid State', 'invalid-slug', 'restaurant', 'Ethiopian', 'Bole', 'Bole', 'INVALID_STATUS')
	`)
	if err == nil {
		t.Fatal("Expected check constraint error on invalid publication_status, got nil")
	}

	// Invalid category
	_, err = pool.Exec(ctx, `
		INSERT INTO businesses (name, slug, category, cuisine, location_name, neighborhood)
		VALUES ('Invalid Cat', 'invalid-cat-slug', 'gas_station', 'Ethiopian', 'Bole', 'Bole')
	`)
	if err == nil {
		t.Fatal("Expected check constraint error on invalid category, got nil")
	}
}

// Test 9: Ownership Relationship Uniqueness
func TestInvariant_BusinessManagerUniqueness(t *testing.T) {
	pool, ctx := setupTestDB(t)

	managerID := createTestUser(t, ctx, pool, "+251918"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "mgr-venue-"+uuid.NewString()[:8])

	// First assignment succeeds
	_, err := pool.Exec(ctx, "INSERT INTO business_managers (business_id, user_id, manager_role) VALUES ($1, $2, 'PRIMARY_OWNER')", busID, managerID)
	if err != nil {
		t.Fatalf("First manager assignment failed: %v", err)
	}

	// Duplicate assignment of same user to same venue must fail
	_, err = pool.Exec(ctx, "INSERT INTO business_managers (business_id, user_id, manager_role) VALUES ($1, $2, 'MANAGER')", busID, managerID)
	if err == nil {
		t.Fatal("Expected error on duplicate business manager assignment, got nil")
	}
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("Expected unique violation (23505), got: %v", err)
	}
}

// Test 10: Claim Uniqueness (No duplicate active pending claims for same business)
func TestInvariant_PendingClaimUniqueness(t *testing.T) {
	pool, ctx := setupTestDB(t)

	userA := createTestUser(t, ctx, pool, "+251919"+uuid.NewString()[:8])
	userB := createTestUser(t, ctx, pool, "+251920"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "claim-venue-"+uuid.NewString()[:8])

	// User A claims business (PENDING)
	_, err := pool.Exec(ctx, "INSERT INTO business_claims (business_id, claimant_user_id, status) VALUES ($1, $2, 'PENDING')", busID, userA)
	if err != nil {
		t.Fatalf("User A pending claim failed: %v", err)
	}

	// User B attempts concurrent claim on same business while first is PENDING -> MUST FAIL
	_, err = pool.Exec(ctx, "INSERT INTO business_claims (business_id, claimant_user_id, status) VALUES ($1, $2, 'PENDING')", busID, userB)
	if err == nil {
		t.Fatal("Expected error on concurrent pending claim for same business, got nil")
	}
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || pgErr.Code != "23505" {
		t.Fatalf("Expected partial unique index violation (23505), got: %v", err)
	}
}

// Test 11: Reservation State Machine Transitions Guard
func TestInvariant_ReservationStateMachineTransitions(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251921"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "sm-venue-"+uuid.NewString()[:8])

	var resID string
	_ = pool.QueryRow(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size, status)
		VALUES ($1, $2, $3, 'Abebe', '+251921000000', CURRENT_DATE, '19:00', 4, 'PENDING')
		RETURNING id;
	`, "SM-"+uuid.NewString()[:6], busID, customerID).Scan(&resID)

	// Illegal: PENDING directly to COMPLETED (must confirm first)
	_, err := pool.Exec(ctx, "UPDATE reservations SET status = 'COMPLETED' WHERE id = $1", resID)
	if err == nil {
		t.Fatal("Expected illegal transition error PENDING -> COMPLETED, got nil")
	}

	// Illegal: DECLINED without decline reason
	_, err = pool.Exec(ctx, "UPDATE reservations SET status = 'DECLINED', decline_reason = '' WHERE id = $1", resID)
	if err == nil {
		t.Fatal("Expected decline reason required error, got nil")
	}

	// Legal: PENDING -> DECLINED with reason
	_, err = pool.Exec(ctx, "UPDATE reservations SET status = 'DECLINED', decline_reason = 'Fully booked' WHERE id = $1", resID)
	if err != nil {
		t.Fatalf("Failed legal transition to DECLINED: %v", err)
	}

	// Illegal: Modifying a terminal state (DECLINED -> CONFIRMED)
	_, err = pool.Exec(ctx, "UPDATE reservations SET status = 'CONFIRMED' WHERE id = $1", resID)
	if err == nil {
		t.Fatal("Expected error modifying terminal DECLINED state, got nil")
	}
}

// Test 12: Foreign Key Integrity (Cannot delete completed reservation referenced by review)
func TestInvariant_ForeignKeyIntegrityReservationReview(t *testing.T) {
	pool, ctx := setupTestDB(t)

	customerID := createTestUser(t, ctx, pool, "+251922"+uuid.NewString()[:8])
	busID := createTestBusiness(t, ctx, pool, "fk-venue-"+uuid.NewString()[:8])

	var resID string
	_ = pool.QueryRow(ctx, `
		INSERT INTO reservations (reservation_code, business_id, customer_user_id, customer_name, customer_phone, reservation_date, reservation_time, party_size, status)
		VALUES ($1, $2, $3, 'Abebe', '+251922000000', CURRENT_DATE, '19:00', 2, 'COMPLETED')
		RETURNING id;
	`, "FK-"+uuid.NewString()[:6], busID, customerID).Scan(&resID)

	// Submit review
	_, err := pool.Exec(ctx, `
		INSERT INTO reviews (business_id, customer_user_id, reservation_id, rating, review_text)
		VALUES ($1, $2, $3, 5, 'Great meal!')
	`, busID, customerID, resID)
	if err != nil {
		t.Fatalf("Failed to create review: %v", err)
	}

	// Deleting reservation must be BLOCKED by ON DELETE RESTRICT
	_, err = pool.Exec(ctx, "DELETE FROM reservations WHERE id = $1", resID)
	if err == nil {
		t.Fatal("Expected foreign key violation deleting reservation with review, got nil")
	}
	pgErr, ok := err.(*pgconn.PgError)
	if !ok || (pgErr.Code != "23001" && pgErr.Code != "23503") { // 23001 = restrict_violation, 23503 = foreign_key_violation
		t.Fatalf("Expected RESTRICT foreign key violation (23001 or 23503), got: %v", err)
	}
}
