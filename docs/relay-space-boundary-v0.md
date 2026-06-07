# CarbonStackCypher Relay Space Boundary v0

Status: component-local architecture note
Component: CarbonStackCypher
Maturity: experimental / pre-release
Related main doctrine: carbonstack/docs/178-v0.5.16-relay-space-architecture-decision-v0.md

## 1. Purpose

This document records what Relay Space means for CarbonStackCypher.

This is planning only.

No Cypher schema or API changes are introduced by this document.

## 2. Definition

A Relay Space is an addressable relay/conversation space.

For Cypher, it may eventually mean:

    routing namespace;
    encrypted envelope storage;
    invite creation/claim surface;
    device registration and lookup;
    delivery queues;
    rate limits;
    operational configuration;
    future server-side access moderation.

## 3. Boundary

Cypher is routing/storage infrastructure.

Cypher is not identity authority.

Cypher must not become:

    plaintext authority;
    verified device authority;
    verified group membership authority;
    safety-number authority;
    trust-store authority;
    silent key replacement authority.

## 4. What Cypher may know

Cypher may know operational metadata such as:

    Relay Space identifier;
    route/delivery queues;
    envelope size;
    timing;
    server-side device registration metadata;
    invite claim metadata;
    access/rate-limit metadata.

This does not mean CarbonStack has metadata privacy.

## 5. Invite boundary

A Cypher invite may authorize routing access.

It must not authorize verified trust.

Invite claim can mean:

    server accepted a join credential;
    server created or updated routing access;
    server associated a device/account claim with a Relay Space.

Invite claim must not mean:

    user verified a device;
    key material is trusted;
    group membership is locally trusted;
    server can replace identity.

## 6. Admin boundary

Cypher operators may affect availability and routing policy.

They must not be able to:

    decrypt messages;
    mark client identity verified;
    suppress client trust warnings;
    silently replace client keys;
    mutate Comms trust.json;
    complete verification on behalf of users.

## 7. Future implementation warning

Do not implement Relay Space schema/API changes until Comms-side candidate identity, mapped mismatch, and local-backbone feasibility decisions are stable enough.

Current Cypher behavior remains pre-Relay-Space skeleton behavior.
