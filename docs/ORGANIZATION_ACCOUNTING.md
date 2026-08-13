# Accounting setup before categorization

An organization needs an accounting connection before an LLM can create
meaningful categorization rules. Bitwave's implicit `Manual` connection is
always addressed by the stable ID `Manual` and supplies `manual.assets`
(Digital Assets) and `manual.fee` (Crypto Fees). The connection-list endpoint
may omit it, so the CLI materializes it explicitly. A generated manual or
external connection does not inherit those defaults. Additional category IDs
and contacts are scoped to the accounting
connection. Prompt for client-specific accounts and contacts either immediately
before wallet onboarding or immediately after wallet creation; do not wait
until the user asks to create their first rule.

An accounting connection ID is a namespace and must not be described as an
account. For example, `RolAwp0ATwlSLTw1jKeZ` identifies one connection, while
an ID such as `RolAwp0ATwlSLTw1jKeZ.1000` identifies a category within that
connection. Agents must resolve and display the category name instead of
guessing from the connection ID.

## One fast readiness check

```bash
bitwave org accounting status --json
```

The response is deliberately compact:

- `accounting_connection_missing`: the normally provisioned manual connection
  is absent; verify provisioning or connect the client's external system. Do
  not create a second manual connection from the CLI.
- `client_categories_and_contacts_needed`: the connection exists but categories
  and contacts are absent; ask for the Digital Assets mapping and the client's
  additional accounts and contacts, or offer a transaction-based minimal proposal.
- `client_categories_needed` / `contacts_required`: collect only the missing
  client-specific resource.
- `ready_for_categorization_and_rules`: continue without another prompt.

`rule context` includes the same `accountingReadiness` object. An LLM should not
ask again after the organization is ready.

## External accounting system

Provider authorization and credentials remain in the Bitwave web application.
The LLM should direct the user to Accounting Connections, let them select and
authorize their provider, and then rerun:

```bash
bitwave org accounting status --json
```

The provider's chart should be created and maintained in that accounting
system, then synced into Bitwave. The CLI will not write manual accounts into an
external connection.

### Rillet invoice and bill visibility

When an active Rillet connection is present, `bitwave org accounting status
--json` includes compact `providerSyncGuidance`. An empty invoice selector does
not by itself prove that Rillet failed to return or Bitwave failed to import the
invoice. The LLM should distinguish provider retrieval, Bitwave materialization,
API serialization, and UI eligibility before stating the cause.

Rillet monetary amounts use ISO currency codes such as `USD`. Bitwave's stored
invoice contract uses canonical asset IDs such as `FIAT.1`; API and UI DTOs then
render that value as `USD`. If the Bitwave invoice endpoint reports `Bad assetId,
received USD`, the record can already exist: the likely defect is that the
provider adapter persisted the ISO code directly or the read path failed to
normalize a legacy record. Do not tell the user to recreate the invoice until
this distinction is checked.

Rillet contact identity is deterministic:

```text
<accountingConnectionId>.<raw Rillet customer_id or vendor_id>
```

Preserve both parts exactly. For a selected contact to expose an invoice or
bill, the imported record must use that contact ID and the same accounting
connection. Payment categorization normally further restricts results to an
`AwaitingPayment` status and a non-zero due amount. Therefore the LLM should
check, in order:

1. the invoice or bill exists in Rillet for the raw customer or vendor ID
   (`GET /invoices?customer_id=<raw id>` or `GET
   /bills?vendor_id=<raw id>`), using the required API-version header and
   following cursor pagination;
2. the prefixed contact exists in Bitwave;
3. the Bitwave invoice endpoint response for that exact contact, including any
   error body;
4. invoice sync skip settings, the remote-invoice writing kill switch, and
   materialization/upsert errors; and
5. the imported record's currency, `contactId`, status, `dueAmount`, and
   `accountingConnectionId`.

