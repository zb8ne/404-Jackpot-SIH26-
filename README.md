# Government Credential Registry

Departments issue documents to citizens. Anyone can check that a document is authentic
and current. A smart contract holds the hash and metadata; the PDFs themselves stay
off-chain. Tampering shows up as a hash mismatch.

## Run it

```sh
make demo
```

One command from a clean clone: boots Anvil, deploys the registry, seeds the three
department roles, issues three documents each to two citizens, corrects one, revokes
another, and starts the REST API. Takes about 15 seconds.

```sh
make stop     # kill anvil + backend
make test     # contract tests + the verify state machine
```

Needs Go 1.22+ and [Foundry](https://getfoundry.sh) (`curl -L https://foundry.paradigm.xyz | bash && foundryup`).

## The demo

`make demo` writes signed PDFs to `./demo-files/`, one for each verdict:

| File | Verdict | |
| --- | --- | --- |
| `asha-menon-birth-certificate-v2.pdf` | `VALID` | hashes match |
| `asha-menon-birth-certificate-v1.pdf` | `SUPERSEDED` | genuine, but corrected by v2 |
| `rahul-iyer-driving-licence.pdf` | `REVOKED` | genuine bytes, revoked by Transport Dept |
| `rahul-iyer-degree-TAMPERED.pdf` | `TAMPERED` | class upgraded, docId marker intact |
| `never-issued-driving-licence.pdf` | `NOT_ISSUED` | well-formed, but no such docId |

```sh
curl -s -F file=@demo-files/asha-menon-birth-certificate-v1.pdf localhost:8088/verify | jq
curl -s -F file=@demo-files/rahul-iyer-degree-TAMPERED.pdf      localhost:8088/verify | jq
curl -s localhost:8088/verify/BC-2019-004471 | jq   # what a QR scan hits
curl -s 'localhost:8088/credentials/Asha%20Menon'  | jq   # v1 and v2, side by side
```

### Corrections keep their history

Asha's birth certificate was issued with her name misspelled. The Birth Dept does not
edit it — it issues a corrected v2 and supersedes v1. Both stay on the record:

```
BC-2019-004471     birth_certificate -> SUPERSEDED
BC-2019-004471-R1  birth_certificate -> VALID
```

Verifying the untouched v1 returns `SUPERSEDED` along with a `supersededBy` pointer to the
version that replaced it, so a verifier holding an out-of-date copy is sent to the live one
rather than told it is fake. Scanning v1's QR resolves the same way.

### How a document is identified

Every issued PDF carries its id twice: a **QR code** encoding the docId, and a plain-text
marker in the content stream reading exactly `CREDREG-DOCID:<docId>`. Verification reads
the marker — the QR is for scanning on stage, and is never decoded server-side.

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
| POST | `/supersede` | multipart: `file`, `dept`, `doc_id`, `citizen`, `old_hash` |
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
