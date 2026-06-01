# CarbonStackCypher

CarbonStackCypher is the experimental relay/storage server for CarbonStack.

**It stores opaque envelopes, is not externally audited, does not handle plaintext, does not decide trust, is not production-certified.**

The current validated role of Cypher is to relay CarbonStackComms/OpenMLS sidecar artifacts through a simple HTTP JSON envelope API.

_Related repositories: [carbonstack](https://git.bitcrusher32.win/bitcrusher32/carbonstack) / [carbonstack-comms](https://git.bitcrusher32.win/bitcrusher32/carbonstack-comms) / [carbonstack-os](https://git.bitcrusher32.win/bitcrusher32/carbonstack-os)_


## Current implemented behavior

Cypher currently provides:

- health check;
- development invite/account/device registration;
- envelope submission;
- recipient inbox listing;
- envelope acknowledgement;
- OpenMLS artifact content-type support;
- payload metadata over decoded envelope payload bytes.

Current OpenMLS relay content types:

    carbonstack.mls.keypackage.v0
    carbonstack.mls.welcome.v0
    carbonstack.mls.application-message.v0

Current OpenMLS relay protocol version:

    carbonstack-openmls-sidecar-v0

Existing stub content type:

    carbonstack.message.text.stub.v0

## Payload model

Envelope payloads are stored as:

    ciphertext_b64

For OpenMLS relay artifacts, this field carries opaque artifact bytes encoded as base64.

Cypher computes:

    payload_size_bytes
    payload_sha256

Both describe decoded `ciphertext_b64` bytes.

These fields are relay/debug/storage sanity metadata. They are not a cryptographic trust root.

## Routes

Current route family:

    GET  /v0/health
    POST /v0/invites/claim
    POST /v0/devices/register
    GET  /v0/accounts/{account_id}/devices
    POST /v0/envelopes
    GET  /v0/devices/{device_id}/envelopes
    POST /v0/envelopes/{envelope_id}/ack

## Database

Cypher uses SQLite for current development and smoke testing.

Current migrations:

    migrations/001_init.sql
    migrations/002_envelope_payload_metadata.sql

The schema is pre-release and may change before any production claim.

## Known-good validation

Run from this repository:

    go test ./... -count=1

For the full CarbonStack OpenMLS relay proof, use the runbook in the main `carbonstack` repo:

    docs/113-experimental-backbone-deployability-runbook-v0.md

## What Cypher does not prove

Cypher does not currently prove:

- production E2EE;
- hostile-server safety;
- metadata privacy;
- secure identity;
- secure local vault/storage;
- rollback/replay safety against a malicious server;
- multi-user production operations;
- external audit or certification.

Cypher is a relay/storage component inside the current experimental backbone.

---

License: MIT.
See the repository's LICENSE file for more information.

