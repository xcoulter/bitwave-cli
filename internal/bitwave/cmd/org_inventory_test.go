package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestUSInventoryProfilesKeepBooksAndTaxSeparate(t *testing.T) {
	profiles := usInventoryProfiles()
	if len(profiles) != 2 {
		t.Fatalf("profiles = %#v", profiles)
	}
	books, err := inventoryProfileByID(usGAAPProfile)
	if err != nil {
		t.Fatal(err)
	}
	tax, err := inventoryProfileByID(usFederalTaxFIFOProfile)
	if err != nil {
		t.Fatal(err)
	}
	if books.Purpose != "books" || !books.Request.Impair || books.Request.Config.DefaultValuationStrategy != "gaap-fair-value" || books.Request.Config.CapitalizeTradingFees || books.Request.Config.ImpairmentMethodology != "org-default" {
		t.Fatalf("books = %#v", books)
	}
	if tax.Purpose != "tax" || tax.Request.Impair || !tax.Request.Config.CapitalizeTradingFees || tax.Request.Config.ImpairmentMethodology != "org-default" {
		t.Fatalf("tax = %#v", tax)
	}
	if books.Request.Config.InventoryMappingRule == nil || books.Request.Config.InventoryMappingRule.Type != "inventory-per-wallet" || tax.Request.Strategy.TaxStrategy != "FIFO" {
		t.Fatalf("mapping/strategy books=%#v tax=%#v", books.Request, tax.Request)
	}
}

func TestInventoryGuidancePromptsInsteadOfRejectingOtherJurisdictions(t *testing.T) {
	guidance, err := buildInventoryGuidance("UK", "IFRS", "tax", "2026-07-31")
	if err != nil {
		t.Fatal(err)
	}
	if guidance["schemaVersion"] != "2" || guidance["mustRecheckSources"] != true {
		t.Fatalf("guidance = %#v", guidance)
	}
	notes := guidance["jurisdictionNotes"].([]inventoryJurisdictionNote)
	if len(notes) != 2 || notes[0].ID != "IFRS" || notes[1].ID != "UK" {
		t.Fatalf("notes = %#v", notes)
	}
	for _, question := range guidance["questions"].([]inventoryGuidanceQuestion) {
		if question.ID == "jurisdiction" || question.ID == "framework" || question.ID == "effective_date" {
			t.Fatalf("resolved question was repeated: %#v", question)
		}
	}
}

func TestInventoryGuidanceReturnsGeneralFrameworkForUnknownJurisdiction(t *testing.T) {
	guidance, err := buildInventoryGuidance("DE", "", "tax", "")
	if err != nil {
		t.Fatal(err)
	}
	if guidance["status"] != "requires-user-input" {
		t.Fatalf("guidance = %#v", guidance)
	}
	warnings := strings.Join(guidance["warnings"].([]string), " ")
	if !strings.Contains(warnings, "No reviewed tax note is embedded for DE") {
		t.Fatalf("warnings = %q", warnings)
	}
}

func TestInventoryUpdateDateMatchesUIUpdateNow(t *testing.T) {
	now := time.Date(2026, time.August, 13, 1, 30, 0, 0, time.UTC)
	got, err := resolveInventoryUpdateDate("", "America/Los_Angeles", now)
	if err != nil || got != "2026-08-11" {
		t.Fatalf("date = %q err=%v", got, err)
	}
	got, err = resolveInventoryUpdateDate("2026-08-10", "America/Los_Angeles", now)
	if err != nil || got != "2026-08-10" {
		t.Fatalf("explicit date = %q err=%v", got, err)
	}
	if _, err := resolveInventoryUpdateDate("2026-08-12", "America/Los_Angeles", now); err == nil {
		t.Fatal("expected organization-local today to be rejected")
	}
}

func TestUSInventoryGuidanceIsAdvisoryAndRequiresVerification(t *testing.T) {
	for _, profile := range usInventoryProfiles() {
		if !profile.AdvisoryOnly || len(profile.Sources) < 4 || profile.LastReviewed == "" {
			t.Fatalf("profile = %#v", profile)
		}
		joined := strings.Join(append(profile.Confirmations, profile.Limitations...), " ")
		if !strings.Contains(strings.ToLower(joined), "confirm") || !strings.Contains(strings.ToLower(joined), "state") {
			t.Fatalf("guidance = %q", joined)
		}
	}
	if !strings.Contains(strings.ToLower(newOrgInventoryCmd().Long), "not legal") {
		t.Fatal("inventory help must retain the advice disclaimer")
	}
}
