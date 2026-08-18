# 404-Jackpot-SIH26 Repository Guide

This file preserves repository context for future Codex sessions. Treat the checked-out code as the final source of truth and update this file when a major phase changes the architecture or project status.

## 1. PROJECT OVERVIEW

404-Jackpot-SIH26 is a government credential registry prototype. Government departments issue PDF credentials to citizens, the backend stamps each issued PDF with a registry marker and QR code, and the SHA-256 hash of the stamped bytes is anchored in an Ethereum smart contract. The PDF itself and citizen-facing metadata remain off-chain in SQLite.

Verification distinguishes an authentic current document from an authentic superseded or revoked document, a modified document, and a document that was never issued. Superseding adds a replacement without deleting the original, preserving credential history.

The repository is currently a local demo built around Anvil. Supabase authentication, the Phase 1 backend RBAC foundation, and Phase 2 audit/monitoring with temporary validation UI are implemented; citizen consent is future work.

## 2. CURRENT ARCHITECTURE

### Frontend

- React 19 + TypeScript, built with Vite and styled with Tailwind CSS.
- Entry point: `frontend/src/main.tsx`; application shell: `frontend/src/App.tsx`.
- There is currently no router. The application switches between tabs in `App.tsx`.
- Supabase client setup is in `frontend/src/lib/supabase.ts`.
- Backend request functions and shared response types, including `/health`, are centralized in `frontend/src/api.ts`.

### Backend

- Go REST service using `net/http` and Go 1.22-style method/path patterns on `http.ServeMux`.
- Entrypoint: `backend/cmd/server/main.go`.
- Route handlers and verification state machine: `backend/internal/api/api.go`.
- Supabase JWT validation and bearer middleware: `backend/internal/auth/`.
- Ethereum client and department signers: `backend/internal/chain/chain.go`.
- SQLite persistence: `backend/internal/store/store.go`.
- PDF stamping: `backend/internal/pdfdoc/pdfdoc.go`.
- Embedded document ID extraction: `backend/internal/docid/docid.go`.
- Generated Go contract binding: `backend/internal/contracts/credentialregistry.go`.

### SQLite/off-chain storage

SQLite stores stamped PDF bytes plus document, citizen, filename, issuer wallet, transaction hash, and issue-time metadata. It also stores stable application departments, backend-owned Supabase user profiles with roles, department assignments and active state, and hash-chained audit events. It does not store consent requests or lifecycle status. Live lifecycle status is read from Ethereum.

### Solidity/Ethereum registry

`contracts/src/CredentialRegistry.sol` stores credential hashes and lifecycle state. Foundry tests are in `contracts/test/CredentialRegistry.t.sol`; deployment is in `contracts/script/Deploy.s.sol`. The demo uses local Anvil accounts and keys currently embedded in the Go chain package.

### Current interaction

1. The frontend signs a user into Supabase and receives a session.
2. The centralized frontend API layer attaches the session access token to protected requests.
3. The backend verifies the token, loads the backend-owned profile, and enforces role/department permissions.
4. For authorized credential writes, the backend derives the department from the profile, selects the matching hardcoded signer, and submits a transaction to the registry contract.
5. The backend stores stamped PDFs and off-chain metadata in SQLite.
6. Verification reads the authoritative hash/status from the contract and supplements it with SQLite metadata when available.
7. Important credential operations and profile authorization changes append hash-chained SQLite audit events with the human actor, department, outcome, and relevant credential/transaction references.

## 3. EXISTING FUNCTIONALITY

### Document issuing

`POST /issue` in `backend/internal/api/api.go` accepts multipart fields `file`, `dept`, `doc_type`, `doc_id`, and `citizen`. It rejects already-marked uploads, stamps the PDF, hashes the stamped bytes, submits `issue` to the contract through `backend/internal/chain`, and saves the stamped PDF through `backend/internal/store`.

The order is intentionally:

`stamp -> SHA-256(stamped bytes) -> anchor hash -> store stamped PDF`

Do not reverse or bypass this ordering. The citizen's stamped copy must be the exact byte sequence anchored on-chain.

### Document IDs, PDF stamping, and QR

- `backend/internal/docid/docid.go` defines and extracts `CREDREG-DOCID:<docId>`.
- `backend/internal/pdfdoc/pdfdoc.go` appends a registry page using a PDF incremental update, leaving original bytes untouched.
- The page includes a vector QR and visible marker.
- If the simple PDF parser cannot handle a file, stamping falls back to a trailing PDF comment containing the marker.
- The current QR encodes only the bare document ID, not a frontend URL.
- PDF behavior is covered by `backend/internal/pdfdoc/pdfdoc_test.go`.

