# Organization manual transaction imports

`bitwave import` operates the same organization-product workflow as Bitwave's
**Manual Transaction Imports** page at `/import`. It is separate from
`bitwave je import`, which writes journal entries into a local/plain-text CLI
ledger.

## End-to-end workflow

Inspect a file first:

```sh
bitwave import transactions ./transactions.csv --mode auto --json
```

`auto` is read-only and returns a structured plan with the recommended mode,
row count, reasons, validation issues, target wallets, expected requests, and
whether an import-history record will exist. Run the selected workflow
explicitly:

```sh
bitwave import transactions ./transactions.csv --mode direct --dry-run --json
bitwave import transactions ./transactions.csv --mode direct --yes --json
bitwave import transactions ./transactions.csv --mode staged --yes --json
```

Direct mode creates canonical transaction rows immediately and does not create
an import-history record. It is fastest for small, clean, single-wallet files
with stable unique IDs. Staged mode is slower but provides server-side row
validation, warning/error review, progress monitoring, and import history. It
is recommended for large, messy, ambiguous, mixed-wallet, or review-sensitive
files. Explicit `direct` or `staged` selection overrides the recommendation
when the chosen workflow can represent the file safely.

The confirmed staged command performs:

1. `createImport` for the active organization;
2. an unauthenticated CSV `PUT` to the returned signed upload URL;
3. `validateImport` plus status polling;
4. a hard stop when validation has any row errors;
5. `runImport` plus status polling when validation passes.

The signed upload receives no Bitwave bearer token. The command never prints
the signed URL.

Use `--validate-only` to stop at `ready-to-import`. The combined command stays
attached so one approval covers the entire safe workflow. For an asynchronous
split flow, use `upload` followed by `validate --no-wait`, then inspect/resume:

```sh
bitwave import status IMPORT_ID --watch --json
bitwave import run IMPORT_ID --yes --json
```

## Individual lifecycle commands

```sh
bitwave import template
bitwave import list --offset 0 --limit 25
bitwave import upload ./transactions.csv --yes --json
bitwave import preview IMPORT_ID --json
bitwave import validate IMPORT_ID --yes --json
bitwave import errors IMPORT_ID --stage validate --all --json
bitwave import errors IMPORT_ID --stage run --format csv --out run-errors.csv
bitwave import run IMPORT_ID --yes --json
bitwave import status IMPORT_ID --watch --json
```

`preview` supports organizations opted out of the Temporal engine. The current
Temporal v2 engine returns an empty preview because validation and staged row
errors are its pre-commit review.

## CSV and validation

Only one non-empty `.csv` file is accepted. Use `bitwave import template` for
the current official template. Timestamps should be ISO 8601 values with an
explicit timezone, for example `2026-07-01T09:00:00Z`.

With `--json`, `bitwave import template` also returns `supportedColumns`. Preserve
all source values that have a corresponding supported column, particularly the
transaction ID/hash, blockchain ID, wallet/account ID, full from/to addresses,
amount and ticker, fee and fee ticker, timestamp, transaction type, memo,
description, contact/category IDs, trade/group IDs, cost fields, and tax-exempt
indicator. The CLI uploads these fields without discarding optional columns;
Bitwave validation determines whether each value is accepted.

Auto/direct analysis is operational only: it checks whether canonical fields
can be transported safely. It does not infer accounting treatment. Direct mode
preserves backend authorization and closed-period enforcement, batches requests,
and uses the CSV `id` as the caller-controlled idempotency key. Duplicate or
missing IDs block direct execution.

Validation errors never trigger `runImport`. Export the complete fix list,
correct the source file, and create a new import:

```sh
bitwave import errors IMPORT_ID --stage validate --format csv --out import-errors.csv
```

The Bitwave page has no retry/replace operation; a corrected upload creates a
new import record.

## Failure and retry safety

The run phase is not atomic. A failed or cancelled import may have created some
transactions. When available, the CLI returns `rowsImported` and `rowsTotal`.
Review those counts, run-stage errors, and the resulting transactions before
submitting a corrected file. Do not blindly rerun the same import.

## Scope boundary

This command family covers `/import` Manual Transaction Imports. Bitwave's
`/importv2` and `/importv3` advanced Data Import workflows are separate product
surfaces and are not represented as supported by these commands.
