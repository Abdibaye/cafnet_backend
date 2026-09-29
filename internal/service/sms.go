package service

import (
	"context"
	"sync"
)

// SMSProvider defines the abstraction for dispatching SMS verification messages.
type SMSProvider interface {
	SendOTP(ctx context.Context, phone, otpCode string) error
}

// MockSMSProvider is a thread-safe test/dev implementation that records sent OTPs in memory.
type MockSMSProvider struct {
	mu       sync.RWMutex
	messages map[string][]string
}

// NewMockSMSProvider creates a fresh in-memory SMS mock.
func NewMockSMSProvider() *MockSMSProvider {
	return &MockSMSProvider{
		messages: make(map[string][]string),
	}
}

// SendOTP records the sent OTP code for the target phone number.
func (m *MockSMSProvider) SendOTP(ctx context.Context, phone, otpCode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[phone] = append(m.messages[phone], otpCode)
	return nil
}

// LastSentOTP returns the most recent OTP sent to a given phone number.
func (m *MockSMSProvider) LastSentOTP(phone string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	codes := m.messages[phone]
	if len(codes) == 0 {
		return "", false
	}
	return codes[len(codes)-1], true
}

// Clear wipes all recorded SMS messages.
func (m *MockSMSProvider) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = make(map[string][]string)
}
