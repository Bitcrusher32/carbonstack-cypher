# CarbonStackCypher Relay Space Join/Invite/Member Boundary v0

Status: component-local Relay Space planning boundary
Component: CarbonStackCypher
Maturity: experimental / pre-release
Related main doctrine: carbonstack/docs/185-v0.5.31-relay-space-join-invite-member-planning-v0.md

## 1. Purpose

This document records what Relay Space join/invite/member mechanics mean for CarbonStackCypher.

This is planning only.

No Cypher schema or API changes are introduced by this document.

## 2. Core rule

Cypher may provide routing membership.

Cypher must not provide verified trust.

Cypher is allowed to know operational routing metadata.

Cypher is not plaintext authority, identity authority, verified-device authority, or silent key-replacement authority.

## 3. Addressing

Future Relay Space implementation should use a full-strength canonical Relay Space identifier:

    UUIDv4;
    128-bit random token;
    or equivalent high-entropy opaque ID.

Short display codes may exist:

    12 to 16 hex characters;
    grouped for readability;
    optionally mapped to word-code phrases.

Short codes are UX/display aids, not canonical security identifiers.

QR codes should carry the full invite package/token/URL rather than only the short display code.

## 4. Invite records

Future Cypher invite records may include:

    invite_id;
    relay_space_id;
    invite_token_hash;
    display_code;
    optional word_code;
    created_at;
    expires_at;
    max_claims;
    claim_count;
    created_by_member_id;
    state;
    constraints.

Invite claim can mean:

    server accepted an invite credential;
    server created or updated routing access;
    server associated account/device/routing member data with a Relay Space.

Invite claim must not mean:

    user verified a device;
    key material is trusted;
    group membership is locally trusted;
    server can replace identity;
    Comms trust.json should mutate.

## 5. Routing member records

Future Cypher routing members may include:

    routing_member_id;
    relay_space_id;
    account_id;
    device_id;
    display label if provided;
    joined_at;
    state;
    last_seen_at if later needed;
    routing/access constraints.

This is server-side routing state only.

It is not verified identity.

## 6. Envelope scoping

Future envelopes should likely be scoped to relay_space_id.

Cypher may route/store:

    envelope_id;
    relay_space_id;
    sender routing/device ID;
    recipient routing/device ID or queue ID;
    content_type;
    protocol_version;
    ciphertext_b64;
    payload_size_bytes;
    payload_sha256;
    created_at;
    ack state.

Payload metadata remains relay/debug/storage sanity metadata, not a trust root.

## 7. API direction

Possible future API families:

    POST /v0/relay-spaces
    GET  /v0/relay-spaces/{relay_space_id}
    POST /v0/relay-spaces/{relay_space_id}/invites
    POST /v0/relay-spaces/invites/claim
    GET  /v0/relay-spaces/{relay_space_id}/members
    POST /v0/relay-spaces/{relay_space_id}/envelopes
    GET  /v0/relay-spaces/{relay_space_id}/devices/{device_id}/envelopes
    POST /v0/relay-spaces/{relay_space_id}/envelopes/{envelope_id}/ack

Exact routes are not implemented by this document.

## 8. Admin/operator boundary

Cypher operators may affect availability, routing access, storage retention, rate limits, and server-side access policy.

They must not be able to:

    decrypt messages;
    mark client identity verified;
    suppress client trust warnings;
    silently replace client keys;
    mutate Comms trust.json;
    complete verification on behalf of users.

## 9. Nonclaims

This document does not claim:

    Relay Space schema;
    Relay Space API;
    Relay Space join implementation;
    production deployment;
    metadata privacy;
    hostile-server safety;
    identity authority;
    verified group membership authority;
    local-backbone readiness.
