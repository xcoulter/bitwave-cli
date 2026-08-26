package orgreports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestManualImportContractsAndSignedUpload(t *testing.T) {
	var mu sync.Mutex
	operations := []string{}
	uploadHadAuthorization := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upload" {
			uploadHadAuthorization = r.Header.Get("Authorization") != ""
			if r.Method != http.MethodPut || r.Header.Get("Content-Type") != "text/csv" {
				t.Fatalf("upload method=%s content-type=%q", r.Method, r.Header.Get("Content-Type"))
			}
			return
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		var request struct {
			OperationName string         `json:"operationName"`
			Variables     map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		operations = append(operations, request.OperationName)
		mu.Unlock()
		switch request.OperationName {
		case "ImportsPage":
			_, _ = w.Write([]byte(`{"data":{"importsPage":{"items":[{"id":"imp-1","status":"completed","createdMS":10}],"total":1}}}`))
		case "GetImport":
			_, _ = w.Write([]byte(`{"data":{"getImport":{"id":"imp-1","status":"ready-to-import","rowErrorCount":{"validate":0,"run":0},"warningRowCount":0}}}`))
		case "CreateImport":
			_, _ = w.Write([]byte(`{"data":{"createImport":{"id":"imp-1","fileUploadUrl":"` + serverURLForRequest(r) + `/upload?signature=secret"}}}`))
		case "ValidateImport":
			_, _ = w.Write([]byte(`{"data":{"validateImport":"imp-1"}}`))
		case "RunImport":
			_, _ = w.Write([]byte(`{"data":{"runImport":"imp-1"}}`))
		case "ImportRowErrors":
			_, _ = w.Write([]byte(`{"data":{"getImport":{"rowErrors":[{"rowIndex":2,"rowId":"r3","stage":"validate","errors":["bad ticker"]}]}}}`))
		case "ImportPreview":
			_, _ = w.Write([]byte(`{"data":{"getImport":{"id":"imp-1","status":"preview-ready","previewLines":[{"index":0,"hasErrors":false,"errors":[],"data":{"id":"txn-1"}}]}}}`))
		default:
			t.Fatalf("operation = %q", request.OperationName)
		}
	}))
	defer server.Close()

	client := New(server.URL, func() (string, error) { return "token", nil })
	client.RulesMutationURL = server.URL
	ctx := context.Background()
	page, err := client.ImportsPage(ctx, "org-1", 0, 25)
	if err != nil || page.Total != 1 || page.Items[0].ID != "imp-1" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	record, err := client.Import(ctx, "org-1", "imp-1")
	if err != nil || record.Status != "ready-to-import" {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	created, err := client.CreateImport(ctx, "org-1")
	if err != nil || created.ID != "imp-1" {
		t.Fatalf("created=%#v err=%v", created, err)
	}
	file := filepath.Join(t.TempDir(), "transactions.csv")
	if err := os.WriteFile(file, []byte("id,time\na,2026-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := client.UploadImportFile(ctx, created.FileUploadURL, file); err != nil {
		t.Fatal(err)
	}
	if uploadHadAuthorization {
		t.Fatal("signed upload leaked the Bitwave Authorization header")
	}
	if err := client.ValidateImport(ctx, "org-1", "imp-1"); err != nil {
		t.Fatal(err)
	}
	if err := client.RunImport(ctx, "org-1", "imp-1"); err != nil {
		t.Fatal(err)
	}
	rowErrors, err := client.ImportRowErrors(ctx, "org-1", "imp-1", "validate", 0, 100)
	if err != nil || len(rowErrors) != 1 || rowErrors[0].RowIndex == nil || *rowErrors[0].RowIndex != 2 {
		t.Fatalf("rowErrors=%#v err=%v", rowErrors, err)
	}
	status, preview, err := client.ImportPreview(ctx, "org-1", "imp-1")
	if err != nil || status != "preview-ready" || len(preview) != 1 {
		t.Fatalf("status=%s preview=%#v err=%v", status, preview, err)
	}
	if len(operations) < 7 {
		t.Fatalf("operations = %#v", operations)
	}
}

// The test server's listener is the Host header for its own request. Reusing it
// builds a signed-upload URL without closing over server before initialization.
func serverURLForRequest(r *http.Request) string {
	return "http://" + r.Host
}

func TestImportsPageFallsBackOnlyForMissingSchema(t *testing.T) {
	pageCalls, listCalls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			OperationName string `json:"operationName"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		switch request.OperationName {
		case "ImportsPage":
			pageCalls++
			_, _ = w.Write([]byte(`{"errors":[{"message":"Cannot query field importsPage"}]}`))
		case "Imports":
			listCalls++
			_, _ = w.Write([]byte(`{"data":{"imports":[{"id":"a"},{"id":"b"},{"id":"c"}]}}`))
		}
	}))
	defer server.Close()
	client := New(server.URL, func() (string, error) { return "token", nil })
	client.RulesMutationURL = server.URL
	page, err := client.ImportsPage(context.Background(), "org-1", 1, 1)
	if err != nil || page.Total != 3 || len(page.Items) != 1 || page.Items[0].ID != "b" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	if pageCalls != 1 || listCalls != 1 {
		t.Fatalf("pageCalls=%d listCalls=%d", pageCalls, listCalls)
	}
}

func TestImportStatusFallsBackWhenPartialCountFieldsAreMissing(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if strings.Contains(request.Query, "rowsImported") {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Cannot query field rowsImported"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"getImport":{"id":"imp-1","status":"failed","importErrors":["boom"]}}}`))
	}))
	defer server.Close()
	client := New(server.URL, func() (string, error) { return "token", nil })
	client.RulesMutationURL = server.URL
	record, err := client.Import(context.Background(), "org-1", "imp-1")
	if err != nil || record.Status != "failed" || calls != 2 {
		t.Fatalf("record=%#v calls=%d err=%v", record, calls, err)
	}
}
