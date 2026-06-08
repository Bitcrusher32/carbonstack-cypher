# CarbonStackCypher Validation Profile Boundary v0

Status: component-local validation-profile boundary
Component: CarbonStackCypher
Maturity: experimental / pre-release
Related main doctrine: carbonstack/docs/187-v0.5.33-validation-profile-boundary-v0.md

## 1. Purpose

This document records Cypher-side validation profile boundaries before Relay Space schema/API and local-backbone validation work.

This is planning only.

No Cypher schema, API, or validation profile is changed by this document.

## 2. Cypher validation state

Cypher validation may create:

    temporary local DBs;
    temporary server processes;
    test invites;
    test accounts;
    test devices;
    test envelopes;
    acked envelope state;
    future Relay Space routing state.

This state must remain scoped to temp/generated validation roots unless a profile explicitly documents otherwise.

## 3. Cleanup boundary

Validation cleanup may remove:

    runner-owned temp directories;
    temporary Cypher DBs created by the runner;
    generated test artifacts created by a specific validation profile.

Validation cleanup must not remove:

    user DBs;
    unknown DBs;
    production-like DBs;
    non-temp operational state;
    Comms trust.json;
    Comms trust-events.jsonl;
    Comms identity-candidates.json.

## 4. Claim boundary

Cypher validation may prove routing/storage behavior.

Cypher validation must not prove:

    verified identity;
    local trust;
    OpenMLS group safety;
    plaintext safety;
    hostile-server safety;
    metadata privacy;
    production deployment readiness;
    server trustworthiness.

## 5. Future Relay Space profile participation

Future Cypher profiles may later test:

    Relay Space schema migration;
    Relay Space create/get/list;
    invite create/claim;
    routing member register/list;
    Relay Space-scoped envelope submit/retrieve/ack;
    restart persistence for routing state.

These must be described as routing/storage validation, not identity/trust validation.

## 6. Nonclaims

This document does not claim:

    Relay Space schema/API implementation;
    local-backbone validation;
    provider live-flow validation;
    identity authority;
    trust authority;
    production readiness.
