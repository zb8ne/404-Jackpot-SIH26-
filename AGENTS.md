# 404-Jackpot-SIH26 Repository Guide

This file preserves repository context for future Codex sessions. Treat the checked-out code as the final source of truth and update this file when a major phase changes the architecture or project status.

## 1. PROJECT OVERVIEW

404-Jackpot-SIH26 is a government credential registry prototype. Government departments issue PDF credentials to citizens, the backend stamps each issued PDF with a registry marker and QR code, and the SHA-256 hash of the stamped bytes is anchored in an Ethereum smart contract. The PDF itself and citizen-facing metadata remain off-chain in SQLite.

Verification distinguishes an authentic current document from an authentic superseded or revoked document, a modified document, and a document that was never issued. Superseding adds a replacement without deleting the original, preserving credential history.

The repository is a demo built around Anvil. Phase 1 Supabase/RBAC, Phase 2 audit/monitoring, and Phase 3 QR-based citizen-consent verification are implemented. It can run locally as a Docker Compose stack or directly on the host, and it includes container/Vercel configuration for hosted demonstrations.

This guide currently describes the `demo-ui` line of development. The Demo UI commits are present on `demo-ui` but are not yet merged into `main` as of commit `f58f8d1`.

## 2. CURRENT ARCHITECTURE

### Frontend

- React 19 + TypeScript, built with Vite and styled with Tailwind CSS.
- Entry point: `frontend/src/main.tsx`; application shell: `frontend/src/App.tsx`.
- There is currently no router. The application switches between tabs in `App.tsx`.
- Supabase client setup is in `frontend/src/lib/supabase.ts`.
- Backend request functions and shared response types, including `/health`, are centralized in `frontend/src/api.ts`.
- `/demo` is selected through pathname handling in `App.tsx` and lazy-loads `frontend/src/screens/Demo.tsx`; it is not a React Router route.
- The Demo UI uses PixiJS 8 and the scene implementation under `frontend/src/scene/`. The authenticated application lazy-loads a collapsible `LiveFloor`; `/demo` provides a standalone guided/projector view that can also run without authentication in preview mode.
- `frontend/vercel.json` builds the Vite application and rewrites all paths to `index.html`, which preserves the pathname-based `/demo`, `/verify`, and `/consent/...` entry points on Vercel.

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

SQLite stores stamped PDFs and metadata, stable departments, government profiles/RBAC, citizen accounts and contact data, document-to-citizen linkage, verification requests, notification attempts, consent state, and hash-chained audit events. It does not store authoritative credential lifecycle status; live lifecycle status remains on Ethereum.

### Solidity/Ethereum registry

`contracts/src/CredentialRegistry.sol` stores credential hashes and lifecycle state. Foundry tests are in `contracts/test/CredentialRegistry.t.sol`; deployment is in `contracts/script/Deploy.s.sol`. The demo uses local Anvil accounts and keys currently embedded in the Go chain package.

### Runtime and deployment topology

- `docker-compose.yml` is the preferred local path. It starts Anvil, deploys the contract, provisions configured profiles plus demo citizens, starts the backend, conditionally seeds documents through the authenticated REST API, and starts the Vite frontend.
- Doppler is only an environment-injection mechanism in this repository: `doppler run -- docker compose up` supplies the same variables that a local untracked `.env` file can supply. There is no committed Doppler project configuration or runtime dependency in application code.
- `backend/Dockerfile` and `frontend/Dockerfile` build the split local Compose services.
- The root `Dockerfile` plus `deploy/standalone/start.sh` are a standalone hosted-backend target used for Railway-style deployment. One container starts a loopback-only Anvil node, restores/persists its state under `DATA_DIR`, deploys the registry on first boot, and then starts the Go server. A persistent volume is required if chain and SQLite state must survive replacement/restart.
- The backend honors a platform-provided `PORT` before `ADDR`. The root standalone image exposes `8088`, but the effective listener may be assigned by the hosting platform.

