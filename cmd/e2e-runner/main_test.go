package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidateConfigRequiresFullMode(t *testing.T) {
	cfg := config{
		GatewayURL:     "http://localhost:8080",
		AdminURL:       "http://localhost:8081",
		AdminToken:     "admin",
		DatabaseURL:    "postgres://example",
		MaxRequests:    30,
		MaxTokens:      32,
		Model:          "chat-fast",
		ProjectID:      defaultProject,
		OrganizationID: defaultOrganization,
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig returned error: %v", err)
	}
	cfg.MaxRequests = 29
	if err := validateConfig(cfg); err == nil || !strings.Contains(err.Error(), "at least 30") {
		t.Fatalf("expected max request validation, got %v", err)
	}
}

func TestSelectSeedUsersRequiresTenDemoUsers(t *testing.T) {
	users := make([]adminUser, 0, 10)
	for i := 1; i <= 10; i++ {
		users = append(users, adminUser{UserID: fmt.Sprintf("user-%d", i), Email: fmt.Sprintf("user%02d@democorp.local", i)})
	}
	selected, err := selectSeedUsers(users)
	if err != nil {
		t.Fatalf("selectSeedUsers returned error: %v", err)
	}
	if len(selected) != 10 || selected[0].Email != "user01@democorp.local" || selected[9].Email != "user10@democorp.local" {
		t.Fatalf("unexpected selected users: %+v", selected)
	}
}

func TestAssertRequestResults(t *testing.T) {
	results := []requestResult{
		{StatusCode: 402, Budget0: true},
		{StatusCode: 200},
		{StatusCode: 200},
		{StatusCode: 200},
	}
	if err := assertRequestResults(results); err != nil {
		t.Fatalf("assertRequestResults returned error: %v", err)
	}
	results = append(results, requestResult{StatusCode: 500})
	if err := assertRequestResults(results); err == nil || !strings.Contains(err.Error(), "5xx") {
		t.Fatalf("expected 5xx error, got %v", err)
	}
}

func TestWithinOnePercent(t *testing.T) {
	if !withinOnePercent(1.00, 1.009) {
		t.Fatalf("expected values within tolerance")
	}
	if withinOnePercent(1.00, 1.02) {
		t.Fatalf("expected values outside tolerance")
	}
}
