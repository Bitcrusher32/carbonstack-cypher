# Cypher provider/OpenMLS join wiring boundary

Status: component boundary / planning-only
Parent: carbonstack/docs/190-v0.5.42-provider-openmls-join-wiring-plan-v0.md
Scope: Cypher route/store implications for future provider/OpenMLS join wiring
Date: 2026-06-08 local session

## 1. Purpose

Cypher supports future provider/OpenMLS join wiring only as a routing/storage server.

Cypher does not verify identity.

Cypher does not interpret OpenMLS trust.

Cypher does not decide provider membership safety.

## 2. Current Cypher substrate

Cypher currently provides:

    Relay Space schema;
    Relay Space DB helpers;
    Relay Space HTTP routes;
    Relay Space-scoped envelope submit;
    Relay Space-scoped recipient inbox;
    Relay Space-scoped ack.

These are routing primitives.

## 3. Future join use

Future Comms join wiring may use Cypher to route:

    KeyPackage artifacts;
    Welcome artifacts;
    later OpenMLS application-message artifacts.

Cypher should treat these as opaque ciphertext/artifact envelopes with content_type, protocol_version, payload_sha256, and payload_size_bytes metadata.

## 4. Boundary

Cypher must not:

    mark devices verified;
    infer local trust;
    infer provider trust;
    replace keys;
    mutate candidate state;
    treat Relay Space membership as identity;
    treat invite possession as identity;
    expose local-backbone claims.

## 5. Ack semantics

Cypher can store ack state.

Cypher cannot know whether sidecar consume/open/join was actually safe.

Therefore Comms must enforce the rule that a scoped ack is sent only after successful local sidecar processing.

## 6. Nonclaims

This doc does not claim:

    provider live-flow;
    OpenMLS join automation;
    local-backbone;
    verified identity import;
    trust mutation;
    metadata privacy;
    hostile-server safety;
    production readiness.
