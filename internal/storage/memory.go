package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryStorage is an in-memory implementation of ObjectStorage for testing and environments without Tigris credentials.
type MemoryStorage struct {
	mu        sync.RWMutex
	objects   map[string]*ObjectMetadata
	publicURL string
}

// NewMemoryStorage initializes an in-memory ObjectStorage.
func NewMemoryStorage(publicURL string) *MemoryStorage {
	if publicURL == "" {
		publicURL = "https://mock.storage.cafnet.local"
	}
	publicURL = strings.TrimRight(publicURL, "/")
	return &MemoryStorage{
		objects:   make(map[string]*ObjectMetadata),
		publicURL: publicURL,
	}
}

func (m *MemoryStorage) CreatePresignedUpload(ctx context.Context, key string, contentType string, contentLength int64, lifetime time.Duration) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// In memory, register an impending object so completion or mock tests can find it
	m.objects[key] = &ObjectMetadata{
		Key:           key,
		ContentType:   contentType,
		ContentLength: contentLength,
		ETag:          `"mock-etag-12345"`,
		LastModified:  time.Now(),
	}

	return fmt.Sprintf("%s/upload-mock/%s?expires=%d", m.publicURL, key, time.Now().Add(lifetime).Unix()), nil
}

func (m *MemoryStorage) CreatePresignedDownload(ctx context.Context, key string, lifetime time.Duration) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.objects[key]; !exists {
		return "", ErrObjectNotFound
	}
	return fmt.Sprintf("%s/%s?signed=%d", m.publicURL, key, time.Now().Add(lifetime).Unix()), nil
}

func (m *MemoryStorage) HeadObject(ctx context.Context, key string) (*ObjectMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	meta, exists := m.objects[key]
	if !exists {
		return nil, ErrObjectNotFound
	}
	return meta, nil
}

func (m *MemoryStorage) ObjectExists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.objects[key]
	return exists, nil
}

func (m *MemoryStorage) DeleteObject(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.objects, key)
	return nil
}

func (m *MemoryStorage) CopyObject(ctx context.Context, srcKey, destKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meta, exists := m.objects[srcKey]
	if !exists {
		return ErrObjectNotFound
	}

	m.objects[destKey] = &ObjectMetadata{
		Key:           destKey,
		ContentType:   meta.ContentType,
		ContentLength: meta.ContentLength,
		ETag:          meta.ETag,
		LastModified:  time.Now(),
	}
	return nil
}

func (m *MemoryStorage) PublicURL(key string) string {
	cleanKey := strings.TrimLeft(key, "/")
	return fmt.Sprintf("%s/%s", m.publicURL, cleanKey)
}

// PutMockObject allows unit tests to simulate direct browser upload completion.
func (m *MemoryStorage) PutMockObject(key string, contentType string, contentLength int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.objects[key] = &ObjectMetadata{
		Key:           key,
		ContentType:   contentType,
		ContentLength: contentLength,
		ETag:          `"mock-etag"`,
		LastModified:  time.Now(),
	}
}
