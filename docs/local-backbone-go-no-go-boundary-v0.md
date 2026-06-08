# CarbonStackCypher Local-Backbone Go/No-Go Boundary v0

Status: component-local go/no-go boundary
Component: CarbonStackCypher
Maturity: experimental / pre-release
Related main doctrine: carbonstack/docs/188-v0.5.34-local-backbone-go-no-go-v0.md

## 1. Purpose

This document records Cypher-side implications of the v0.5.34 local-backbone go/no-go reassessment.

This is planning only.

No Cypher schema or API is changed by this document.

## 2. Decision

Local-backbone receives a conditional GO for first narrow implementation planning.

Preferred first implementation target:

    Cypher Relay Space schema/API substrate.

This is not full local-backbone.

## 3. Why Cypher first

Cypher is the right first substrate because:

    Relay Space needs routing/storage primitives;
    provider live-flow needs a server/conversation anchor;
    Comms client wrappers need an API to wrap;
    validation profiles need concrete substrate before they can make honest claims.

## 4. Possible future Cypher implementation surfaces

Future narrow rungs may add:

    relay_spaces table;
    relay_space_invites table;
    relay_space_members table;
    relay_space_id-scoped envelope support or migration path;
    create/get/list Relay Space APIs;
    invite create/claim APIs;
    routing member registration/list APIs;
    tests proving routing-only semantics.

These should be split if needed.

## 5. Cypher authority boundary

Cypher may provide routing/storage.

Cypher must not provide:

    verified identity;
    local trust state;
    plaintext authority;
    local verification result;
    key replacement authority;
    trust.json authority;
    identity-candidates.json authority;
    provider safety truth.

## 6. Validation and cleanup boundary

Future Cypher validation must use explicit generated/test state.

It must not delete or mutate:

    user DBs;
    unknown DBs;
    Comms trust.json;
    Comms trust-events.jsonl;
    Comms identity-candidates.json;
    provider identity state.

## 7. Implementation continuity

Before implementation after this planning arc, use the latest LogDoc as a direct continuity anchor and scout relevant planning docs again.

## 8. Nonclaims

This document does not claim:

    Relay Space schema/API implementation;
    local-backbone implementation;
    provider live-flow;
    local trust;
    identity authority;
    CLI/registry exposure;
    production readiness.
