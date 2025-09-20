package types_test

import (
	"testing"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

func TestNewLogEntry(t *testing.T) {
	// Test with dynamic fields
	fields := map[string]interface{}{
		"user_id": 123,
		"action":  "login",
	}

	log, err := types.NewLogEntry(types.Info, "auth", "localhost", "User logged in", "", fields)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check mandatory fields
	if log.Level != types.Info {
		t.Errorf("expected level %v, got %v", types.Info, log.Level)
	}
	if log.Service != "auth" {
		t.Errorf("expected service 'auth', got %v", log.Service)
	}
	if log.Host != "localhost" {
		t.Errorf("expected host 'localhost', got %v", log.Host)
	}

	// Check dynamic fields
	if val, ok := log.Properties["user_id"]; !ok || val != 123 {
		t.Errorf("expected user_id 123, got %v", val)
	}

	if val, ok := log.Properties["action"]; !ok || val != "login" {
		t.Errorf("expected action 'login', got %v", val)
	}

	// Test empty Msg / StackTrace
	log2, _ := types.NewLogEntry(types.Debug, "metrics", "localhost", "", "", nil)
	if log2.Message != "" {
		t.Errorf("expected empty Message, got %v", log2.Message)
	}
	if log2.StackTrace != "" {
		t.Errorf("expected empty StackTrace, got %v", log2.StackTrace)
	}
}

func TestIsValidLogLevel(t *testing.T) {
	if !types.IsValidLogLevel(types.Info) || !types.IsValidLogLevel(types.Debug) || !types.IsValidLogLevel(types.Error) {
		t.Errorf("expected levels to be valid")
	}
	if types.IsValidLogLevel("TRACE") {
		t.Errorf("TRACE should not be valid")
	}
}

func TestPropertiesHelpers(t *testing.T) {
	log, _ := types.NewLogEntry(types.Info, "auth", "localhost", "test", "", nil)

	// Set field
	log.SetProperty("request_id", "abc123")

	val, ok := log.GetProperty("request_id")
	if !ok || val != "abc123" {
		t.Errorf("expected request_id 'abc123', got %v", val)
	}

	// Get non-existent Property
	_, ok = log.GetProperty("nonexistent")
	if ok {
		t.Errorf("expected nonexistent Property to return false")
	}
}