### Current interaction

1. The frontend signs a user into Supabase and receives a session.
2. The centralized frontend API layer attaches the session access token to protected requests.
3. The backend verifies the token, loads the backend-owned profile, and enforces role/department permissions.
4. For authorized credential writes, the backend derives the department from the profile, selects the matching hardcoded signer, and submits a transaction to the registry contract.
5. The backend stores stamped PDFs and off-chain metadata in SQLite.
6. Verification reads the authoritative hash/status from the contract and supplements it with SQLite metadata when available.
7. Important credential operations and profile authorization changes append hash-chained SQLite audit events with the human actor, department, outcome, and relevant credential/transaction references.
8. A QR verification URL leads an authenticated Admin/Official into a one-time request. The citizen approves or denies through an expiring email token; approved completion rereads the requested hash's current Solidity status.
9. Successful frontend operations publish typed scene events to an in-process bus. The Demo UI translates them into queued PixiJS animations; eligible roles also receive best-effort cross-tab events from audit polling.

## 3. EXISTING FUNCTIONALITY

### Document issuing

`POST /issue` in `backend/internal/api/api.go` accepts multipart fields `file`, `dept`, `doc_type`, `doc_id`, legacy `citizen`, and required `citizen_account_id`. It loads the active citizen account, derives the display name/linkage, rejects already-marked uploads, stamps the PDF, hashes the stamped bytes, submits `issue` to the contract, and saves the stamped PDF.

The order is intentionally:

`stamp -> SHA-256(stamped bytes) -> anchor hash -> store stamped PDF`

Do not reverse or bypass this ordering. The citizen's stamped copy must be the exact byte sequence anchored on-chain.

### Document IDs, PDF stamping, and QR

- `backend/internal/docid/docid.go` defines and extracts `CREDREG-DOCID:<docId>`.
- `backend/internal/pdfdoc/pdfdoc.go` appends a registry page using a PDF incremental update, leaving original bytes untouched.
- The page includes a vector QR and visible marker.
- If the simple PDF parser cannot handle a file, stamping falls back to a trailing PDF comment containing the marker.
- New credentials encode `{PUBLIC_WEB_URL}/verify?docId=...`; legacy `Stamp` and existing PDFs retain bare document-ID compatibility.
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

- `frontend/src/screens/Login.tsx`: role-intent selection for Controller, Government Authority (Admin/Official), and Citizen followed by Supabase email/password sign-in. The selection is session-scoped presentation state only; backend profile resolution remains authoritative.
- `frontend/src/screens/Verify.tsx`: PDF upload/drop verification and result display, with a helper path for verifying an ID.
- `frontend/src/screens/Issue.tsx`: department selection, citizen/document fields, PDF upload, client-side ID generation, and stamped-document result/download.
- `frontend/src/screens/Citizen.tsx`: citizen selection and credential history.
- `frontend/src/screens/Monitoring.tsx` and `frontend/src/screens/AuditEvents.tsx`: minimal temporary Phase 2 browser-validation UI, intentionally replaceable by the frontend team's final dashboard.
- `frontend/src/screens/Lifecycle.tsx`: minimal temporary Admin-only revoke/supersede validation forms using the existing Phase 2 API contracts.
- `frontend/src/screens/Demo.tsx`: standalone `/demo` PixiJS registry-floor presentation with a nine-step role-aware guided walkthrough and a manual preview-event panel.
- `frontend/src/scene/LiveFloor.tsx`: collapsible live visualization inside the authenticated application shell.
- `frontend/src/verdicts.ts`: centralized verdict display configuration.

### Demo UI event model