### Verification

- `POST /verify` hashes an uploaded file and extracts its embedded ID.
- `GET /verify/{docId}` resolves by ID without uploaded bytes; this is the current nominal QR lookup endpoint.
- `backend/internal/api/verify_test.go` covers `VALID`, `SUPERSEDED`, `REVOKED`, `TAMPERED`, and `NOT_ISSUED`.

The verification order is an important invariant:

1. Check the computed hash first. A known hash is a genuinely issued byte sequence, regardless of current status.
2. Only if the hash is unknown, fall back to the embedded document ID.
3. A known ID with unknown bytes is `TAMPERED`; an unknown ID is `NOT_ISSUED`.

Looking up the ID first would misclassify a genuine superseded original because the old ID points to its replacement.

### Superseding and revoking

- `POST /supersede` stamps and anchors a replacement, marks the old contract record `SUPERSEDED`, preserves the old record, and saves the replacement PDF.
- `POST /revoke` changes the contract record to `REVOKED`.
- A revoked document remains addressable through `currentHashOf`, so it verifies as revoked rather than not issued.
- The contract restricts these operations to the wallet that issued the credential and the appropriate document type.

### Lookup and download

- `GET /credentials/{citizen}` lists SQLite documents for a citizen and augments each with current on-chain status.
- `GET /citizens` lists known citizen names from SQLite.
- `GET /documents/{hash}/download` returns the stored PDF.
- `GET /departments` exposes the current compiled-in departments and document types.
- `GET /health` reports API health and the configured contract address.

### Existing frontend screens

- `frontend/src/screens/Login.tsx`: Supabase email/password sign-in.
- `frontend/src/screens/Verify.tsx`: PDF upload/drop verification and result display, with a helper path for verifying an ID.
- `frontend/src/screens/Issue.tsx`: department selection, citizen/document fields, PDF upload, client-side ID generation, and stamped-document result/download.
- `frontend/src/screens/Citizen.tsx`: citizen selection and credential history.
- `frontend/src/screens/Monitoring.tsx` and `frontend/src/screens/AuditEvents.tsx`: minimal temporary Phase 2 browser-validation UI, intentionally replaceable by the frontend team's final dashboard.
- `frontend/src/screens/Lifecycle.tsx`: minimal temporary Admin-only revoke/supersede validation forms using the existing Phase 2 API contracts.
- `frontend/src/verdicts.ts`: centralized verdict display configuration.

### Current backend endpoints

- `GET /health`
- `GET /departments`
- `GET /citizens`
- `POST /issue`
- `POST /verify`
- `GET /verify/{docId}`
- `POST /revoke`
- `POST /supersede`
- `GET /credentials/{citizen}`
- `GET /documents/{hash}/download`
- `GET /me` (authenticated application profile)
- `GET /auth-test` (temporary authentication-only diagnostic)
- `GET /audit-events` (Controller system-wide; Admin department-scoped)
- `GET /monitoring/overview` (Controller only)

## 4. AUTHENTICATION

Supabase Auth answers: **Who is this user?** Authorization will answer: **What can this user do?** These are separate concerns.

Current implementation:

- `frontend/src/lib/supabase.ts` creates the Supabase client using `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY`.
- `frontend/src/screens/Login.tsx` calls `supabase.auth.signInWithPassword`.
- `frontend/src/App.tsx` restores the session with `getSession`, subscribes to `onAuthStateChange`, shows the authenticated email, and supports sign-out.
- `backend/internal/auth/auth.go` validates Supabase ES256 JWTs against `${SUPABASE_URL}/auth/v1/.well-known/jwks.json`.
- Validation requires ES256, a matching P-256 JWKS key, expiration, audience `authenticated`, and issuer `${SUPABASE_URL}/auth/v1`.
- The backend extracts `sub` and `email` into `auth.User`.
- `backend/internal/auth/middleware.go` parses `Authorization: Bearer <token>`, verifies the JWT, and stores the authenticated user in request context.
- `auth.UserFromContext` retrieves that identity.
- `backend/cmd/server/main.go` requires `SUPABASE_URL` and constructs the verifier.
- `/auth-test` demonstrates middleware and authenticated-user extraction.

