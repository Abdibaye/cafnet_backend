package storage

import (
	"context"
	"testing"
	"time"
)

func TestStorage_KeyGenerators(t *testing.T) {
	subKey := SubmissionCoverKey("sub-123", "med-456", "jpg")
	expectedSub := "submissions/sub-123/cover/med-456.jpg"
	if subKey != expectedSub {
		t.Errorf("expected %s, got %s", expectedSub, subKey)
	}

	bizCoverKey := BusinessCoverKey(42, "med-789", ".webp")
	expectedBizCover := "businesses/42/cover/med-789.webp"
	if bizCoverKey != expectedBizCover {
		t.Errorf("expected %s, got %s", expectedBizCover, bizCoverKey)
	}

	bizGalleryKey := BusinessGalleryKey(42, "med-000", "png")
	expectedBizGallery := "businesses/42/gallery/med-000.png"
	if bizGalleryKey != expectedBizGallery {
		t.Errorf("expected %s, got %s", expectedBizGallery, bizGalleryKey)
	}

	menuItemKey := MenuItemKey(42, "item-99", "med-111", ".jpg")
	expectedMenuItem := "businesses/42/menu-items/item-99/med-111.jpg"
	if menuItemKey != expectedMenuItem {
		t.Errorf("expected %s, got %s", expectedMenuItem, menuItemKey)
	}
}

func TestStorage_Validation(t *testing.T) {
	allowed := []string{"image/jpeg", "image/png", "image/webp"}

	if err := ValidateMimeType("image/jpeg", allowed); err != nil {
		t.Errorf("expected valid mime type, got %v", err)
	}
	if err := ValidateMimeType("image/svg+xml", allowed); err == nil {
		t.Errorf("expected SVG to be rejected")
	}
	if err := ValidateMimeType("application/octet-stream", allowed); err == nil {
		t.Errorf("expected binary to be rejected")
	}

	if ext, err := MimeToExtension("image/webp"); err != nil || ext != ".webp" {
		t.Errorf("expected .webp, got %s, err %v", ext, err)
	}

	if err := ValidateFileSize(500, 1000); err != nil {
		t.Errorf("expected valid size, got %v", err)
	}
	if err := ValidateFileSize(1500, 1000); err == nil {
		t.Errorf("expected oversized file to be rejected")
	}
	if err := ValidateFileSize(0, 1000); err == nil {
		t.Errorf("expected zero byte file to be rejected")
	}
}

func TestStorage_MemoryStorage(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage("https://media.cafnet.app")

	key := "businesses/1/cover/test.jpg"
	url, err := store.CreatePresignedUpload(ctx, key, "image/jpeg", 1024, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to create upload url: %v", err)
	}
	if url == "" {
		t.Fatal("upload url is empty")
	}

	exists, err := store.ObjectExists(ctx, key)
	if err != nil || !exists {
		t.Fatalf("expected object to exist, got %v, err %v", exists, err)
	}

	meta, err := store.HeadObject(ctx, key)
	if err != nil || meta.ContentType != "image/jpeg" {
		t.Fatalf("unexpected meta: %+v, err: %v", meta, err)
	}

	pubURL := store.PublicURL(key)
	if pubURL != "https://media.cafnet.app/businesses/1/cover/test.jpg" {
		t.Fatalf("unexpected public url: %s", pubURL)
	}

	destKey := "businesses/1/gallery/test.jpg"
	if err := store.CopyObject(ctx, key, destKey); err != nil {
		t.Fatalf("failed to copy: %v", err)
	}

	if err := store.DeleteObject(ctx, key); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	exists, err = store.ObjectExists(ctx, key)
	if err != nil || exists {
		t.Fatalf("expected deleted object not to exist")
	}

	existsDest, err := store.ObjectExists(ctx, destKey)
	if err != nil || !existsDest {
		t.Fatalf("expected copied object to exist")
	}
}