- `frontend/src/scene/sceneApi.ts` defines the `SceneEvent` vocabulary for issue, verify, supersede, revoke, and consent activity.
- `frontend/src/api.ts` remains the application API boundary and emits scene events after the relevant API calls. Do not bypass it with new component-level `fetch()` calls merely to drive the visualization.
- `frontend/src/scene/apiBus.ts` is an in-memory publish/subscribe bus. It is the primary source for actions performed in the current tab and includes an eight-second duplicate guard.
- `frontend/src/scene/auditPoll.ts` polls `/audit-events` every four seconds as a secondary source for actions performed elsewhere, such as a citizen decision. It is available only to roles allowed to read audit events and stops after a `403`; OFFICIAL users therefore rely on the in-tab bus.
- `frontend/src/scene/choreography.ts` queues events and plays one animation at a time using simulated Pixi ticker time. A backgrounded browser tab may pause animation because `requestAnimationFrame` is suspended.
- `frontend/src/scene/guidedScript.ts` performs real backend calls only when the current profile has permission. ADMIN can run the complete sequence; OFFICIAL runs issue/verify/consent steps and previews lifecycle steps; CONTROLLER and anonymous visitors preview the sequence.
- `frontend/public/demo/blank-credential.pdf` is the input used by real guided issue/supersede steps. Real guided mode also requires seeded citizen accounts and valid backend/Supabase configuration.
- The vendored LimeZu tilesets and sprites are licensed for non-commercial use only. See `frontend/src/scene/assets/tilesets/LIMEZUASSETS-LICENSE.txt`; replace them before any commercial use.

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
- `GET /citizen-accounts` (Admin/Official, masked issuance choices)
- `POST /verification-requests`, `GET /verification-requests`, `GET /verification-requests/{id}`, and `POST /verification-requests/{id}/complete`
- `GET /consent/{token}`, `POST /consent/{token}/approve`, and `POST /consent/{token}/deny` (token-authenticated, no Supabase citizen login)
- `GET /development/notifications/{id}` (authenticated/scoped in-memory demo email capture)

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

Phase 3 implements:

`QR scan -> verification page -> government user authenticates -> request + purpose -> development email capture -> citizen email link -> approve/deny -> approved requester completes -> current chain verdict`

Citizen approval applies to one specific verification request. It must not permanently authorize an official or department.

The request lifecycle is:

- `PENDING`
- `APPROVED`
- `DENIED`
- `EXPIRED`
- `COMPLETED`

Citizens are separate from government `user_profiles`; `CITIZEN` is not an RBAC role. Citizen accounts require email and are provisioned with `backend/cmd/citizen-seed`. The optional unique `supabase_user_id` links a citizen account to Supabase Auth, and `CitizenAccountBySupabaseUserID` is the trusted lookup for future authenticated citizen handlers. Routine reseeding without `-supabase-user-id` preserves an existing identity link. Raw random tokens/URLs exist only in the authenticated in-memory development notification capture; SQLite stores SHA-256 token hashes. Tokens expire after 15 minutes and are request-specific. Consent decisions and completion use guarded atomic transitions, and consent/audit writes are atomic. No scheduler or real email provider is included.

`POST /verify` remains the authenticated file-possession authenticity check. Direct `GET /verify/{docId}` returns `409` for linked credentials so it cannot bypass consent; it remains compatible for old unlinked credentials. The QR frontend never calls direct ID verification.

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
- citizen-consent request, expiry, decision, and completion rules;
- blockchain signing and interaction;
- authoritative validation of all client input.

Keep frontend API calls and TypeScript contracts centralized in `frontend/src/api.ts` or a deliberate successor API module. Do not scatter new `fetch()` calls through components.

All authenticated backend requests must send:

`Authorization: Bearer <Supabase access token>`

Do not invent response fields or endpoints. If the frontend needs a role, department, consent state, or audit field that the backend does not expose, agree on and implement the API contract first.

## 12. LOCAL DEVELOPMENT AND CONFIGURATION

