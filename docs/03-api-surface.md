# CarbonStackCypher API Surface

Status: current development API surface
Component: CarbonStackCypher
Maturity: experimental / pre-release

Cypher exposes a narrow HTTP JSON API for development relay testing.

It is not a stable public protocol.

It is not a production deployment surface.

## Current routes

    GET  /v0/health
    POST /v0/dev/invites
    POST /v0/invites/claim
    POST /v0/devices/register
    GET  /v0/accounts/{account_id}/devices
    POST /v0/envelopes
    GET  /v0/devices/{device_id}/envelopes
    POST /v0/envelopes/{envelope_id}/ack

## Health

Purpose:

- confirm the server is running;
- expose basic service/API version status.

## Development invite/account/device routes

Purpose:

- support local development account setup;
- support test and smoke-harness device registration.

These routes are development scaffolding. They are not production authentication.

## Envelope submit

Route:

    POST /v0/envelopes

Purpose:

- accept an opaque payload for a recipient device;
- validate content type and protocol version compatibility;
- validate base64 payload form;
- compute payload metadata;
- store the envelope as `queued`.

Submit does not prove OpenMLS semantic validity.

Submit does not prove delivery.

## Inbox

Route:

    GET /v0/devices/{device_id}/envelopes

Purpose:

- return queued envelopes for the recipient device.

Current inbox behavior:

    returns delivery_state = queued envelopes only.

It does not return acknowledged envelopes.

Inbox retrieval is not ack.

Inbox retrieval is not sidecar consume.

## Ack

Route:

    POST /v0/envelopes/{envelope_id}/ack

Purpose:

- mark an envelope handled for the recipient device.

Current ack behavior:

- requires `recipient_device_id`;
- rejects unknown envelopes;
- rejects wrong-recipient ack;
- is idempotent for the correct recipient;
- sets or returns `delivery_state = acknowledged`.

Ack is not proof of OpenMLS consume by itself.

In the current CarbonStackComms proof, Comms sends ack only after the relevant sidecar consume command succeeds.

## Nonclaims

This API does not prove:

- production E2EE;
- hostile-server safety;
- metadata privacy;
- secure identity;
- secure local vault/storage;
- Android readiness;
- external audit or certification.
