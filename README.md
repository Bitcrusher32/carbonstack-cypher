# CarbonStackCypher

CarbonStackCypher is the experimental relay/storage server for CarbonStack.

It stores opaque envelopes.

It is not externally audited.

It does not handle plaintext.

It does not decide trust.

It is not production-certified.

It is not identity authority.

## Source of truth

Use the main CarbonStack repository for public release framing, release assets, validation runbooks, roadmap state, and project-wide claim boundaries:

    https://git.bitcrusher32.win/bitcrusher32/carbonstack

Related repositories:

    https://git.bitcrusher32.win/bitcrusher32/carbonstack
    https://git.bitcrusher32.win/bitcrusher32/carbonstack-comms
    https://git.bitcrusher32.win/bitcrusher32/carbonstack-os

Gitea remains source of truth. GitHub mirrors may exist but are not release authority unless project policy changes.

## Current implemented behavior

Cypher currently provides:

    health check;
    development invite/account/device registration;
    account device lookup;
    envelope submission;
    recipient inbox listing;
    envelope acknowledgement;
    OpenMLS artifact content-type support;
    payload metadata over decoded envelope payload bytes;
    Relay Space schema/API substrate;
    Relay Space member/invite routing surfaces;
    Relay Space-scoped envelope submit/inbox/ack routes;
    scoped ack rejection for wrong Relay Space and wrong recipient.

Current OpenMLS relay content types include:

    carbonstack.mls.keypackage.v0
    carbonstack.mls.welcome.v0
    carbonstack.mls.application-message.v0

Current OpenMLS relay protocol version:

    carbonstack-openmls-sidecar-v0

Existing stub content type:

    carbonstack.message.text.stub.v0

## Relay Space boundary

Relay Space is routing/conversation infrastructure.

Relay Space is not identity authority.

Cypher may route encrypted envelopes and manage server-side access, but must not become plaintext authority, verified-device authority, trust authority, or silent key-replacement authority.

Server membership claims are not enough for client trust.

Local Comms trust remains client-owned.

## Payload model

Envelope payloads are stored as:

    ciphertext_b64

For OpenMLS relay artifacts, this field carries opaque artifact bytes encoded as base64.

Cypher computes:

    payload_size_bytes
    payload_sha256

Both describe decoded ciphertext_b64 bytes.

These fields are relay/debug/storage sanity metadata. They are not a cryptographic trust root.

## Routes

Current route families include:

    GET  /v0/health
    POST /v0/invites/claim
    POST /v0/devices/register
    GET  /v0/accounts/{account_id}/devices
    POST /v0/envelopes
    GET  /v0/devices/{device_id}/envelopes
    POST /v0/envelopes/{envelope_id}/ack

Relay Space routes include create/list/get/member/invite/scoped envelope behavior. See current tests and docs for exact route contracts.

## Database

Cypher uses SQLite for current development and smoke testing.

Current schema is pre-release and may change before any production claim.

## Known-good validation

Run from this repository:

    go test ./... -count=1

For cross-repo validation, use the main CarbonStack runner:

    cd ~/repos/carbonstack_umbrella/carbonstack/tools/carbonstack-validate
    go test ./... -count=1
    go run . --profile doctor

Use release-specific runbooks for release-package validation.

## Local operator note

Use explicit local-only settings for development/operator experiments.

Current work remains pre-alpha local deployability work, not a production deployment guide.

## Docs

Component docs live under:

    docs/

Start with:

    docs/README.md

The main CarbonStack repo remains the public release and roadmap authority.

## What Cypher does not prove

Cypher does not currently prove:

    production E2EE;
    hostile-server safety;
    metadata privacy;
    secure identity;
    secure local vault/storage;
    verified trust;
    rollback/replay safety against a malicious server;
    multi-user production operations;
    public ingress safety;
    external audit or certification.

Cypher is a relay/storage component inside the current experimental backbone.

License: MIT.
See the repository's LICENSE file for more information.