A real Supabase ES256 access token has already been successfully tested end-to-end against the Go backend. There is currently no automated Supabase integration test in the repository, so do not describe that manual result as repository test coverage.

Authentication is wired into protected application routes. The backend loads authorization data from SQLite by verified `sub`; role and department are not trusted from the frontend or JWT metadata.

## 5. CURRENT RBAC MODEL

### CONTROLLER

- System-wide monitoring and audit visibility.
- Cannot issue documents.
- Cannot verify documents.
- Cannot supersede documents.
- Cannot revoke documents.
- Has no department assignment and does not receive department credential authority.

### ADMIN

- Belongs to exactly one department.
- Can issue and verify for the authorized department.
- Can supersede and revoke for the authorized department.
- Can view audit activity only for that department.
- Cannot access system-wide monitoring.

### OFFICIAL

- Belongs to exactly one department.
- Can issue and verify for the authorized department.
- Cannot supersede or revoke.
- Cannot access audit events or system-wide monitoring.

The Go backend enforces every permission. Frontend role-based navigation is useful UX but is not a security boundary.

## 6. DEPARTMENT MODEL

Departments currently conceptually represent Birth, Transport, and Degree/Education. In `backend/internal/chain/chain.go`, they are a compiled-in slice containing a slug, display name, numeric document type, Ethereum address, and private key:

- `birth` -> `birth_certificate` -> contract type `1`
- `transport` -> `driving_licence` -> contract type `2`
- `education` -> `degree_certificate` -> contract type `3`

The same numeric types are constants in `CredentialRegistry.sol`. Deployment grants each demo Anvil wallet its corresponding type. The frontend obtains this list from `/departments`. For protected department operations, the backend derives authority from the authenticated SQLite profile. A submitted compatibility `dept` value must match that profile, and document type and target credential ownership are checked before a chain write.

Authorization uses stable backend department identifiers and configurable display names. Never use a mutable display name as a foreign key or permission rule. Keep contract document-type identifiers and human-facing department names as separate concepts.

## 7. AUTHORIZATION DESIGN

The implemented authorization flow is:

`Supabase JWT -> authenticated Supabase user ID -> backend-owned profile -> role + department -> permission check -> business operation`

Profile/role/department records are backend-owned and keyed by the verified Supabase `sub`. Do not trust role or department values supplied by forms, query parameters, local storage, UI state, or user-editable JWT metadata.

For department-scoped operations, derive the department from the authenticated backend profile. Validate document type and target credential against that department before invoking blockchain or storage operations.

## 8. AUDIT SYSTEM

Phase 2 audit logging is implemented for:

- the authenticated official/admin who issued a document;
- the authenticated official/admin who verified a document;
- the authenticated admin who superseded a document;
- the authenticated admin who revoked a document;
- timestamp;
- document ID and hash;
- department;
- action and result;
- relevant transaction/request identifiers.

The Controller can use this history through `/audit-events` and `/monitoring/overview`; Admin access is department-scoped and Officials have no audit API access.

Audit rows snapshot the actor profile and department and include previous/current entry hashes. Integrity validation recomputes the chain and detects modification, reordering, or a gap within retained rows. SQLite-only protections cannot prevent a database administrator from replacing the entire database, recomputing the chain, or deleting an uncheckpointed tail; stronger guarantees would require anchoring checkpoints externally or on-chain.

Audit outcomes have these meanings:

- `SUCCESS`: the authorized operation completed. A successful verification request remains `SUCCESS` even when its business verdict is `TAMPERED` or `NOT_ISSUED`.
- `DENIED`: authentication succeeded but RBAC or department authorization rejected the operation.
- `FAILURE`: validation, lookup, blockchain/RPC, or other operational work failed without completing the intended state change.
- `PARTIAL_FAILURE`: the blockchain write succeeded but the subsequent SQLite document save failed, so reconciliation is required.

Profile authorization changes use the audited store method, which commits the profile mutation and audit event in one SQLite transaction. The underlying generic upsert is private to the store package. Credential write failures after backend authorization are audited as `FAILURE`; blockchain-success/SQLite-document-save failures are `PARTIAL_FAILURE`.

## 9. QR + CITIZEN CONSENT FLOW

This is planned functionality only. The intended flow is:

`QR scan -> verification page -> official authenticates -> official requests verification -> citizen receives email/SMS/link -> citizen approves or denies -> verification proceeds only after approval`

Citizen approval applies to one specific verification request. It must not permanently authorize an official or department.

