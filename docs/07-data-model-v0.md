# CarbonStackCypher Data Model v0

Status: implemented development schema
Component: CarbonStackCypher
Maturity: experimental / pre-release

This document describes the current SQLite data model used by CarbonStackCypher.

The schema is implemented for the current relay/server scaffold. It is not a production database contract. It may change before any stable release.

## Design principles

Cypher stores routing and opaque envelope data.

Cypher does not store plaintext message bodies.

Cypher does not decide trust.

Cypher does not parse OpenMLS internals.

Cypher currently supports SQLite for development and local smoke testing.

## Migrations

Current migrations:

    migrations/001_init.sql
    migrations/002_envelope_payload_metadata.sql

## Tables

### invites

Purpose: allow development invite/account creation without open signup.

Core fields:

- `invite_id`
- `invite_code_hash`
- `created_at`
- `claimed_at`
- `claimed_by_account_id`
- `disabled_at`

Invite codes are not a production authentication system.

### accounts

Purpose: represent an account namespace.

Core fields:

- `account_id`
- `display_name`
- `created_at`
- `disabled_at`

Display names are not trusted identity.

### devices

Purpose: represent client devices under accounts.

Core fields:

- `device_id`
- `account_id`
- `device_label`
- `public_identity_key`
- `public_prekey_bundle`
- `created_at`
- `revoked_at`

Device records are routing/accounting records in the current scaffold. They are not a complete production identity system.

### envelopes

Purpose: store opaque relay envelopes for recipient devices.

Core fields:

- `envelope_id`
- `sender_device_id`
- `recipient_device_id`
- `content_type`
- `protocol_version`
- `ciphertext_b64`
- `payload_sha256`
- `payload_size_bytes`
- `client_created_at`
- `server_received_at`
- `delivery_state`

`ciphertext_b64` stores opaque payload bytes encoded as base64.

For OpenMLS relay artifacts, those bytes are sidecar artifact bytes. Cypher does not parse the MLS content.

`payload_sha256` is the lowercase SHA-256 digest of decoded `ciphertext_b64` bytes.

`payload_size_bytes` is the decoded byte length of `ciphertext_b64`.

Payload metadata is relay/debug/storage sanity metadata. It is not a trust root.

## Delivery state

Current delivery state is minimal:

- `queued`
- acknowledged through the ack route after recipient-side consume succeeds in the current Comms proof.

The current consume-then-ack rule is enforced by Comms tests, not by Cypher semantic knowledge of OpenMLS.

## Security boundary

Cypher is hostile-server-aware in design direction, but the current implementation is not a complete hostile-server proof.

Cypher does not provide:

- plaintext access;
- MLS semantic validation;
- identity trust decisions;
- local vault security;
- production metadata privacy;
- external audit or certification.
