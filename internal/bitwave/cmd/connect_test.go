package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bitwave-io/bitwave-cli/internal/orgctx"
)

func TestConnectUsesExistingTokenAndSelectsVerifiedOrg(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BITWAVE_TOKEN", "test-token")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v3/orgs/org-123" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing bearer token")
		}
		_, _ = writer.Write([]byte(`{"id":"org-123","name":"Acme"}`))
	}))
	defer server.Close()

	previousBaseURL := defaultCoreBaseURL
	previousTokenFlag := tokenFlag
	defaultCoreBaseURL = server.URL
	tokenFlag = ""
	t.Cleanup(func() {
		defaultCoreBaseURL = previousBaseURL
		tokenFlag = previousTokenFlag
	})

	var output bytes.Buffer
	command := newConnectCmd()
	command.SetOut(&output)
	command.SetArgs([]string{"org-123"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Connected to Acme (org-123).") {
		t.Fatalf("unexpected output: %s", output.String())
	}
	active, err := orgctx.Load()
	if err != nil {
		t.Fatal(err)
	}
	if active.OrgID != "org-123" || active.OrgName != "Acme" {
		t.Fatalf("unexpected active org: %#v", active)
	}
}

func TestConnectDoesNotSelectUnverifiedOrg(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("BITWAVE_TOKEN", "test-token")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()

	previousBaseURL := defaultCoreBaseURL
	previousTokenFlag := tokenFlag
	defaultCoreBaseURL = server.URL
	tokenFlag = ""
	t.Cleanup(func() {
		defaultCoreBaseURL = previousBaseURL
		tokenFlag = previousTokenFlag
	})

	command := newConnectCmd()
	command.SetArgs([]string{"org-123"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "verify access") {
		t.Fatalf("expected verification error, got %v", err)
	}
	if _, loadErr := orgctx.Load(); loadErr == nil {
		t.Fatal("unverified org was saved")
	}
}