- The preferred local path requires Docker, Docker Compose, and a reachable Supabase project. Doppler is optional; it can inject configuration instead of a local `.env`.
- Copy `.env.example` to an untracked `.env` when not using Doppler. Required values are `SUPABASE_URL`, `VITE_SUPABASE_URL`, and `VITE_SUPABASE_PUBLISHABLE_KEY`. Government `SEED_*_USER_ID` values are optional and each non-empty ID provisions that profile. Demo-document seeding runs only when all three department `SEED_*_TOKEN` values are present; the tokens are short-lived and must correspond to their Admin profiles.
- With Doppler configured for the repository, start the complete stack from the repository root with `doppler run -- docker compose up`. Without Doppler, use `docker compose up`; Compose automatically reads `.env`.
- The Compose dependency order is `init-volumes`/`anvil` -> `deploy` -> `profiles` -> `backend` -> `seed`, with `frontend` depending on the backend. `deploy`, `profiles`, and `seed` are expected one-shot containers that exit successfully.
- Default local addresses are frontend `http://127.0.0.1:5173`, backend `http://127.0.0.1:8088`, and Anvil `http://127.0.0.1:8545`. Override collisions with `WEB_PORT`, `API_PORT`, or `ANVIL_PORT`; keep `VITE_API_URL` and `PUBLIC_WEB_URL` consistent with browser-reachable addresses.
- Use `docker compose down` to stop while retaining named-volume state. `docker compose down -v` also deletes the Compose chain, deployment, and SQLite volumes and is intentionally destructive to demo state. Inspect a service with `docker compose logs -f backend` (or another service name).
- The direct-host fallback requires Go 1.22+, Node 20+, Foundry/Anvil, and the same Supabase values. Run `make demo` from the repository root and `make stop` when finished. Be aware that `make demo` begins with `clean` and deletes local demo state/artifacts before rebuilding.
- The backend requires `SUPABASE_URL`. It also supports `RPC_URL`, `DB_PATH`, `CONTRACT_ADDRESS` or `DEPLOYMENT_FILE`, and `ADDR`.
- The frontend requires `VITE_SUPABASE_URL` and `VITE_SUPABASE_PUBLISHABLE_KEY`; `VITE_API_URL` optionally overrides the backend URL. The backend `PUBLIC_WEB_URL` explicitly controls QR/consent URLs. Keep local `.env` files untracked.
- A Supabase account's JWT `sub` must exactly match `user_profiles.supabase_user_id`. The backend never derives application roles from JWT metadata.
- The direct-host `make demo` path still expects the three department Admin IDs and tokens. Flexible partial-profile provisioning is currently specific to Docker Compose.
- `backend/cmd/profile-seed` is the non-interactive local profile provisioning path. Profile changes and their audit entries commit atomically.
- Direct validation commands remain `GOCACHE=/tmp/credreg-go-cache go test ./...` from `backend`, `npm run build` and `npm run lint` from `frontend`, and `forge test` from `contracts` when Solidity changes.

### Quick local run checklist

1. Ensure at least one Supabase government account exists and copy its stable user ID. A full seeded fixture requires all three department Admins.
2. For the full fixture only, obtain fresh access tokens for all three Admin accounts.
3. Configure the applicable variables from `.env.example` in Doppler, or place them in an untracked `.env`.
4. Run `doppler run -- docker compose up` or `docker compose up` from the repository root.
5. Wait for the one-shot `deploy`, `profiles`, and `seed` services to exit successfully and for the three long-running services to remain up.
6. Open `http://127.0.0.1:5173` for the authenticated app or `http://127.0.0.1:5173/demo` for the registry-floor presentation.
7. Use preview mode at `/demo` without signing in. Sign in as ADMIN or OFFICIAL only when intentionally exercising the guided script against the live backend; those live steps create and mutate demo credentials.
8. Stop with `docker compose down`. Add `-v` only when deliberately resetting the local chain and database.

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
- Separate citizen accounts with mandatory email and controlled provisioning/linking CLI.
- URL-based QR stamping for new credentials with legacy bare-ID compatibility.
- Consent-gated verification requests, expiring hashed tokens, atomic approve/deny/complete transitions, and current-chain completion.
- Minimal QR landing, request status/completion, and public citizen consent screens.
- In-memory authenticated development notification capture with redacted persistent attempts.
- Docker Compose orchestration for Anvil, deployment, profile/citizen provisioning, backend, authenticated document seeding, and the Vite frontend.
- Optional Doppler-driven environment injection, with `.env.example` as the non-Doppler configuration template.
- Split backend/frontend container images, Vercel SPA rewrite configuration, and a standalone root image that co-locates Anvil and the Go backend for Railway-style hosting.
- A PixiJS registry-floor Demo UI on the unmerged `demo-ui` branch, including preview controls, role-aware guided execution, an authenticated live floor, API-bus events, and audit-poll fallback.