Typical Rillet status mapping is `UNPAID` and `PARTIALLY_PAID` to
`AwaitingPayment`, `PAID` and `APPLIED` to `Paid`, and `UNBILLED` to `Draft`.
Other statuses may intentionally be ineligible for payment categorization.
This guidance is provider troubleshooting context, not evidence that a specific
organization has a sync defect.

## Automatically provisioned manual setup

Discover and reuse the existing manual connection:

```bash
bitwave org accounting manual use --json
```

Normal organization setup already contains this implicit connection. The
command never creates another one. It uses the stable `Manual` namespace and
its built-in Digital Assets and Crypto Fees accounts. Never substitute a
generated manual connection ID for this setup.

Inspect the conservative starter set without writing, or reapply it later:

```bash
bitwave org accounting starter show --accounting-connection CONNECTION_ID --json
bitwave org accounting starter apply --accounting-connection CONNECTION_ID --yes --json
```

The apply command is idempotent by enabled resource name. It creates only
General Revenue, General Expense, and Gas Fees categories, plus General
Customer, General Vendor, and Gas Fees contacts. These are broad fallbacks that
make the common simple-inflow, simple-outflow, gas-only, and trade-fee workflows
possible; they are not a substitute for the client's accounting policy.

Create one additional client-specific account:

```bash
bitwave org accounting accounts create \
  --accounting-connection CONNECTION_ID \
  --id 4000 \
  --code 4000 \
  --name "Revenue" \
  --type revenue \
  --yes --json
```

Or import a chart in one command:

```json
{
  "accounts": [
    {
      "connectionId": "CONNECTION_ID",
      "id": "4000",
      "code": "4000",
      "name": "Revenue",
      "type": "revenue"
    },
    {
      "connectionId": "CONNECTION_ID",
      "id": "6100",
      "code": "6100",
      "name": "Crypto Fees",
      "type": "expense"
    }
  ]
}
```

```bash
bitwave org accounting accounts import --input accounts.json --dry-run --json
bitwave org accounting accounts import --input accounts.json --yes --json
```

Imports reuse one authenticated client, default to two concurrent writes, and
support `--concurrency 1` through `8`. The importer retries HTTP 429 responses
with backoff and skips account IDs that already exist, so rerunning the same
file safely resumes a partial import. Supported account types are `asset`,
`bank`, `equity`, `expense`, `liability`, `other`, and `revenue`.

List bounded account choices without filling LLM context:

```bash
bitwave org accounting accounts list \
  --accounting-connection CONNECTION_ID \
  --query revenue --limit 20 --json
```

After setup, rerun status. When `readyForRules` is true, continue to transaction
analysis and rule planning.

Do not auto-seed staking, DeFi, wrapped-token, bridge, exchange, network, or
token-specific asset accounts. Those can conflict with the client's selected
Digital Assets mapping unless the client's accounting policy explicitly calls for
them. Prefer the client's supplied chart. If none is supplied, transaction
analysis may suggest a small set of evidence-backed revenue, expense,
liability, or equity categories and counterparties; the LLM should present the
proposal before creating them.

The CLI's returned `starter.guardrails` is advisory context for an LLM, and
`starter.advisoryOnly` is always true. Warnings explain likely consequences but
never prevent a requested mutation. The CLI stops only for malformed or missing
technical inputs, missing mutation confirmation, or an API rejection.

An LLM may analyze transactions and recommend additional resources, but it must
describe those as proposals and obtain the user's approval before creating
them. If the user chooses a treatment that differs from the recommendation, the
LLM should state the likely Bitwave consequence and proceed.

## Important fee-policy distinction

Create a reusable fee contact for trade categorization. Trade fees require that
contact but normally do not use a fee category: the fee remains in the trade so
it can be capitalized. A standalone gas-only transaction is different and uses
both a gas-fee category and fee contact. The LLM must select the treatment from
the transaction type instead of assuming every fee takes the same category.
