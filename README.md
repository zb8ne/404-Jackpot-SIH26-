# Government Credential Registry

Departments issue documents to citizens. Anyone can check that a document is authentic
and current. A smart contract holds the hash and metadata; the PDFs themselves stay
off-chain. Tampering shows up as a hash mismatch.

## Run it

```sh
make demo
```

One command from a clean clone: boots Anvil, deploys the registry, seeds the three
department roles, issues three documents each to two citizens, revokes one, and starts
the REST API. Takes about 15 seconds.

```sh
make stop     # kill anvil + backend
make test     # contract tests only
```

Needs Go 1.22+ and [Foundry](https://getfoundry.sh) (`curl -L https://foundry.paradigm.xyz | bash && foundryup`).

## The demo

`make demo` writes signed PDFs to `./demo-files/`, one for each verdict:

| File | Verdict | |
| --- | --- | --- |
| `asha-menon-degree.pdf` | `VALID` | hashes match |
| `rahul-iyer-driving-licence.pdf` | `REVOKED` | genuine bytes, revoked by Transport Dept |
| `asha-menon-birth-certificate-TAMPERED.pdf` | `TAMPERED` | body edited, docId marker intact |
| `never-issued-driving-licence.pdf` | `NOT_ISSUED` | well-formed, but no such docId |

```sh
curl -s -F file=@demo-files/asha-menon-degree.pdf                     localhost:8088/verify | jq
curl -s -F file=@demo-files/asha-menon-birth-certificate-TAMPERED.pdf localhost:8088/verify | jq
curl -s localhost:8088/verify/BC-2019-004471 | jq   # what a QR scan hits
curl -s 'localhost:8088/credentials/Asha%20Menon'  | jq
```

### How a document is identified

Every issued PDF carries its id twice: a **QR code** encoding the docId, and a plain-text
marker in the content stream reading exactly `CREDREG-DOCID:<docId>`. Verification reads
the marker — the QR is for scanning on stage, and is never decoded server-side.

Looking the document up by id first, rather than by hash, is what lets the registry tell
these two apart:

```
no marker, or unknown docId   -> NOT_ISSUED   this document was never issued
docId known, hashes differ    -> TAMPERED     it was issued, but these bytes changed
docId known, hashes match     -> VALID / SUPERSEDED / REVOKED
```

`/verify` always returns both `expectedHash` and `computedHash`, even when they match, so
a verifier can show them side by side:

```json
{
  "status": "TAMPERED",
  "docId": "BC-2019-004471",
  "expectedHash": "0x014dfc0d...",
  "computedHash": "0x75900631...",
  "message": "document BC-2019-004471 exists, but this file does not match the hash on record ..."
}
```

The centrepiece is the role gate. Transport Dept holds the driving-licence role, so the
contract rejects it issuing a birth certificate:

```sh
curl -s -F file=@demo-files/asha-menon-degree.pdf -F dept=transport \
     -F doc_type=birth_certificate -F doc_id=BC-FAKE-1 -F citizen=Mallory \
     localhost:8088/issue
# 403 — contract rejected the call (department not authorised for this document type)
```

Same rule, proved at the contract level, in `test_TransportDeptCannotIssueBirthCertificate`.

## Layout

```
contracts/   Solidity + Foundry tests + deploy script
backend/     Go REST API (ethclient + abigen bindings, SQLite for PDFs)
```

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

| Method | Path | |
| --- | --- | --- |
| POST | `/issue` | multipart: `file`, `dept`, `doc_type`, `doc_id`, `citizen` |
| POST | `/verify` | multipart: `file` → `VALID` / `SUPERSEDED` / `REVOKED` / `TAMPERED` / `NOT_ISSUED` |
| GET | `/verify/{docId}` | same shape minus `computedHash` — the QR-scan path |
| POST | `/revoke` | json: `docHash`, `dept` |
| GET | `/credentials/{citizen}` | documents with live on-chain status |
| GET | `/departments`, `/citizens`, `/health` | |
| GET | `/documents/{hash}/download` | the stored PDF |

## Demo-grade shortcuts

- Hashes are taken over the raw PDF bytes, so re-saving a file breaks verification. Fine
  here — it is what makes tampering detectable at all.
- No auth. The department is a picker, and its private key is an Anvil default baked into
  the backend. Never do this anywhere real.
- Local Anvil only, no testnet, no internet.

Frontend (React + TypeScript + Tailwind) is not built yet.