### B. Partially implemented / known limitations

- Department handling: application display names/profiles are data-driven, but chain signer definitions and demo private keys remain compiled into the backend.
- Frontend routing: Phase 3 uses minimal pathname/query handling rather than a full router.
- Audit hash chaining detects ordinary row tampering but has no external checkpoint; a database administrator can replace or recompute the entire SQLite history.
- A chain-success/SQLite-save failure is recorded as `PARTIAL_FAILURE`, but an unavailable/corrupt audit database cannot record its own logging failure.
- Monitoring statistics cover retained audit events, not operations performed before audit logging was introduced.
- Supabase authentication has been manually validated end-to-end, but there is no automated live-Supabase integration test.
- Monitoring, Audit Events, Revoke, and Supersede frontend screens are intentionally minimal temporary validation UI, not the frontend team's final dashboard.
- Development notifications do not send real email and disappear on backend restart.
- Existing unlinked credentials cannot use secure consent until linked explicitly with controlled tooling.
- The Demo UI visualization has been browser-validated in preview mode, but the commit history does not establish a complete browser-validated live walkthrough for every RBAC role and hosted environment.
- `/demo` and the consent/verification entry paths still use manual pathname handling rather than a router.
- Demo scene art is licensed for non-commercial use only and is unsuitable for a commercial release without replacement.
- Doppler supplies environment variables only; the repository contains no committed Doppler project definition. Hosted service configuration outside the checked-in Docker/Vercel files remains external state.

### C. Planned/not implemented

- Final Controller dashboard UI (the backend contracts and temporary validation UI are implemented).
- Real email provider integration and production notification delivery/retry policy.
- A final routed frontend architecture; Phase 3 uses minimal pathname/query handling.
- Production wallet/key management.
- Replacement or appropriately licensed production artwork for the Demo UI.

## 16. CURRENT NEXT PHASE

Phase 1 — RBAC Foundation, Phase 2 — Audit + Administrative Monitoring, and Phase 3 — QR Citizen Consent are implemented. Docker/Vercel/Railway-oriented demo deployment work is present on `main`. The PixiJS Demo UI is implemented on `demo-ui` through commit `f58f8d1` but is not merged into `main`. The next functional phase must be explicitly agreed before implementation.

Before changing code in a future session:

1. Inspect the current worktree and relevant files.
2. Verify which authentication changes are committed versus still in progress.
3. Agree on the smallest safe schema/API increment.
4. Preserve the credential lifecycle and verification invariants.
5. Run relevant tests and builds.
6. Review the final diff.

## CURRENT STATE / NEXT TASK

**Current state:** Core lifecycle, URL/bare-ID compatible stamping, SQLite persistence, blockchain anchoring, Supabase government authentication, backend RBAC, audit/monitoring, citizen accounts and document linkage, consent requests, hashed one-time email tokens, current-chain completion, Docker/hosted-demo configuration, and minimal Phase 2/3 validation UI are implemented. This `demo-ui` branch additionally contains the PixiJS registry-floor visualization, preview driver, live event bridge, audit fallback, and role-aware guided walkthrough.

**Next task:** Validate the Demo UI's live guided path end-to-end with configured Supabase accounts and the Docker Compose stack, decide whether/how to merge `demo-ui`, then agree on the next bounded phase. Do not add real notification infrastructure or redesign the frontend without explicit scope.
