# CarbonStackCypher Envelope Model

Status: current conceptual model
Component: CarbonStackCypher
Maturity: experimental / pre-release

Cypher stores and routes opaque envelopes.

It does not handle plaintext.

It does not parse OpenMLS internals.

It does not decide trust.

## Current envelope shape

Current envelope records carry:

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

`ciphertext_b64` is the stored opaque payload encoded as base64.

For OpenMLS relay artifacts, this name is historical and imperfect. The payload may be a KeyPackage artifact, Welcome artifact, or application-message artifact.

## Content type

Current OpenMLS relay content types:

    carbonstack.mls.keypackage.v0
    carbonstack.mls.welcome.v0
    carbonstack.mls.application-message.v0

Existing stub content type:

    carbonstack.message.text.stub.v0

The content type helps the client and server agree on the envelope family. It does not prove payload authenticity or semantic validity.

## Protocol version

Current OpenMLS relay protocol version:

    carbonstack-openmls-sidecar-v0

Existing stub protocol version:

    stub-v0

The OpenMLS relay protocol version is a CarbonStack compatibility label. It is not a claim of generic OpenMLS standard compatibility.

## Payload metadata

Cypher computes:

    payload_sha256
    payload_size_bytes

Both describe decoded `ciphertext_b64` bytes.

Payload metadata is relay/debug/storage sanity metadata.

It is not a trust root.

A malicious server can lie about server-returned metadata.

OpenMLS sidecar consume remains the cryptographic validity gate.

## Delivery state

Current delivery states:

    queued
    acknowledged

`queued` means the envelope is available through the recipient inbox route.

`acknowledged` means Cypher accepted a recipient-device ack for the envelope.

Cypher does not know whether the sidecar consumed the artifact. Comms decides when to ack.

## Payload boundary

Cypher must not:

- parse plaintext;
- parse MLS internals;
- generate previews;
- scan links;
- scan attachments;
- mutate payload content;
- normalize text;
- decide identity trust.

The server-visible envelope exists for routing and storage, not content authority.
