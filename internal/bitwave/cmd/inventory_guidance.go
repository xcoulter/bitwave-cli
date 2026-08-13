package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const inventoryGuidanceReviewed = "2026-08-13"

type inventoryGuidanceQuestion struct {
	ID            string   `json:"id"`
	Prompt        string   `json:"prompt"`
	Why           string   `json:"why"`
	Options       []string `json:"options,omitempty"`
	AllowFreeform bool     `json:"allowFreeform"`
	Required      bool     `json:"required"`
}

type inventoryStrategyGuidance struct {
	ID               string   `json:"id"`
	Meaning          string   `json:"meaning"`
	CommonUses       []string `json:"commonUses,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	NativeInBitwave  bool     `json:"nativeInBitwave"`
	RequiresEvidence bool     `json:"requiresEvidence,omitempty"`
}

type inventoryJurisdictionNote struct {
	ID                    string                    `json:"id"`
	Scope                 string                    `json:"scope"`
	Summary               []string                  `json:"summary"`
	BitwaveStartingPoints []string                  `json:"bitwaveStartingPoints,omitempty"`
	Gaps                  []string                  `json:"gaps,omitempty"`
	Sources               []inventoryGuidanceSource `json:"sources"`
	LastReviewed          string                    `json:"lastReviewed"`
}

func buildInventoryGuidance(jurisdiction, framework, purpose, effectiveDate string) (map[string]any, error) {
	validPurposes := map[string]bool{"": true, "books": true, "tax": true, "management": true, "reconciliation": true}
	if !validPurposes[purpose] {
		return nil, errors.New("--purpose must be books, tax, management, or reconciliation")
	}
	if effectiveDate != "" {
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			return nil, errors.New("--effective-date must be a valid calendar date in YYYY-MM-DD format")
		}
	}
	framework = normalizeInventoryFramework(framework)
	if framework != "" && framework != "US-GAAP" && framework != "IFRS" {
		return nil, fmt.Errorf("unsupported --framework %q; use US-GAAP or IFRS, or omit it so the LLM asks", framework)
	}

	questions := inventoryDecisionQuestions(jurisdiction, framework, purpose, effectiveDate)
	profiles := []inventoryProfile{}
	if jurisdiction == "US" || framework == "US-GAAP" {
		profiles = usInventoryProfiles()
		if purpose != "" {
			filtered := make([]inventoryProfile, 0, len(profiles))
			for _, profile := range profiles {
				if profile.Purpose == purpose {
					filtered = append(filtered, profile)
				}
			}
			profiles = filtered
		}
	}
	status := "ready-for-professional-review"
	if len(questions) > 0 {
		status = "requires-user-input"
	}
	warnings := []string{
		"Location does not determine reporting framework, tax treatment, entity classification, or asset scope.",
		"Re-check linked primary sources and Bitwave capabilities at execution time; embedded guidance can become stale.",
		"A Bitwave strategy name does not prove that the treatment is permitted for the user's facts and jurisdiction.",
		"Show assumptions, accept user overrides, and obtain qualified professional approval before reliance.",
	}
	if jurisdiction != "" && !knownInventoryJurisdiction(jurisdiction) {
		warnings = append(warnings, "No reviewed tax note is embedded for "+jurisdiction+"; consult current local primary authority rather than mapping the country to a strategy.")
	}
	return map[string]any{
		"schemaVersion": "2", "status": status,
		"inputs":     map[string]string{"jurisdiction": jurisdiction, "framework": framework, "purpose": purpose, "effectiveDate": effectiveDate},
		"disclaimer": "Informational guidance only; not legal, tax, accounting, or financial advice. Verify current primary sources and obtain qualified professional approval.",
		"llmInstructions": []string{
			"Ask only unresolved questions and do so in a short conversational sequence.",
			"Keep financial-statement books, tax, management, and reconciliation views separate.",
			"Explain proposed settings, assumptions, unsupported mechanics, and effective date before creation.",
			"Prefer the organization's approved pricing default unless a documented alternative policy is supplied.",
			"Record answers, source URLs and access dates, selected settings, overrides, approver, and policy effective date.",
			"Never present an optimization as legally permitted merely because Bitwave can calculate it.",
		},
		"questions": questions, "pickingStrategies": inventoryPickingStrategies(), "valuationStrategies": inventoryValuationStrategies(),
		"pricingMethodologies": inventoryPricingMethodologies(), "jurisdictionNotes": selectedInventoryGuidanceNotes(jurisdiction, framework),
		"reviewedProfiles": profiles, "warnings": warnings, "lastReviewed": inventoryGuidanceReviewed, "mustRecheckSources": true,
	}, nil
}

func normalizeInventoryFramework(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "GAAP", "US GAAP", "US_GAAP", "USGAAP":
		return "US-GAAP"
	default:
		return value
	}
}

func inventoryDecisionQuestions(jurisdiction, framework, purpose, effectiveDate string) []inventoryGuidanceQuestion {
	questions := []inventoryGuidanceQuestion{}
	add := func(condition bool, q inventoryGuidanceQuestion) {
		if condition {
			questions = append(questions, q)
		}
	}
	add(purpose == "", inventoryGuidanceQuestion{ID: "purpose", Prompt: "Is this view for books, tax, management reporting, or balance reconciliation?", Why: "Each purpose can require different valuation and lot policies.", Options: []string{"books", "tax", "management", "reconciliation"}, Required: true})
	add(framework == "" && purpose != "tax" && purpose != "reconciliation", inventoryGuidanceQuestion{ID: "framework", Prompt: "Which financial-reporting framework does the entity apply?", Why: "US GAAP and IFRS can produce materially different carrying values and fee treatment.", Options: []string{"US-GAAP", "IFRS", "other", "not-applicable"}, AllowFreeform: true, Required: purpose == "books"})
	add(jurisdiction == "" && purpose != "books" && purpose != "reconciliation", inventoryGuidanceQuestion{ID: "jurisdiction", Prompt: "Which country and subnational jurisdictions govern this view?", Why: "Tax lot identification, pooling, character, and reporting rules are jurisdiction-specific.", AllowFreeform: true, Required: purpose == "tax"})
	add(true, inventoryGuidanceQuestion{ID: "entity", Prompt: "What is the entity and taxpayer type, and is industry-specific guidance applicable?", Why: "Individuals, corporations, funds, broker-traders, dealers, and investment companies can follow different rules.", AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "asset_scope", Prompt: "Which assets and activities belong in this view?", Why: "Fungible tokens, NFTs, issued tokens, wrapped assets, staking, DeFi, and derivatives can require different treatment.", AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "holding_purpose", Prompt: "Are assets held for treasury, investment, ordinary-course sale, broker-trading, or mixed purposes?", Why: "Holding purpose can change the applicable accounting and tax treatment.", Options: []string{"treasury", "long-term-investment", "ordinary-course-sale", "broker-trading", "mixed"}, AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "lot_policy", Prompt: "Which lot-selection or pooling policy is approved, and what evidence supports it?", Why: "Specific identification and pooling can require records or matching rules.", Options: []string{"FIFO", "LIFO", "cost-average", "specific-identification", "balance-only", "undecided"}, AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "inventory_mapping", Prompt: "Should basis be tracked per wallet/account, by inventory group, or in a jurisdictional pool?", Why: "The inventory boundary can materially change gains and losses.", Options: []string{"per-wallet", "inventory-groups", "jurisdictional-pool", "undecided"}, AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "fees", Prompt: "How should acquisition, disposal, trade, transfer, and gas fees be treated?", Why: "Different fee types and reporting frameworks do not necessarily receive the same treatment.", AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "pricing", Prompt: "Does the organization have an approved pricing source and methodology?", Why: "Market observations produce different adjustments; use organization default when approved.", Options: []string{"org-default", "documented-override", "undecided"}, AllowFreeform: true, Required: true})
	add(effectiveDate == "", inventoryGuidanceQuestion{ID: "effective_date", Prompt: "What is the policy effective date and latest reconciled close date?", Why: "Rules change, and running beyond reconciled data can compromise reports.", AllowFreeform: true, Required: true})
	add(true, inventoryGuidanceQuestion{ID: "approval", Prompt: "Who will approve the proposed policy and configuration?", Why: "The LLM provides guidance; a qualified professional remains responsible.", AllowFreeform: true, Required: true})
	return questions
}

func inventoryPickingStrategies() []inventoryStrategyGuidance {
	return []inventoryStrategyGuidance{
		{ID: "FIFO", Meaning: "Disposes the oldest available acquisition lots first.", CommonUses: []string{"Operational default", "U.S. tax fallback without valid specific identification"}, Warnings: []string{"Not mandated by FASB", "Does not reproduce UK matching rules"}, NativeInBitwave: true},
		{ID: "LIFO", Meaning: "Disposes the newest available acquisition lots first.", Warnings: []string{"Prohibited for interchangeable IAS 2 inventory", "Tax availability is jurisdiction-specific"}, NativeInBitwave: true},
		{ID: "cost-average", Meaning: "Pools interchangeable units and assigns weighted-average cost to disposals.", CommonUses: []string{"Canadian ACB starting point", "IAS 2 weighted average"}, Warnings: []string{"Does not implement UK same-day and 30-day overlays"}, NativeInBitwave: true},
		{ID: "specific-identification", Meaning: "Selects lots using documented holding-period and gain/loss priorities.", CommonUses: []string{"Documented tax-lot elections", "HIFO-like optimization"}, Warnings: []string{"May require timely identification and substantiating records"}, NativeInBitwave: true, RequiresEvidence: true},
		{ID: "balance-only", Meaning: "Tracks quantities without disposal cost basis or realized gains and losses.", CommonUses: []string{"Reconciliation"}, Warnings: []string{"Not a tax or financial-statement gain/loss view"}, NativeInBitwave: true},
	}
}

func inventoryValuationStrategies() []inventoryStrategyGuidance {
	return []inventoryStrategyGuidance{
		{ID: "historical-cost", Meaning: "Keeps unsold holdings at original carrying cost without periodic fair-value remeasurement.", CommonUses: []string{"Tax cost-basis views"}, NativeInBitwave: true},
		{ID: "gaap-impairment", Meaning: "Legacy write-down-only model without subsequent recovery.", Warnings: []string{"Not current U.S. GAAP for assets within ASU 2023-08 scope"}, NativeInBitwave: true},
		{ID: "gaap-fair-value", Meaning: "Remeasures qualifying crypto assets to fair value and records unrealized changes without changing disposal lot basis.", Warnings: []string{"Scope criteria must be satisfied", "Acquisition costs are generally expensed absent industry-specific guidance"}, NativeInBitwave: true},
		{ID: "ifrs-impairment", Meaning: "IAS 38-style cost and impairment model with qualifying reversals capped at the no-impairment carrying amount.", Warnings: []string{"Decide IAS 2 versus IAS 38 classification first"}, NativeInBitwave: true},
		{ID: "ifrs-impairment-revalued", Meaning: "Allows approved revaluation above cost in addition to impairment mechanics.", Warnings: []string{"IAS 38 revaluation requires an active market"}, NativeInBitwave: true, RequiresEvidence: true},
		{ID: "mark-to-market-fb-rollback", Meaning: "Bitwave full fair-value adjustment mode.", Warnings: []string{"Public docs do not fully define F&B rollback journals", "Confirm product behavior before mapping it to accounting guidance"}, NativeInBitwave: true, RequiresEvidence: true},
	}
}

func inventoryPricingMethodologies() []inventoryStrategyGuidance {
	return []inventoryStrategyGuidance{
		{ID: "org-default", Meaning: "Inherits the organization's approved valuation pricing methodology.", CommonUses: []string{"Preferred default when approved"}, NativeInBitwave: true},
		{ID: "open-close-low-high-vwap", Meaning: "Chooses the market observation used at the configured frequency.", Warnings: []string{"A price input is not proof of compliance", "Apply consistently and document it"}, NativeInBitwave: true},
	}
}

func knownInventoryJurisdiction(value string) bool {
	switch value {
	case "US", "UK", "GB", "CA", "SG":
		return true
	default:
		return false
	}
}

func selectedInventoryGuidanceNotes(jurisdiction, framework string) []inventoryJurisdictionNote {
	all := inventoryGuidanceNotes()
	if jurisdiction == "" && framework == "" {
		return all
	}
	selected := []inventoryJurisdictionNote{}
	for _, note := range all {
		if note.ID == jurisdiction || (jurisdiction == "GB" && note.ID == "UK") || note.ID == framework {
			selected = append(selected, note)
		}
	}
	return selected
}

func inventoryGuidanceNotes() []inventoryJurisdictionNote {
	return []inventoryJurisdictionNote{
		{ID: "US-GAAP", Scope: "financial-statement books", Summary: []string{"ASU 2023-08 requires qualifying fungible crypto assets at fair value through net income.", "Out-of-scope assets require separate analysis.", "Acquisition transaction costs are generally expensed unless industry guidance applies."}, BitwaveStartingPoints: []string{"GAAP Fair Value", "Organization-default pricing"}, Sources: []inventoryGuidanceSource{{Title: "FASB ASU 2023-08", URL: "https://storage.fasb.org/ASU%202023-08.pdf"}}, LastReviewed: inventoryGuidanceReviewed},
		{ID: "IFRS", Scope: "financial-statement books", Summary: []string{"IAS 2 applies to ordinary-course inventory; commodity broker-traders may use fair value less costs to sell.", "Otherwise qualifying cryptocurrency is generally analyzed under IAS 38.", "IAS 38 revaluation requires an active market; IAS 2 permits FIFO or weighted average, not LIFO."}, BitwaveStartingPoints: []string{"IFRS Impairment for an approved IAS 38 cost model", "IFRS revaluation only after active-market analysis", "FIFO or Cost Average for applicable IAS 2 inventory"}, Gaps: []string{"Confirm Mark to Market journal behavior before using it for broker-trader reporting."}, Sources: []inventoryGuidanceSource{{Title: "IFRS Holdings of Cryptocurrencies", URL: "https://www.ifrs.org/projects/completed-projects/2019/holdings-of-cryptocurrencies/"}, {Title: "IAS 2 Inventories", URL: "https://www.ifrs.org/issued-standards/list-of-standards/ias-2-inventories/"}, {Title: "IAS 38 Intangible Assets", URL: "https://www.ifrs.org/content/dam/ifrs/publications/pdf-standards/english/2021/issued/part-a/ias-38-intangible-assets.pdf"}}, LastReviewed: inventoryGuidanceReviewed},
		{ID: "US", Scope: "federal tax starting point", Summary: []string{"For dispositions from 2025, basis identification generally operates by wallet or account.", "Adequately documented specific identification may be used; FIFO is the default when absent."}, BitwaveStartingPoints: []string{"Per-wallet inventory", "FIFO or supported Specific Identification", "Historical cost"}, Gaps: []string{"State, character, entity, dealer/trader, and form conclusions remain outside the preset."}, Sources: []inventoryGuidanceSource{{Title: "IRS Digital Asset FAQs", URL: "https://www.irs.gov/individuals/international-taxpayers/frequently-asked-questions-on-digital-asset-transactions"}}, LastReviewed: inventoryGuidanceReviewed},
		{ID: "UK", Scope: "individual capital-gains tax starting point", Summary: []string{"Disposals can require same-day matching, following-30-day matching, then a Section 104 pool."}, BitwaveStartingPoints: []string{"Cost Average may assist with the Section 104 pool"}, Gaps: []string{"Cost Average alone does not reproduce same-day and 30-day matching; use a jurisdiction-specific layer."}, Sources: []inventoryGuidanceSource{{Title: "HMRC Cryptoasset Pooling", URL: "https://www.gov.uk/hmrc-internal-manuals/cryptoassets-manual/crypto22200"}}, LastReviewed: inventoryGuidanceReviewed},
		{ID: "CA", Scope: "Canadian tax starting point", Summary: []string{"Capital holdings generally use weighted-average adjusted cost base.", "Business inventory treatment depends on facts and consistency."}, BitwaveStartingPoints: []string{"Cost Average for reviewed capital treatment"}, Gaps: []string{"Capital versus business income requires analysis."}, Sources: []inventoryGuidanceSource{{Title: "CRA Cryptoasset Capital Gains", URL: "https://www.canada.ca/en/revenue-agency/news/newsroom/tax-tips/tax-tips-2024/reporting-your-capital-gains-as-crypto-asset-user.html"}}, LastReviewed: inventoryGuidanceReviewed},
		{ID: "SG", Scope: "Singapore tax starting point", Summary: []string{"IRAS accepts FIFO or weighted average for taxable token trading and rejects LIFO.", "Unrealized accounting fair-value movements are not automatically taxable or deductible."}, BitwaveStartingPoints: []string{"FIFO or Cost Average after confirming taxable trading"}, Gaps: []string{"Capital versus revenue treatment depends on facts."}, Sources: []inventoryGuidanceSource{{Title: "IRAS Digital Token Tax", URL: "https://www.iras.gov.sg/media/docs/default-source/e-tax/etaxguide_cit_income-tax-treatment-of-digital-tokens_0910203299.pdf"}}, LastReviewed: inventoryGuidanceReviewed},
	}
}
