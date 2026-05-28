# CarbonStackCypher API Contract v0

Status: implemented development API
Component: CarbonStackCypher
Maturity: experimental / pre-release

This document describes the current HTTP JSON API used by CarbonStackCypher.

The API is stable enough for the current CarbonStackComms smoke harness. It is not a stable public protocol.

## API principles

Cypher should remain dumb.

It stores opaque envelopes.

It does not parse plaintext.

It does not parse OpenMLS internals.

It does not decide trust.

It exposes boring HTTP JSON for early testing and dev harnesses.

## Base path

    /v0

## Health

    GET /v0/health

Purpose: verify the server is running.

Response shape:

    {
      "status": "ok",
      "service": "carbonstack-cypher",
      "api_version": "v0"
    }

## Invite claim

    POST /v0/invites/claim

Purpose: claim a development invite and create an account.

This is development scaffolding, not a production authentication system.

## Device registration

    POST /v0/devices/register

Purpose: register a device under an account.

Device records are routing/accounting records in the current scaffold. They are not a full production identity system.

## Device lookup

    GET /v0/accounts/{account_id}/devices

Purpose: list non-revoked devices for an account.

## Envelope submit

    POST /v0/envelopes

Purpose: submit an opaque envelope for a recipient device.

Core request fields:

- `sender_device_id`
- `recipient_device_id`
- `content_type`
- `protocol_version`
- `ciphertext_b64`
- `client_created_at`

Current accepted OpenMLS content types:

    carbonstack.mls.keypackage.v0
    carbonstack.mls.welcome.v0
    carbonstack.mls.application-message.v0

Current OpenMLS protocol version:

    carbonstack-openmls-sidecar-v0

Existing stub content type:

    carbonstack.message.text.stub.v0

Existing stub protocol version:

    stub-v0

Submit response includes:

- `envelope_id`
- `delivery_state`
- `server_received_at`
- `payload_sha256`
- `payload_size_bytes`

`payload_sha256` and `payload_size_bytes` are computed by the server from decoded `ciphertext_b64` bytes.

## Inbox list

    GET /v0/devices/{device_id}/envelopes

Purpose: list queued envelopes for a recipient device.

Envelope records include:

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

## Ack

    POST /v0/envelopes/{envelope_id}/ack

Purpose: mark an envelope handled.

In the current Comms proof, ack occurs only after recipient-side OpenMLS sidecar consume succeeds.

Cypher itself does not know OpenMLS consume state. It only records the ack request.

## Security boundary

This API does not prove:

- production E2EE;
- hostile-server safety;
- metadata privacy;
- secure identity;
- secure local vault/storage;
- stable public protocol status;
- external audit or certification.
