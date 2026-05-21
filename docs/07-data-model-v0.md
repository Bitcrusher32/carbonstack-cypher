# CarbonStackCypher Data Model v0

## Status

Classification: PLANNED / NOT IMPLEMENTED

This document defines the initial local/server-side data model for the Phase 1 relay skeleton.

## Design Principles

- Store opaque envelopes.
- Do not store plaintext message bodies.
- Keep schema simple enough for SQLite.
- Use stable identifiers.
- Avoid group complexity in v0.
- Keep future migration to PostgreSQL possible.

## Tables

### invites

Purpose: allow private registration without open signup.

Fields:

- invite_id: TEXT PRIMARY KEY
- invite_code_hash: TEXT NOT NULL
- created_at: TEXT NOT NULL
- claimed_at: TEXT NULL
- claimed_by_account_id: TEXT NULL
- disabled_at: TEXT NULL

Notes:

- Store invite code hashes, not raw invite codes.
- Phase 1 may use a simple hash scheme for development.
- Production password/invite hashing must be revisited before security claims.

### accounts

Purpose: represent a user/account namespace.

Fields:

- account_id: TEXT PRIMARY KEY
- display_name: TEXT NOT NULL
- created_at: TEXT NOT NULL
- disabled_at: TEXT NULL

Notes:

- Display names are not trusted identity.
- Account IDs are routing/accounting identifiers, not cryptographic identity.

### devices

Purpose: represent client device identities.

Fields:

- device_id: TEXT PRIMARY KEY
- account_id: TEXT NOT NULL
- device_label: TEXT NOT NULL
- public_identity_key: TEXT NOT NULL
- public_prekey_bundle: TEXT NULL
- created_at: TEXT NOT NULL
- revoked_at: TEXT NULL

Foreign keys:

- account_id -> accounts.account_id

Notes:

- public_identity_key and public_prekey_bundle may be stub values in Phase 1.
- Device key changes must become loud in later protocol phases.

### envelopes

Purpose: store opaque message envelopes.

Fields:

- envelope_id: TEXT PRIMARY KEY
- sender_device_id: TEXT NOT NULL
- recipient_device_id: TEXT NOT NULL
- content_type: TEXT NOT NULL
- protocol_version: TEXT NOT NULL
- ciphertext_b64: TEXT NOT NULL
- client_created_at: TEXT NULL
- server_received_at: TEXT NOT NULL
- delivery_state: TEXT NOT NULL

Foreign keys:

- sender_device_id -> devices.device_id
- recipient_device_id -> devices.device_id

Allowed delivery_state values:

- queued
- delivered
- acknowledged
- expired

Notes:

- ciphertext_b64 is opaque to the server.
- content_type describes envelope category, not plaintext contents.
- Phase 1 content_type should stay narrow.

### envelope_acks

Purpose: record receipt acknowledgement.

Fields:

- ack_id: TEXT PRIMARY KEY
- envelope_id: TEXT NOT NULL
- recipient_device_id: TEXT NOT NULL
- acknowledged_at: TEXT NOT NULL

Foreign keys:

- envelope_id -> envelopes.envelope_id
- recipient_device_id -> devices.device_id

## Initial Content Types

- carbonstack.message.text.stub.v0

## Initial Protocol Versions

- stub-v0

## Known Limitations

- Metadata privacy is not solved.
- Auth model is incomplete.
- No message expiration policy yet.
- No group state.
- No revocation propagation.
- No replay resistance validated.
- No cryptographic protocol validated.
