# Inventory Views and Jurisdiction Guidance

`bitwave inventory` lets an LLM list, create, and start calculations for
Bitwave inventory views. A jurisdiction profile is a reviewable starting
configuration and prompt set. It is not legal, tax, accounting, or financial
advice, and it must never be presented as a conclusion about a user.

## Required LLM flow

Before creating a view, distinguish among:

1. financial-statement books;
2. federal tax;
3. state and local tax;
4. management reporting; and
5. balance/reconciliation-only reporting.

Location alone does not answer that question. Ask for the entity type,
reporting framework, fiscal year, industry-specific guidance, filing
jurisdictions, asset scope, pricing policy, wallet/account mapping, lot method,
fee policy, and records supporting any specific-identification election. Ask a
qualified accountant to approve the setup. Guidance is advisory and never
blocks an explicit user override.

The schema-version 2 guidance response is designed for an LLM. It returns
unresolved setup questions, plain-language strategy definitions, reviewed
jurisdiction notes, source links, known engine gaps, and reviewed executable
profiles. An unknown jurisdiction is not an error: the CLI returns the general
decision framework and directs the LLM to current local primary authority.
Questions should be asked in a short conversational sequence, not dumped on
the user at once.

The LLM must record answers, assumptions, overrides, the professional
approver, policy effective date, source URLs, and source access dates. A
`lastReviewed` date is not a guarantee that guidance remains current;
`mustRecheckSources` is always true.

Run:

```bash
bitwave inventory guidance
bitwave inventory guidance --framework IFRS --jurisdiction UK --purpose tax --effective-date 2026-07-31
bitwave inventory guidance --framework US-GAAP --jurisdiction US --purpose books
bitwave inventory create --profile us-gaap --dry-run
bitwave inventory create --profile us-gaap --yes
bitwave inventory update "US GAAP - Fair Value" --yes
bitwave inventory delete "US GAAP - Fair Value" --dry-run
```

`inventory update` mirrors the web application's **Update Now** behavior. It
uses the enhanced update-request endpoint and, unless `--as-of` is supplied,
sets the current run end date to yesterday in the organization's timezone.
This prevents a same-day pricing window from extending into the future. The
resolved date and timezone are returned in JSON.

Before choosing that default, an LLM must determine the latest period through
which transaction sync, pricing, categorization, and reconciliation are
complete. When books are only ready through an earlier close date, pass that
date explicitly with `--as-of`; do not extend the run merely because newer
transactions exist. Uncategorized activity can produce incomplete or errored
actions and compromise downstream reports. If a wrong-cutoff run is still
`Running` or `New`, inspect and cancel that exact run before replacing it:

```bash
bitwave inventory updates "US GAAP - Fair Value"
bitwave inventory cancel "US GAAP - Fair Value" UPDATE_ID --dry-run
bitwave inventory cancel "US GAAP - Fair Value" UPDATE_ID --yes
bitwave inventory update "US GAAP - Fair Value" --as-of 2026-07-31 --yes
```

When the reference-inventory-view feature is enabled, an LLM may provide a
prior run and its end date together:

```bash
bitwave inventory update "US GAAP - Fair Value" \
  --as-of 2026-08-12 \
  --reference-run UPDATE_ID \
  --reference-end-date 2026-07-31 \
  --yes
```

After starting a run, inspect the same run history and errors shown by the UI:

```bash
bitwave inventory updates "US GAAP - Fair Value"
```

Never invent a reference run ID. `--reference-run` and
`--reference-end-date` must either both be present or both be absent. Explicit
end dates cannot be today or later in the organization timezone.

All writes require `--yes`; `--dry-run` returns the exact request without
changing the organization. Creation is idempotent by exact case-insensitive
view name.

## U.S. GAAP books profile

`us-gaap` is a starting view for a company that has confirmed it issues U.S.
GAAP financial statements:

- engine v2.9;
- FIFO operational lot selection, per wallet;
- GAAP fair-value valuation for in-scope crypto assets;
- acquisition transaction costs expensed by default under ASU 2023-08 unless
  industry-specific guidance applies;
