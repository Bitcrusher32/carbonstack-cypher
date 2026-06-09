# CarbonStackCypher docs

This directory contains component-local CarbonStackCypher design, planning, and result notes.

The main CarbonStack repository remains the source of truth for public release framing, roadmap state, release packages, and cross-repo validation.

## How to read these docs

These docs are historical and component-local.

Older notes may be stale.

Use them for:

    schema/API rationale;
    Relay Space routing boundaries;
    local operator context;
    validation-profile boundaries;
    debugging why current tests or helper routes exist.

Do not treat older notes as current public release claims.

## Current component state

CarbonStackCypher is dev/pre-alpha.

It currently includes:

    invite/account/device development surfaces;
    opaque envelope submit/inbox/ack;
    payload metadata over decoded envelope payload bytes;
    Relay Space schema/API substrate;
    Relay Space-scoped envelope submit/inbox/ack;
    wrong Relay Space / wrong recipient ack rejection tests.

It does not provide:

    plaintext authority;
    identity authority;
    trust authority;
    verified device status;
    secure vault/key storage;
    production deployment safety;
    hostile-server safety proof;
    metadata privacy proof;
    audit or certification.

## Current important boundaries

Relay Space is routing/conversation infrastructure, not identity authority.

Cypher delivery is not trust.

Ack is local-processing/delivery state, not identity verification.

Server membership claims must not mutate local trust.

## Current validation shape

From this repo:

    go test ./... -count=1

Cross-repo validation lives in the main CarbonStack runner:

    carbonstack/tools/carbonstack-validate

Use release-specific runbooks for release-package validation.