The future request lifecycle should support:

- `pending`
- `approved`
- `denied`
- `expired`
- `completed`

The repository currently has no verification-request table, citizen contact/identity model, notification integration, approval token, consent API, request state machine, frontend route, or authenticated QR landing flow. The current QR contains a bare document ID. Changing it to a URL affects stamped bytes and therefore must happen before hashing newly issued documents; existing stamped documents need a compatibility path.

## 10. BLOCKCHAIN RULE

Do not unnecessarily redesign or rewrite the existing Solidity contract.

The blockchain registry remains responsible for credential hashes, document IDs, issuer-wallet/document-type enforcement, and lifecycle state (`VALID`, `SUPERSEDED`, `REVOKED`). Application-level human roles (`CONTROLLER`, `ADMIN`, `OFFICIAL`) belong in the Go backend using the authenticated Supabase identity and backend profile.

Blockchain department identity and application user identity are separate:

- the contract sees a department Ethereum signer;
- the application sees an individual Supabase user;
- future audit records connect the human actor to the backend operation and blockchain transaction.

Preserve the existing Solidity lifecycle semantics and Go verification ordering unless a specific, tested requirement requires a contract change.

## 11. FRONTEND/BACKEND CONTRACT

Frontend responsibilities:

- UI and UX;
- navigation;
- displaying backend data;
- client-side form validation;
- acquiring the Supabase access token.

Backend responsibilities:

- authentication enforcement;
- authorization and department scoping;
- business and document-lifecycle rules;
- audit logging;
- citizen-consent rules when that future phase is implemented;
- blockchain signing and interaction;
- authoritative validation of all client input.

Keep frontend API calls and TypeScript contracts centralized in `frontend/src/api.ts` or a deliberate successor API module. Do not scatter new `fetch()` calls through components.

All authenticated backend requests must send:

`Authorization: Bearer <Supabase access token>`

Do not invent response fields or endpoints. If the frontend needs a role, department, consent state, or audit field that the backend does not expose, agree on and implement the API contract first.

## 12. LOCAL DEVELOPMENT AND CONFIGURATION

- The local demo requires Go 1.22+, Node 20+, Foundry/Anvil, and a reachable Supabase project.
- The backend requires `SUPABASE_URL`. It also supports `RPC_URL`, `DB_PATH`, `CONTRACT_ADDRESS` or `DEPLOYMENT_FILE`, and `ADDR`.
- The frontend requires `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY`; `VITE_API_URL` optionally overrides the backend URL. Keep local `.env` files untracked.
- A Supabase account's JWT `sub` must exactly match `user_profiles.supabase_user_id`. The backend never derives application roles from JWT metadata.
- `make demo` starts Anvil, deploys the contract, provisions three department Admin profiles, seeds authenticated demo credentials, and starts the backend and frontend. It requires matching `SEED_*_USER_ID` and `SEED_*_TOKEN` environment variables for the three demo departments.
- `backend/cmd/profile-seed` is the non-interactive local profile provisioning path. Profile changes and their audit entries commit atomically.
- Use `make stop` to stop local services. Direct validation commands remain `GOCACHE=/tmp/credreg-go-cache go test ./...` from `backend`, `npm run build` from `frontend`, and `forge test` from `contracts` only when Solidity changes.

## 13. GIT / DEVELOPMENT RULES

- Do not work directly on `main`; use a focused feature branch.
- Inspect `git status` before work and do not overwrite unrelated or uncommitted user changes.
- Review `git diff` before committing.
- Run relevant tests and builds before pushing.
- Never commit `.env` files, JWTs, private keys, Supabase service-role keys, access tokens, or other secrets.
- The Anvil keys currently in `backend/internal/chain/chain.go` are demo-only and must not be treated as a production key-management pattern.
- Do not rewrite working blockchain, PDF stamping, hashing, document-ID, or verification logic without a specific reason and regression coverage.
- Do not commit or push unless the user explicitly instructs you to do so.

## 14. CODEX WORKING RULES