- valuation pricing explicitly inherited from the organization's configured
  default rather than left unspecified;
- NFTs excluded because they do not meet ASU 2023-08's fungibility scope
  criterion; and
- original acquisition dates preserved for wallet-level internal transfers.

Fee treatment is a configurable accounting-policy decision and varies by fee
type and applicable guidance; the accountant must confirm it. FIFO is not
represented as a FASB requirement. The accountant must determine
which assets meet every ASU 2023-08 scope criterion and separately analyze
NFTs, issued or related-party assets, assets carrying enforceable rights,
wrapped or receipt tokens, staking, DeFi, derivatives, and industry-specific
guidance. U.S. GAAP books do not determine tax treatment.

## U.S. federal tax profile

`us-federal-tax-fifo` is a separate starting view:

- FIFO;
- inventory mapped per wallet/account;
- historical cost rather than book fair-value remeasurement;
- trading transaction costs reflected in the inventory calculation; and
- original acquisition dates preserved for internal transfers.

For dispositions on or after January 1, 2025, the IRS wallet/account rules
make a universal multi-wallet basis pool inappropriate. Specific
identification requires timely identification and adequate substantiating
records; otherwise the CLI surfaces FIFO as the federal default. Transaction
costs for acquisitions, dispositions, asset-for-asset exchanges, and transfers
between the user's own wallets do not all receive the same treatment. The
profile does not decide asset character, taxpayer classification, dealer or
trader status, section 1256 treatment, state conformity, or filing forms.

## Primary and product sources

Recheck these at execution time. The embedded guidance was last reviewed on
2026-08-13.

- [IRS Digital Assets](https://www.irs.gov/filing/digital-assets)
- [IRS Digital Asset Transaction FAQs](https://www.irs.gov/individuals/international-taxpayers/frequently-asked-questions-on-digital-asset-transactions)
- [IRS Revenue Procedure 2024-28](https://www.irs.gov/pub/irs-drop/rp-24-28.pdf)
- [FASB ASU 2023-08](https://storage.fasb.org/ASU%202023-08.pdf)
- [Bitwave Inventory Views](https://docs.bitwave.io/docs/inventory-views-1)

Reviewed decision notes are embedded for U.S. GAAP, IFRS, U.S. federal tax,
UK individual capital-gains pooling, Canadian tax starting points, and
Singapore digital-token tax. Only the two U.S. configurations are executable
profiles. Other notes remain prompts because entity facts and current local
rules can change the correct setup.

Important product gaps remain explicit. A plain Cost Average view does not
reproduce the UK's same-day and 30-day matching overlays, and Bitwave's public
documentation does not fully define the journal mechanics of "Mark to Market
and F&B Rollback." The LLM must not conceal those gaps or claim compliance
merely because Bitwave offers a similarly named strategy.

Additional primary sources:

- [IFRS Holdings of Cryptocurrencies](https://www.ifrs.org/projects/completed-projects/2019/holdings-of-cryptocurrencies/)
- [IAS 2 Inventories](https://www.ifrs.org/issued-standards/list-of-standards/ias-2-inventories/)
- [IAS 38 Intangible Assets](https://www.ifrs.org/content/dam/ifrs/publications/pdf-standards/english/2021/issued/part-a/ias-38-intangible-assets.pdf)
- [HMRC Cryptoasset Pooling](https://www.gov.uk/hmrc-internal-manuals/cryptoassets-manual/crypto22200)
- [CRA Cryptoasset Capital Gains](https://www.canada.ca/en/revenue-agency/news/newsroom/tax-tips/tax-tips-2024/reporting-your-capital-gains-as-crypto-asset-user.html)
- [CRA Cryptoasset Valuation](https://www.canada.ca/en/revenue-agency/programs/about-canada-revenue-agency-cra/compliance/cryptocurrency-guide/value-crypto.html)
- [IRAS Income Tax Treatment of Digital Tokens](https://www.iras.gov.sg/media/docs/default-source/e-tax/etaxguide_cit_income-tax-treatment-of-digital-tokens_0910203299.pdf)
