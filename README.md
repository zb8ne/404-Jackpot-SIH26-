# Government Credential Registry

Departments issue documents to citizens. Anyone can check that a document is authentic
and current. A smart contract holds the hash and metadata; the PDFs themselves stay
off-chain. Tampering shows up as a hash mismatch.

Supabase authenticates people, SQLite owns human roles and department assignments,
and Solidity owns department-wallet credential authority and lifecycle state. Phase 2
adds hash-chained audit history, department-scoped Admin audit access, and system-wide
Controller monitoring. The full API contract is in [`docs/API.md`](docs/API.md).

Phase 3 adds citizen accounts and consent-gated QR verification. Newly issued PDFs
open `/verify?docId=...`; an authenticated government user creates a request, and
the citizen approves or denies through a random one-time email link before the
backend reveals the current chain-backed verdict.

## Run it

```sh
doppler run -- docker compose up
```

That starts Anvil, deploys the contract, provisions every configured government
profile plus the demo citizens, and runs the API and web app. When fresh tokens for
all three department Admins are present it also seeds the demo documents. Nothing
but Docker is needed on the host — no Go, no Node, no Foundry.

```
frontend  http://127.0.0.1:5173
backend   http://127.0.0.1:8088
anvil     http://127.0.0.1:8545
```

```sh
docker compose down -v      # stop, and throw away the chain and database
docker compose logs -f backend
```

If a port is already taken on your machine, override it — the app does not care:

```sh
WEB_PORT=5174 doppler run -- docker compose up
```

### Without Doppler

Every value comes from the environment, so anything that exports them works. Copy
`.env.example` to `.env` and fill it in, and plain `docker compose up` picks it up.
Nothing is baked into an image. The Supabase URL and frontend publishable key are
required; government profile IDs and demo-document tokens are optional.

### On the host instead

```sh
make demo     # same stack, run directly on the host
make stop     # kill anvil, backend and frontend
make test     # contract tests + the verify state machine
```