- Inspect the relevant repository state before modifying anything.
- Prefer small, incremental changes with narrow scope.
- Do not perform broad refactors unless they are necessary and the reason is explained.
- Preserve existing working behavior and tests.
- Run Go tests after backend changes: from `backend`, run `go test ./...`.
- Run Foundry tests after contract changes: from `contracts`, run `forge test`.
- Run the frontend build after frontend changes: from `frontend`, run `npm run build`.
- Run the most focused relevant tests during development, then the appropriate broader suite before handoff.
- Review `git diff` and `git status` after modifications.
- Never commit or push unless explicitly instructed.
- If requirements are ambiguous, explain the ambiguity before making a major architectural decision.
- Do not assume an endpoint, schema, permission, notification provider, or response field exists merely because it appears in planned architecture.
- Clearly distinguish **CURRENT** functionality from **PLANNED** functionality in implementation notes and handoffs.
- Preserve user changes in a dirty worktree and do not assume they belong to the current task.

## 15. CURRENT PROJECT STATUS

### A. Already implemented

- Solidity credential records, document ID index, department wallet/document-type roles, issue, verify, supersede, revoke, and lifecycle events.
- Foundry coverage for contract permissions and lifecycle behavior.
- Go Ethereum client and generated contract binding.
- SHA-256 anchoring of stamped PDF bytes.
- PDF marker/QR stamping with incremental-update and fallback behavior.
- Verification verdict state machine with Go tests.
- SQLite document/PDF storage, citizen lookup, credential listing, and download.
- Issue, verify, supersede, and revoke backend handlers.
- Verify, Issue, Citizen, and Login frontend screens.
- Supabase email/password login and session handling.
- Supabase ES256/JWKS Go verification, bearer middleware, context identity extraction, and temporary `/auth-test`.
- A successful manual real Supabase JWT-to-Go-backend authentication test.
- SQLite department and user-profile models with role/department constraints.
- Reusable backend RBAC permissions for Controller, Admin, and Official.
- `/me`, protected credential routes, and department-scoped operations/read access.
- Centralized authenticated frontend API requests and protected PDF downloads.
- Hash-chained credential-operation and profile-change audit logging.
- Controller system-wide audit/monitoring APIs and Admin department-scoped audit access.
- Authoritative frontend/backend contract documentation in `docs/API.md`.
- Minimal temporary Controller monitoring, Controller/Admin audit-event, and Admin lifecycle validation screens.

### B. Partially implemented / known limitations

- Department handling: application display names/profiles are data-driven, but chain signer definitions and demo private keys remain compiled into the backend.
- QR flow: a QR is stamped, but it carries only a document ID and there is no routed/authenticated consent flow.
- Audit hash chaining detects ordinary row tampering but has no external checkpoint; a database administrator can replace or recompute the entire SQLite history.
- A chain-success/SQLite-save failure is recorded as `PARTIAL_FAILURE`, but an unavailable/corrupt audit database cannot record its own logging failure.
- Monitoring statistics cover retained audit events, not operations performed before audit logging was introduced.
- Supabase authentication has been manually validated end-to-end, but there is no automated live-Supabase integration test.
- Monitoring, Audit Events, Revoke, and Supersede frontend screens are intentionally minimal temporary validation UI, not the frontend team's final dashboard.

### C. Planned/not implemented

- Final Controller dashboard UI (the backend contracts and temporary validation UI are implemented).
- Citizen identity/contact ownership model.
- Verification requests, notifications, one-time approval links, consent states, and expiry/replay controls.
- Frontend routing and QR-to-verification-page navigation.
- Production wallet/key management.

## 16. CURRENT NEXT PHASE

Phase 1 — RBAC Foundation and Phase 2 — Audit + Administrative Monitoring are complete, including the temporary browser-validation UI. The next phase must be explicitly agreed before implementation. Do not infer or begin citizen consent as maintenance to Phase 2.

Before changing code in a future session:

1. Inspect the current worktree and relevant files.
2. Verify which authentication changes are committed versus still in progress.
3. Agree on the smallest safe schema/API increment.
4. Preserve the credential lifecycle and verification invariants.
5. Run relevant tests and builds.
6. Review the final diff.

## CURRENT STATE / NEXT TASK

**Current state:** Core credential lifecycle, PDF stamping, SQLite persistence, blockchain anchoring, frontend login, Supabase JWT verification, backend-owned profiles/departments, `/me`, role/department authorization, hash-chained operation/profile audit logging, atomic audited profile changes, strict upload limits, scoped audit APIs, Controller monitoring APIs, and a temporary Phase 2 browser-validation UI are implemented. Citizen consent and the final Controller dashboard UI are not implemented.

**Next task:** Agree on the next bounded phase before editing. Do not begin citizen consent or broaden the Controller UI as unrelated Phase 2 maintenance.
