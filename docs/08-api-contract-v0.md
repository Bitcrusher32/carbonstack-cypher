# CarbonStackCypher API Contract v0

## Status

Classification: PLANNED / NOT IMPLEMENTED

This document defines the initial HTTP JSON API for the Phase 1 relay skeleton.

## API Principles

- Keep the server dumb.
- Treat envelope payloads as opaque.
- Avoid plaintext message content.
- Avoid group semantics in v0.
- Prefer boring HTTP JSON for early testing.
- Make CLI integration easy.

## Base Path

/v0

## Health

### GET /v0/health

Response:

```json
{
  "status": "ok",
  "service": "carbonstack-cypher",
  "api_version": "v0"
}
Invite Claim
POST /v0/invites/claim

Purpose: claim an invite and create an account.

Request:

{
  "invite_code": "example-invite",
  "display_name": "alice"
}

Response:

{
  "account_id": "uuid",
  "created_at": "2026-05-21T00:00:00Z"
}

Notes:

Phase 1 may return simple development auth material.
Production auth must be redesigned before security claims.
Device Registration
POST /v0/devices/register

Purpose: register a device under an account.

Request:

{
  "account_id": "uuid",
  "device_label": "alice-cli-1",
  "public_identity_key": "stub-public-identity-key",
  "public_prekey_bundle": "stub-prekey-bundle"
}

Response:

{
  "device_id": "uuid",
  "account_id": "uuid",
  "created_at": "2026-05-21T00:00:00Z"
}
Device Lookup
GET /v0/accounts/{account_id}/devices

Purpose: list non-revoked devices for an account.

Response:

{
  "account_id": "uuid",
  "devices": [
    {
      "device_id": "uuid",
      "device_label": "alice-cli-1",
      "public_identity_key": "stub-public-identity-key",
      "public_prekey_bundle": "stub-prekey-bundle",
      "created_at": "2026-05-21T00:00:00Z"
    }
  ]
}
Submit Envelope
POST /v0/envelopes

Purpose: submit an opaque envelope for delivery.

Request:

{
  "sender_device_id": "uuid",
  "recipient_device_id": "uuid",
  "content_type": "carbonstack.message.text.stub.v0",
  "protocol_version": "stub-v0",
  "ciphertext_b64": "YmFzZTY0LXN0dWItY2lwaGVydGV4dA==",
  "client_created_at": "2026-05-21T00:00:00Z"
}

Response:

{
  "envelope_id": "uuid",
  "delivery_state": "queued",
  "server_received_at": "2026-05-21T00:00:00Z"
}

Server requirements:

Do not parse ciphertext.
Do not require plaintext.
Validate size limits.
Validate required routing fields.
Reject unknown content_type values unless explicitly allowed.
Retrieve Envelopes
GET /v0/devices/{device_id}/envelopes

Purpose: retrieve queued envelopes for a recipient device.

Response:

{
  "device_id": "uuid",
  "envelopes": [
    {
      "envelope_id": "uuid",
      "sender_device_id": "uuid",
      "recipient_device_id": "uuid",
      "content_type": "carbonstack.message.text.stub.v0",
      "protocol_version": "stub-v0",
      "ciphertext_b64": "YmFzZTY0LXN0dWItY2lwaGVydGV4dA==",
      "client_created_at": "2026-05-21T00:00:00Z",
      "server_received_at": "2026-05-21T00:00:00Z",
      "delivery_state": "queued"
    }
  ]
}
Acknowledge Envelope
POST /v0/envelopes/{envelope_id}/ack

Purpose: mark an envelope as acknowledged by recipient device.

Request:

{
  "recipient_device_id": "uuid"
}

Response:

{
  "envelope_id": "uuid",
  "delivery_state": "acknowledged",
  "acknowledged_at": "2026-05-21T00:00:00Z"
}
Error Shape

All errors should use:

{
  "error": {
    "code": "invalid_request",
    "message": "Human-readable development message"
  }
}
Known Security Limitations
No production authentication yet.
No final cryptographic protocol.
No replay protection validated.
No hostile-server proof beyond envelope opacity.
No metadata privacy.
No rate limiting yet.
No abuse resistance yet.