Needs Go 1.22+, Node 20+, and [Foundry](https://getfoundry.sh)
(`curl -L https://foundry.paradigm.xyz | bash && foundryup`).

## Configuration

The backend needs `SUPABASE_URL`; the frontend needs `VITE_SUPABASE_URL` and
`VITE_SUPABASE_PUBLISHABLE_KEY`. `VITE_API_URL` overrides the backend address, and
`PUBLIC_WEB_URL` (default `http://127.0.0.1:5173`) is the base URL embedded in new QR
codes. There is no authentication bypass.

Credential operations need a Supabase user with a backend profile. The Compose
`profiles` step provisions each non-empty department Admin ID independently from
`SEED_BIRTH_USER_ID`, `SEED_TRANSPORT_USER_ID`, and `SEED_EDUCATION_USER_ID`. It can
also provision `SEED_BIRTH_OFFICIAL_USER_ID` and `SEED_CONTROLLER_USER_ID`. This lets
a partial local environment run without inventing identities or assigning one user to
multiple departments.

The document seeder issues through the real REST API, so it also needs department Admin
**access tokens** in `SEED_BIRTH_TOKEN`, `SEED_TRANSPORT_TOKEN` and
`SEED_EDUCATION_TOKEN`. It skips document seeding unless all three are present, while
the rest of the stack continues to start. These are short-lived JWTs — they expire in
about an hour, so a seeded run needs freshly minted tokens.

To provision a profile by hand, using an existing Supabase user's `sub`:

```sh
cd backend
go run ./cmd/profile-seed -db credentials.db \
  -id '<supabase-user-id>' -email official@example.gov \
  -name 'Birth Official' -role OFFICIAL -department birth
```

`CONTROLLER` profiles omit `-department`; `ADMIN` and `OFFICIAL` must use `birth`,
`transport` or `education`.

Citizen accounts require an email; phone is optional and is not used for consent:

```sh
cd backend
go run ./cmd/citizen-seed -db credentials.db -id citizen-asha \
  -name 'Asha Menon' -email asha@example.test
```

Use `-link-doc-id` only when explicitly linking an older stored credential. Ownership is
never inferred from a name or email address.

## The demo

With all three Admin tokens configured, either run path seeds the same documents into
`./demo-files/`, one per verdict. A partial Compose run skips this fixture:

| File | Verdict | |
| --- | --- | --- |
| `asha-menon-birth-certificate-v2.pdf` | `VALID` | hashes match |
| `asha-menon-birth-certificate-v1.pdf` | `SUPERSEDED` | genuine, but corrected by v2 |
| `rahul-iyer-driving-licence.pdf` | `REVOKED` | genuine bytes, revoked by Transport Dept |
| `rahul-iyer-degree-TAMPERED.pdf` | `TAMPERED` | class upgraded, docId marker intact |
| `never-issued-driving-licence.pdf` | `NOT_ISSUED` | well-formed, but no such docId |

```sh
curl -s -H "Authorization: Bearer $SEED_BIRTH_TOKEN" \
  -F file=@demo-files/asha-menon-birth-certificate-v1.pdf localhost:8088/verify | jq
curl -s -H "Authorization: Bearer $SEED_EDUCATION_TOKEN" \
  -F file=@demo-files/rahul-iyer-degree-TAMPERED.pdf localhost:8088/verify | jq
curl -s -H "Authorization: Bearer $SEED_BIRTH_TOKEN" \
  localhost:8088/verify/BC-2019-004471 | jq
curl -s -H "Authorization: Bearer $SEED_BIRTH_TOKEN" \
  'localhost:8088/credentials/Asha%20Menon' | jq
```

### Corrections keep their history

Asha's birth certificate was issued with her name misspelled. The Birth Dept does not
edit it — it issues a corrected v2 and supersedes v1. Both stay on the record:

```
BC-2019-004471     birth_certificate -> SUPERSEDED
BC-2019-004471-R1  birth_certificate -> VALID
```

Verifying the untouched v1 PDF returns `SUPERSEDED` along with a `supersededBy` pointer to
the version that replaced it. QR verification of linked credentials is consent-gated and
returns the current lifecycle verdict only after the citizen approves the request.

### Issuing stamps the document

`POST /issue` does not anchor the file it was handed. It stamps the upload with a registry
page — a QR code encoding the configured `/verify?docId=...` URL, and the marker
`CREDREG-DOCID:<docId>` — and only then
hashes it:

```
stamp -> sha256(stamped bytes) -> anchor that hash -> store the stamped PDF
```

The citizen walks away with the stamped copy, so the stamped bytes are the ones the chain
knows. Stamping is an incremental update: the original bytes are left byte-for-byte intact
and the registry page is appended, so a department's document is never rewritten. The
response carries a download link for the stamped file.

Verification reads the marker; the QR is for scanning on stage and is never decoded
server-side. A PDF whose cross-reference table the stamper cannot follow falls back to
appending the marker as a trailing comment — still verifiable, just without a page to look
at, and the response says so.

Carrying an id as well as a hash is what lets the registry tell an altered document from
one it never issued:

```
hash on record          -> VALID / SUPERSEDED / REVOKED   these exact bytes were issued
else docId on record    -> TAMPERED                       issued, but these bytes changed
else                    -> NOT_ISSUED                     never issued at all
```

The hash is checked first and the id is only a fallback. The other way round is a trap: a
superseded document's id points at its replacement, so an untouched original would be
reported as tampered. A hash the registry knows is genuine whatever its standing today.

`/verify` always returns both `expectedHash` and `computedHash`, even when they match, so
a verifier can show them side by side:

```json
{
  "status": "TAMPERED",
  "docId": "DEG-2019-0455",
  "expectedHash": "0x24c4ab47...",
  "computedHash": "0x5cb735bd...",
  "message": "document DEG-2019-0455 exists, but this file does not match the hash on record ..."
}
```

The centrepiece is the role gate. Transport Dept holds the driving-licence role, so the
contract rejects it issuing a birth certificate:

```sh
curl -s -H "Authorization: Bearer $SEED_TRANSPORT_TOKEN" \
     -F file=@demo-files/asha-menon-degree.pdf -F dept=transport \
     -F doc_type=birth_certificate -F doc_id=BC-FAKE-1 -F citizen=Mallory \
     localhost:8088/issue
# 403 — contract rejected the call (department not authorised for this document type)
```

Same rule, proved at the contract level, in `test_TransportDeptCannotIssueBirthCertificate`.

## Layout

```
contracts/   Solidity + Foundry tests + deploy script
backend/     Go REST API (ethclient + abigen bindings, SQLite for PDFs)
frontend/    React + TypeScript + Tailwind (Vite)
docs/        API contract
docker-compose.yml   the whole stack; Dockerfiles live in backend/ and frontend/
```

The backend image is built in two stages: a distroless `runtime` for the server, and an
alpine `tools` stage carrying the seed binaries, which need a shell. Named volumes are
handed to the unprivileged runtime user by an `init-volumes` step before anything writes
to them.

### Frontend

The application uses tab navigation plus minimal pathname/query handling. New QR codes
encode a verification URL; existing bare-document-ID QR payloads remain compatible.

- **Verify** — drag-drop or pick a PDF. One large colour-coded verdict panel. A tampered
  document shows the expected and computed hashes stacked, with every differing character
  highlighted. A superseded one links straight to verifying the version that replaced it.
- **Issue** — department picker fed by `/departments` (the doc type follows the department,
  because the contract will reject any other combination), citizen, doc id, file upload. On
  success it shows the anchored hash and a download link for the stamped PDF.
- **Citizen** — pick a citizen, see their credentials with status badges. Asha's superseded
  v1 and current v2 both appear.
- **Monitoring** and **Audit Events** — minimal temporary Phase 2 validation screens for
  Controller system-wide monitoring and Controller/Admin audit access.
- **Revoke** and **Supersede** — minimal temporary Admin lifecycle validation forms.
- **QR verification request** — `/verify?docId=...` requires government login,
  captures a purpose, sends a development consent notification, and reveals the
  blockchain verdict only after approval.
- **Citizen consent** — `/consent/{token}` is a public one-time approve/deny page;
  citizens do not need a Supabase account for the prototype.

The Phase 2 screens are intentionally replaceable validation UI, not the frontend team's
final dashboard. Backend RBAC remains authoritative: Controllers receive Monitoring and
system-wide Audit only; department Admins receive Issue, Verify, Revoke, Supersede, and
department Audit; Officials receive Issue and Verify only.

### Contract

`docHash -> Credential { docId, docType, issuer, timestamp, status, prevHash }`, with
`status` one of `VALID | SUPERSEDED | REVOKED`, plus `currentHashOf: docId -> bytes32`,
the index that makes a document findable by id. `supersede` repoints it at the
replacement (so a QR on an old copy still resolves); `revoke` deliberately leaves it
alone, so a revoked document stays findable rather than looking as if it never existed.

`issue`, `supersede` and `revoke` are all gated on `issuerRole[msg.sender]` matching the
document type. Anvil's prefunded accounts 0, 1 and 2 are the Birth, Transport and
Education departments.

### API

The complete frontend/backend contract, including authentication, RBAC, exact
request and response shapes, errors, audit history, and Controller monitoring,
is maintained in [`docs/API.md`](docs/API.md).

| Method | Path | |
| --- | --- | --- |
| POST | `/issue` | multipart: `file`, `dept`, `doc_type`, `doc_id`, `citizen_account_id`; stamps, then anchors |
| POST | `/verify` | multipart: `file` → `VALID` / `SUPERSEDED` / `REVOKED` / `TAMPERED` / `NOT_ISSUED` |
| GET | `/verify/{docId}` | legacy ID lookup; linked credentials require the consent-request flow |
| POST | `/revoke` | json: `docHash`, `dept` |
| POST | `/supersede` | multipart replacement; inherits citizen linkage or accepts `citizen_account_id` for legacy records |
| GET | `/credentials/{citizen}` | documents with live on-chain status |
| GET | `/departments`, `/citizens`, `/health` | |
| GET | `/documents/{hash}/download` | the stored PDF |
| GET | `/me` | authenticated SQLite application profile |
| GET | `/auth-test` | temporary authentication diagnostic |
| GET | `/audit-events` | Controller system-wide or Admin department-scoped audit history |
| GET | `/monitoring/overview` | Controller-only system monitoring summary |
| GET | `/citizen-accounts` | masked citizen-account choices for issuance |
| GET, POST | `/verification-requests...` | scoped request status, creation, and completion |
| GET, POST | `/consent/{token}...` | minimal one-time citizen consent flow |
| GET | `/development/notifications/{id}` | authenticated in-memory demo email capture |

## Demo-grade shortcuts

- Hashes are taken over the raw PDF bytes, so re-saving a file breaks verification. Fine
  here — it is what makes tampering detectable at all.
- The PDF stamper handles classic cross-reference tables, not compressed xref streams;
  anything it cannot parse gets the comment fallback.
- Supabase authentication and backend RBAC protect credential operations, but department
  transaction keys are still Anvil defaults baked into the backend. Never do this anywhere real.
- Audit rows are hash-chained and integrity-checkable, but SQLite has no external checkpoint;
  a database administrator could replace or recompute the complete database history.
- Monitoring and audit data cover retained Phase 2 audit events, not older operations.
- The prototype email notifier keeps raw consent URLs only in backend process memory;
  links disappear on restart and no real email is delivered.
- Consent tokens expire after 15 minutes. SQLite stores only their SHA-256 hashes.
- The document seeder needs live department access tokens, which expire in about an hour,
  so a freshly seeded run needs freshly minted ones.
- The chain is a local Anvil node, no testnet — but token verification calls Supabase, so
  the stack is no longer runnable offline.

## Screenshots

_TODO._
