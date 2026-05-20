# MVP Roadmap

CarbonStackCypher should be built in narrow phases.

## Phase 0 — Documentation

- architecture
- envelope model
- API surface
- storage model
- admin plane
- hostile-server model
- deployment model

## Phase 1 — Minimal Relay

Goals:

- start server
- health endpoint
- invite-only registration
- account records
- device records
- encrypted envelope submission
- encrypted envelope retrieval
- delivery acknowledgement
- size limits
- basic rate limits
- no plaintext access

Non-goals:

- group messaging
- attachments
- file storage
- media handling
- federation
- bots
- link previews

## Phase 2 — Hostile-Server Test Harness

Goals:

- simulate dropped messages
- simulate delayed messages
- simulate reordered messages
- simulate replay attempts
- simulate stale key delivery
- simulate revocation suppression
- verify client warnings

## Phase 3 — Admin Plane

Goals:

- hardware-key-friendly admin authentication path
- invite management
- account suspension
- operational logs
- audit logs
- rate-limit configuration

Admin plane must not expose plaintext.

## Phase 4 — Group-Aware Server Support

Goals:

- group delivery fanout
- group epoch routing metadata
- revocation event propagation
- membership event storage
- append-only event research

Group truth remains client/protocol-owned.

## Phase 5 — Deployment Hardening

Goals:

- Docker or container deployment
- reverse proxy guide
- TLS guide
- backup guide
- restore guide
- update guide
- security changelog
- instance lockdown procedure

## Core Principle

Ship the smallest hostile-server relay that lets CarbonStackComms prove the model.
