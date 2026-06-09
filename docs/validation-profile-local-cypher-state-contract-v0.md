# CarbonStackCypher validation-profile local state contract

Status: component validation-profile design contract
Parent: carbonstack/docs/196-v0.5.51-validation-profile-design-contract-v0.md
Scope: local Cypher DB/server lifecycle for future Relay Space + OpenMLS join validation profile
Date: 2026-06-09 local session

## 1. Purpose

This document defines the Cypher-side local server and DB ownership contract for a future narrow validation profile.

It is docs-only.

It does not add routes, schema, runner profile, registry entries, public README promotion, or local-backbone claims.

## 2. Required local server behavior

The future validation profile should start a local Cypher dev server with:

    runner-owned temp DB;
    explicit CYPHER_ADDR;
    explicit CYPHER_DB;
    explicit CYPHER_MIGRATIONS;
    explicit CYPHER_DEV_INVITE;
    health check before any Comms setup.

The profile should prefer a runner-owned temp root over shared /tmp names.

## 3. Port and process boundary

First implementation may use 127.0.0.1:8080 or another explicit local address.

It must refuse if the chosen port is already occupied by an unknown process.

It must not kill arbitrary Cypher processes owned by the user.

Any process cleanup must be limited to the profile-owned process that the profile started.

## 4. DB ownership

The profile-owned DB must be under a runner-owned temp root.

The profile must not use:

    cypher.db in the repo;
    unknown persistent operator DBs;
    user-provided DB paths;
    previous v0.5.49/v0.5.50 smoke DBs.

The profile may remove the profile-owned temp DB only if it created it during the same run.

## 5. Required Cypher setup assertions

The future profile should assert:

    health returns ok;
    two accounts exist;
    two devices exist;
    one Relay Space exists per subrun;
    two Relay Space members exist per subrun;
    two envelopes exist per completed subrun;
    no-ack subrun has zero envelope_acks;
    ACK_AFTER_JOIN subrun has one envelope_acks row;
    no-ack Welcome remains queued;
    ACK_AFTER_JOIN Welcome becomes acknowledged;
    KeyPackage remains queued in both subruns.

## 6. Ack boundary

Cypher ack is delivery/local-processing state.

Cypher must not be treated as:

    identity authority;
    trust root;
    verified membership source;
    metadata privacy proof;
    hostile-server safety proof.

## 7. Nonclaims

Passing future Cypher-backed validation must not claim:

    local-backbone;
    production secure messaging;
    hostile-server safety;
    metadata privacy;
    verified identity;
    secure enrollment;
    deployability;
    audit;
    certification.
