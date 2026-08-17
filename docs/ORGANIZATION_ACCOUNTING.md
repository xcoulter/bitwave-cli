# Organization accounting data

`bitwave org accounting` exposes Bitwave accounting connections, chart
accounts, and categorization contacts. It reports backend state and performs
explicit mutations; it does not select an accounting policy or design a chart
of accounts.

## Inspect current state

```bash
bitwave org accounting status --json
bitwave org accounting connections list --json
bitwave org accounting accounts list --accounting-connection CONNECTION_ID --json
bitwave org accounting contacts list --accounting-connection CONNECTION_ID --json
```

`status` returns active connection and chart-account counts. Use `--query` and
`--limit` on account and contact lists to bound machine-readable output.

Provider authorization and credentials remain in the Bitwave web application.
Provider-owned charts should be maintained in the provider and synchronized to
Bitwave.

## Manual Bitwave connection

```bash
bitwave org accounting manual create --dry-run --json
bitwave org accounting manual create --json
```

Bitwave automatically provisions the manual accounting setup with the stable
connection ID `Manual`. For compatibility, `manual create` now selects and
returns that connection; it does not create a second manual connection.

Create one manual account:

```bash
bitwave org accounting accounts create \
  --accounting-connection Manual \
  --id 4000 --code 4000 --name "Revenue" --type revenue \
  --yes --json
```

Import several manual accounts:

```json
{
  "accounts": [
    {
      "connectionId": "Manual",
      "id": "4000",
      "code": "4000",
      "name": "Revenue",
      "type": "revenue"
    }
  ]
}
```

```bash
bitwave org accounting accounts import --input accounts.json --dry-run --json
bitwave org accounting accounts import --input accounts.json --yes --json
```

Supported account types are `asset`, `bank`, `equity`, `expense`, `liability`,
`other`, and `revenue`.

Create one categorization contact:

```bash
bitwave org accounting contacts create \
  --accounting-connection CONNECTION_ID \
  --name "Counterparty" --type vendor \
  --yes --json
```

The caller is responsible for choosing the connection, accounts, contacts, and
accounting treatment.
