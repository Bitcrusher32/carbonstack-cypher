# CarbonStackCypher Storage Model

Status: current development storage model
Component: CarbonStackCypher
Maturity: experimental / pre-release

Cypher storage exists to route opaque envelopes and operate the development server.

It must not become a plaintext archive.

It must not become an identity oracle.

It must not become a hidden recovery service.

## Current storage backend

Current backend:

    SQLite

Current use:

- development server;
- local smoke tests;
- experimental backbone validation.

The current schema is not a production database contract.

## Current migrations

Current migrations:

    migrations/001_init.sql
    migrations/002_envelope_payload_metadata.sql

## Current tables

### invites

Development invite records.

Not a production authentication system.

### accounts

Development account namespace records.

Display names are not trusted identity.

### devices

Device routing/accounting records.

They carry public identity/prekey material for the current scaffold.

They do not carry private keys.

### envelopes

Opaque relay envelope records.

Current envelope fields include:

- envelope ID;
- sender device ID;
- recipient device ID;
- content type;
- protocol version;
- base64 payload;
- payload SHA-256;
- payload decoded size;
- client/server timestamps;
- delivery state.

The payload is not plaintext.

Cypher does not parse OpenMLS internals.

### envelope_acks

Ack records.

Current ack semantics:

- same-recipient ack is idempotent at the API layer;
- wrong-recipient ack is rejected;
- ack sets the envelope delivery state to `acknowledged`;
- inbox returns queued envelopes only.

Ack records are server records of a recipient-device ack request. They are not proof that Cypher independently verified sidecar consume.

## Retention direction

Future retention policy may delete acknowledged envelopes or expire old queued envelopes.

That is not the current production behavior.

## Nonclaims

Current storage does not prove:

- production metadata privacy;
- production vault security;
- hostile-server completeness;
- secure identity;
- external audit or certification.
