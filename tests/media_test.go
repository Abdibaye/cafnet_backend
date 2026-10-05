package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

func TestMediaUpload_PresignedURLGeneration_Validation(t *testing.T) {
	h := setupHTTPTest(t)
	client, user := h.loginClient(t, "+251911223301", domain.RoleCustomer)

	// 1. Unauthenticated request rejected with 401
	unauthReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/media/uploads", strings.NewReader(`{}`))
	unauthReq.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(unauthReq)
	if err != nil {
		t.Fatalf("unauth request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauth, got %d", resp.StatusCode)
	}

	// 2. Invalid content types rejected with 400
	invalidTypes := []string{"image/svg+xml", "application/pdf", "text/plain", "image/gif", "application/x-executable"}
	for _, ct := range invalidTypes {
		payload, _ := json.Marshal(map[string]any{
			"target_type":  domain.TargetTypeSubmission,
			"content_type": ct,
			"file_size":    1024 * 1024,
		})
		res, err := client.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("post failed: %v", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for content_type %s, got %d", ct, res.StatusCode)
		}
	}

	// 3. Invalid file size rejected with 400
	invalidSizes := []int64{0, -100, 11 * 1024 * 1024}
	for _, sz := range invalidSizes {
		payload, _ := json.Marshal(map[string]any{
			"target_type":  domain.TargetTypeSubmission,
			"content_type": "image/jpeg",
			"file_size":    sz,
		})
		res, err := client.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("post failed: %v", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for file_size %d, got %d", sz, res.StatusCode)
		}
	}

	// 4. Valid upload initiation returns 201 with presigned URL
	validPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeSubmission,
		"content_type": "image/webp",
		"file_size":    500 * 1024,
	})
	res, err := client.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(validPayload))
	if err != nil {
		t.Fatalf("valid post failed: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("expected 201, got %d: %s", res.StatusCode, string(body))
	}

	var presignResp struct {
		Status string                         `json:"status"`
		Data   domain.PresignedUploadResponse `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&presignResp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if presignResp.Data.MediaID == "" {
		t.Fatalf("expected valid MediaID")
	}
	if presignResp.Data.UploadURL == "" {
		t.Fatalf("expected non-empty UploadURL")
	}
	if !strings.HasPrefix(presignResp.Data.ObjectKey, "submissions/") {
		t.Fatalf("expected object_key to start with submissions/, got %s", presignResp.Data.ObjectKey)
	}

	// Verify database record created with PENDING status
	mediaRecord, err := h.mediaRepo.GetByID(context.Background(), presignResp.Data.MediaID)
	if err != nil {
		t.Fatalf("failed to fetch created media record: %v", err)
	}
	if mediaRecord.Status != domain.MediaStatusPending {
		t.Fatalf("expected status PENDING, got %s", mediaRecord.Status)
	}
	if mediaRecord.CreatedByUserID != user.ID {
		t.Fatalf("expected user %s, got %s", user.ID, mediaRecord.CreatedByUserID)
	}
}

func TestMediaUpload_CompletionVerification(t *testing.T) {
	h := setupHTTPTest(t)
	client, _ := h.loginClient(t, "+251911223302", domain.RoleCustomer)

	// Step 1: Initiate upload
	validPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeSubmission,
		"content_type": "image/jpeg",
		"file_size":    200 * 1024,
	})
	res, err := client.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(validPayload))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	var presignResp struct {
		Status string                         `json:"status"`
		Data   domain.PresignedUploadResponse `json:"data"`
	}
	json.NewDecoder(res.Body).Decode(&presignResp)
	res.Body.Close()

	mediaID := presignResp.Data.MediaID

	// Step 2: Delete mock object from storage to test completion failure when file was not uploaded
	_ = h.storageClient.DeleteObject(context.Background(), presignResp.Data.ObjectKey)

	compRes, err := client.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, mediaID), "application/json", nil)
	if err != nil {
		t.Fatalf("complete post failed: %v", err)
	}
	compRes.Body.Close()
	if compRes.StatusCode != http.StatusBadRequest && compRes.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 400 or 404 when storage object missing, got %d", compRes.StatusCode)
	}

	// Step 3: Put object in storage (simulating browser direct upload)
	h.storageClient.PutMockObject(presignResp.Data.ObjectKey, "image/jpeg", 200*1024)

	// Step 4: Complete again -> should succeed with 200 and status READY
	compRes2, err := client.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, mediaID), "application/json", nil)
	if err != nil {
		t.Fatalf("complete post failed: %v", err)
	}
	defer compRes2.Body.Close()
	if compRes2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(compRes2.Body)
		t.Fatalf("expected 200, got %d: %s", compRes2.StatusCode, string(body))
	}

	var compResult struct {
		Status string                        `json:"status"`
		Data   domain.CompleteUploadResponse `json:"data"`
	}
	if err := json.NewDecoder(compRes2.Body).Decode(&compResult); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if compResult.Data.Media.Status != domain.MediaStatusReady {
		t.Fatalf("expected status READY, got %s", compResult.Data.Media.Status)
	}
}

func TestMedia_IDOR_Protection(t *testing.T) {
	h := setupHTTPTest(t)
	userAClient, _ := h.loginClient(t, "+251911223303", domain.RoleCustomer)
	userBClient, _ := h.loginClient(t, "+251911223304", domain.RoleCustomer)

	// User A creates an upload
	validPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeSubmission,
		"content_type": "image/png",
		"file_size":    100 * 1024,
	})
	res, _ := userAClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(validPayload))
	var presignResp struct {
		Data domain.PresignedUploadResponse `json:"data"`
	}
	json.NewDecoder(res.Body).Decode(&presignResp)
	res.Body.Close()

	mediaID := presignResp.Data.MediaID

	// User B tries to complete User A's media -> 403 Forbidden
	compRes, _ := userBClient.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, mediaID), "application/json", nil)
	compRes.Body.Close()
	if compRes.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for User B completing User A's media, got %d", compRes.StatusCode)
	}

	// User B tries to delete User A's media -> 403 Forbidden
	delReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/media/%s", h.server.URL, mediaID), nil)
	delRes, _ := userBClient.Do(delReq)
	delRes.Body.Close()
	if delRes.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for User B deleting User A's media, got %d", delRes.StatusCode)
	}

	// User A can delete their own pending media -> 200 OK or 204 No Content
	delReqA, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/media/%s", h.server.URL, mediaID), nil)
	delResA, _ := userAClient.Do(delReqA)
	delResA.Body.Close()
	if delResA.StatusCode != http.StatusOK && delResA.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 200 or 204 for User A deleting their own media, got %d", delResA.StatusCode)
	}
}

func TestMedia_Submission_Lifecycle_Transition(t *testing.T) {
	h := setupHTTPTest(t)
	custClient, custUser := h.loginClient(t, "+251911223305", domain.RoleCustomer)
	adminClient, _ := h.loginClient(t, "+251911223306", domain.RoleAdmin)

	bizName := fmt.Sprintf("Test Roastery %d", time.Now().UnixNano())
	// 1. Attempt submission without media_id -> rejected 400
	subPayloadNoMedia, _ := json.Marshal(map[string]any{
		"business_name": bizName,
		"category":      "CAFE",
		"area":          "Bole",
		"payload":       map[string]any{"city": "Addis Ababa"},
	})
	res1, _ := custClient.Post(h.server.URL+"/api/v1/submissions", "application/json", bytes.NewReader(subPayloadNoMedia))
	res1.Body.Close()
	if res1.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 when submitting business without cover image, got %d", res1.StatusCode)
	}

	// 2. Upload media and finalize it
	uploadPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeSubmission,
		"content_type": "image/jpeg",
		"file_size":    150 * 1024,
	})
	res2, _ := custClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(uploadPayload))
	var presignResp struct {
		Data domain.PresignedUploadResponse `json:"data"`
	}
	json.NewDecoder(res2.Body).Decode(&presignResp)
	res2.Body.Close()

	// Ensure mock object exists in storage
	h.storageClient.PutMockObject(presignResp.Data.ObjectKey, "image/jpeg", 150*1024)

	// Complete upload
	compRes, _ := custClient.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, presignResp.Data.MediaID), "application/json", nil)
	compRes.Body.Close()
	if compRes.StatusCode != http.StatusOK {
		t.Fatalf("complete failed with %d", compRes.StatusCode)
	}

	// 3. Create submission with media_id -> succeeds with 201
	subPayload, _ := json.Marshal(map[string]any{
		"business_name": bizName,
		"category":      "CAFE",
		"area":          "Bole",
		"media_id":      presignResp.Data.MediaID,
		"payload":       map[string]any{"city": "Addis Ababa"},
	})
	res3, _ := custClient.Post(h.server.URL+"/api/v1/submissions", "application/json", bytes.NewReader(subPayload))
	if res3.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res3.Body)
		t.Fatalf("submission failed (%d): %s", res3.StatusCode, string(body))
	}
	var subResp struct {
		Data struct {
			ID           domain.SubmissionID `json:"id"`
			CoverMediaID *domain.MediaID     `json:"cover_media_id"`
		} `json:"data"`
	}
	json.NewDecoder(res3.Body).Decode(&subResp)
	res3.Body.Close()

	if subResp.Data.CoverMediaID == nil || *subResp.Data.CoverMediaID != presignResp.Data.MediaID {
		t.Fatalf("expected submission cover_media_id to match")
	}

	// 4. Admin approves submission
	approveRes, _ := adminClient.Post(fmt.Sprintf("%s/api/v1/admin/submissions/%s/approve", h.server.URL, subResp.Data.ID), "application/json", nil)
	if approveRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(approveRes.Body)
		t.Fatalf("admin approval failed (%d): %s", approveRes.StatusCode, string(body))
	}
	var bizResp struct {
		Data domain.Business `json:"data"`
	}
	json.NewDecoder(approveRes.Body).Decode(&bizResp)
	approveRes.Body.Close()

	// Verify approved business response has cover image populated
	if bizResp.Data.CoverImage == nil {
		t.Fatalf("expected approved business response cover_image to be populated")
	}

	createdBiz, err := h.businessRepo.GetByID(context.Background(), bizResp.Data.ID)
	if err != nil {
		t.Fatalf("failed to fetch created business: %v", err)
	}

	coverRecord, err := h.mediaRepo.GetBusinessCover(context.Background(), createdBiz.ID)
	if err != nil || coverRecord == nil {
		t.Fatalf("expected business cover to exist in media repo, err: %v", err)
	}

	// Verify media record now has business_id set and type BUSINESS_COVER
	mediaAfterApproval, err := h.mediaRepo.GetByID(context.Background(), presignResp.Data.MediaID)
	if err != nil {
		t.Fatalf("failed to fetch media: %v", err)
	}
	if mediaAfterApproval.BusinessID == nil || *mediaAfterApproval.BusinessID != createdBiz.ID {
		t.Fatalf("expected media business_id to be %v, got %v", createdBiz.ID, mediaAfterApproval.BusinessID)
	}
	if mediaAfterApproval.MediaType != domain.MediaTypeBusinessCover {
		t.Fatalf("expected media type BUSINESS_COVER, got %s", mediaAfterApproval.MediaType)
	}

	_ = custUser
}

func TestMedia_Business_Gallery_And_Cover(t *testing.T) {
	h := setupHTTPTest(t)
	mgrClient, mgrUser := h.loginClient(t, "+251911223307", domain.RoleCustomer)
	otherClient, _ := h.loginClient(t, "+251911223308", domain.RoleCustomer)

	biz := h.createBusiness(t, "Bole Cafe Gallery Test", domain.PublicationStatusPublished)
	_ = h.ownershipRepo.AddManager(context.Background(), &domain.BusinessManager{
		BusinessID:  biz.ID,
		UserID:      mgrUser.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})

	bizIDStr := fmt.Sprintf("%d", biz.ID)

	// 1. Non-manager cannot initiate upload for business -> 403
	nonMgrPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeBusinessGallery,
		"target_id":    bizIDStr,
		"content_type": "image/jpeg",
		"file_size":    100 * 1024,
	})
	res0, _ := otherClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(nonMgrPayload))
	res0.Body.Close()
	if res0.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-manager upload initiation, got %d", res0.StatusCode)
	}

	// 2. Manager uploads 2 gallery images
	var mediaIDs []domain.MediaID
	for i := 1; i <= 2; i++ {
		uploadPayload, _ := json.Marshal(map[string]any{
			"target_type":  domain.TargetTypeBusinessGallery,
			"target_id":    bizIDStr,
			"content_type": "image/jpeg",
			"file_size":    int64(i * 100 * 1024),
		})
		res, _ := mgrClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(uploadPayload))
		if res.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(res.Body)
			t.Fatalf("failed to create upload %d: %s", i, string(body))
		}
		var presignResp struct {
			Data domain.PresignedUploadResponse `json:"data"`
		}
		json.NewDecoder(res.Body).Decode(&presignResp)
		res.Body.Close()

		h.storageClient.PutMockObject(presignResp.Data.ObjectKey, "image/jpeg", int64(i*100*1024))

		compRes, _ := mgrClient.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, presignResp.Data.MediaID), "application/json", nil)
		compRes.Body.Close()
		if compRes.StatusCode != http.StatusOK {
			t.Fatalf("failed to complete upload %d", i)
		}
		mediaIDs = append(mediaIDs, presignResp.Data.MediaID)
	}

	// 3. Manager lists business media
	listRes, _ := mgrClient.Get(fmt.Sprintf("%s/api/v1/businesses/%s/media", h.server.URL, bizIDStr))
	if listRes.StatusCode != http.StatusOK {
		t.Fatalf("failed to list media, got %d", listRes.StatusCode)
	}
	var mediaList struct {
		Data []domain.BusinessMedia `json:"data"`
	}
	json.NewDecoder(listRes.Body).Decode(&mediaList)
	listRes.Body.Close()

	if len(mediaList.Data) != 2 {
		t.Fatalf("expected 2 gallery images, got %d", len(mediaList.Data))
	}

	// 4. Manager sets gallery image 2 as cover
	coverPayload, _ := json.Marshal(map[string]any{
		"media_id": mediaIDs[1],
	})
	coverReq, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/businesses/%s/media/cover", h.server.URL, bizIDStr), bytes.NewReader(coverPayload))
	coverReq.Header.Set("Content-Type", "application/json")
	coverRes, err := mgrClient.Do(coverReq)
	if err != nil {
		t.Fatalf("set cover request failed: %v", err)
	}
	if coverRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(coverRes.Body)
		t.Fatalf("failed to set cover (%d): %s", coverRes.StatusCode, string(body))
	}
	coverRes.Body.Close()

	// 5. Manager reorders gallery
	reorderPayload, _ := json.Marshal(map[string]any{
		"media_ids": []domain.MediaID{mediaIDs[1], mediaIDs[0]},
	})
	reorderReq, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/v1/businesses/%s/media/gallery/reorder", h.server.URL, bizIDStr), bytes.NewReader(reorderPayload))
	reorderReq.Header.Set("Content-Type", "application/json")
	reorderRes, err := mgrClient.Do(reorderReq)
	if err != nil {
		t.Fatalf("reorder request failed: %v", err)
	}
	if reorderRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(reorderRes.Body)
		t.Fatalf("failed to reorder gallery (%d): %s", reorderRes.StatusCode, string(body))
	}
	reorderRes.Body.Close()

	// Non-manager cannot delete gallery image -> 403
	delReqOther, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/media/%s", h.server.URL, mediaIDs[0]), nil)
	delResOther, _ := otherClient.Do(delReqOther)
	delResOther.Body.Close()
	if delResOther.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-manager deleting business media, got %d", delResOther.StatusCode)
	}

	// Manager CAN delete gallery image -> 200 OK or 204 No Content
	delReqMgr, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/media/%s", h.server.URL, mediaIDs[0]), nil)
	delResMgr, _ := mgrClient.Do(delReqMgr)
	delResMgr.Body.Close()
	if delResMgr.StatusCode != http.StatusOK && delResMgr.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 200 or 204 for manager deleting media, got %d", delResMgr.StatusCode)
	}
}

func TestMedia_MenuItem_Lifecycle(t *testing.T) {
	h := setupHTTPTest(t)
	mgrClient, mgrUser := h.loginClient(t, "+251911223309", domain.RoleCustomer)
	otherClient, _ := h.loginClient(t, "+251911223310", domain.RoleCustomer)

	biz := h.createBusiness(t, "Menu Item Media Cafe", domain.PublicationStatusPublished)
	_ = h.ownershipRepo.AddManager(context.Background(), &domain.BusinessManager{
		BusinessID:  biz.ID,
		UserID:      mgrUser.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})

	// Create menu category and item
	cat := &domain.MenuCategory{
		BusinessID: biz.ID,
		Name:       "Espresso Bar",
		SortOrder:  1,
	}
	_ = h.menuRepo.CreateCategory(context.Background(), cat)
	item := &domain.MenuItem{
		CategoryID:    cat.ID,
		BusinessID:    biz.ID,
		Name:          "Signature Macchiato",
		Description:   "Rich espresso with steamed milk foam",
		PriceETBMinor: 6500,
		IsAvailable:   true,
	}
	_ = h.menuRepo.CreateItem(context.Background(), item)

	// 1. Non-manager upload initiation rejected 403
	nonMgrPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeMenuItem,
		"target_id":    string(item.ID),
		"content_type": "image/jpeg",
		"file_size":    50 * 1024,
	})
	res0, _ := otherClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(nonMgrPayload))
	res0.Body.Close()
	if res0.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-manager upload for menu item, got %d", res0.StatusCode)
	}

	// 2. Manager uploads menu item image
	mgrPayload, _ := json.Marshal(map[string]any{
		"target_type":  domain.TargetTypeMenuItem,
		"target_id":    string(item.ID),
		"content_type": "image/jpeg",
		"file_size":    50 * 1024,
	})
	res1, _ := mgrClient.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(mgrPayload))
	if res1.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for manager menu item upload, got %d", res1.StatusCode)
	}
	var presignResp struct {
		Data domain.PresignedUploadResponse `json:"data"`
	}
	json.NewDecoder(res1.Body).Decode(&presignResp)
	res1.Body.Close()

	// Simulate upload to storage and complete
	h.storageClient.PutMockObject(presignResp.Data.ObjectKey, "image/jpeg", 50*1024)
	compRes, _ := mgrClient.Post(fmt.Sprintf("%s/api/v1/media/uploads/%s/complete", h.server.URL, presignResp.Data.MediaID), "application/json", nil)
	compRes.Body.Close()
	if compRes.StatusCode != http.StatusOK {
		t.Fatalf("complete failed with %d", compRes.StatusCode)
	}

	// Verify menu item record updated with image_media_id
	updatedItem, err := h.menuRepo.GetItemByID(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to fetch menu item: %v", err)
	}
	if updatedItem.ImageMediaID == nil || *updatedItem.ImageMediaID != presignResp.Data.MediaID {
		t.Fatalf("expected menu item to have image_media_id set")
	}

	// 3. Non-manager delete menu item media rejected 403
	delReqOther, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/menu/items/%s/image", h.server.URL, item.ID), nil)
	delResOther, _ := otherClient.Do(delReqOther)
	delResOther.Body.Close()
	if delResOther.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-manager deleting menu item media, got %d", delResOther.StatusCode)
	}

	// 4. Manager deletes menu item media -> 200 OK or 204 No Content
	delReqMgr, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/menu/items/%s/image", h.server.URL, item.ID), nil)
	delResMgr, _ := mgrClient.Do(delReqMgr)
	delResMgr.Body.Close()
	if delResMgr.StatusCode != http.StatusOK && delResMgr.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 200 or 204 for manager deleting menu item image, got %d", delResMgr.StatusCode)
	}

	// Verify menu item image_media_id cleared
	clearedItem, err := h.menuRepo.GetItemByID(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("failed to fetch item: %v", err)
	}
	if clearedItem.ImageMediaID != nil {
		t.Fatalf("expected image_media_id to be cleared")
	}
}
