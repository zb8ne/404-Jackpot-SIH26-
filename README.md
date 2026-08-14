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

`make demo` writes signed PDFs to `./demo-files/`:

| File | Verdict |
| --- | --- |
| any `*.pdf` | `VALID` |
| `rahul-iyer-driving-licence.pdf` | `REVOKED` — genuine bytes, revoked by Transport Dept |
| `rahul-iyer-driving-licence-TAMPERED.pdf` | `TAMPERED_OR_NOT_FOUND` — one field edited |

```sh
curl -s -F file=@demo-files/asha-menon-degree.pdf                  localhost:8088/verify | jq
curl -s -F file=@demo-files/rahul-iyer-driving-licence-TAMPERED.pdf localhost:8088/verify | jq
curl -s 'localhost:8088/credentials/Asha%20Menon' | jq
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
`status` one of `VALID | SUPERSEDED | REVOKED`. `issue`, `supersede` and `revoke` are all
gated on `issuerRole[msg.sender]` matching the document type. Anvil's prefunded accounts
0, 1 and 2 are the Birth, Transport and Education departments.

### API

| Method | Path | |
| --- | --- | --- |
| POST | `/issue` | multipart: `file`, `dept`, `doc_type`, `doc_id`, `citizen` |
| POST | `/verify` | multipart: `file` → `VALID` / `SUPERSEDED` / `REVOKED` / `TAMPERED_OR_NOT_FOUND` |
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
