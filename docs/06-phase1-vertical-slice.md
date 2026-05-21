# CarbonStackCypher Phase 1 Vertical Slice

## Status

Classification: PLANNED / NOT IMPLEMENTED

CarbonStackCypher Phase 1 is a local-first relay skeleton for storing and retrieving encrypted-envelope-shaped records.

It is not a production secure server.

## Goal

Build the smallest server that can support a CarbonStackComms CLI lifecycle:

- invite-only registration
- account creation
- device registration
- opaque envelope submission
- recipient envelope retrieval
- envelope acknowledgement

## Server Principle

CarbonStackCypher routes and stores envelopes.

CarbonStackCypher must not require message plaintext.

The server may know operational metadata in Phase 1, including:

- account identifiers
- device identifiers
- sender device identifier
- recipient device identifier
- envelope size
- envelope creation/storage time
- delivery state

This metadata exposure is known and not solved in Phase 1.

## Minimal Entities

- Invite
- Account
- Device
- Envelope
- Delivery acknowledgement

## Phase 1 Flow

1. Operator starts local Cypher server.
2. Operator creates or seeds invite codes.
3. Client A claims invite and registers account/device.
4. Client B claims invite and registers account/device.
5. Client A submits an envelope addressed to Client B device.
6. Cypher stores envelope as opaque payload.
7. Client B polls/retrieves queued envelopes.
8. Client B acknowledges envelope receipt.
9. Cypher marks delivery state.

## Non-Goals

- no groups
- no attachments
- no server-side message parsing
- no plaintext content
- no WebSocket requirement
- no push notifications
- no federation
- no multi-server routing
- no production auth model
- no hardware-key enforcement yet
- no final protocol selection

## Recommended Implementation Stack

Language: Go  
Initial database: SQLite  
Initial API: HTTP JSON  
Initial deployment: local binary  
Later deployment: Docker  

## Trust Boundary

Cypher is treated as hostile by design.

Even in Phase 1, implementation should avoid patterns that require server trust for message contents.

## Phase 1 Security Claim

Allowed:

Cypher can store and route opaque message envelopes.

Not allowed:

Cypher provides production-grade confidentiality, metadata privacy, or protocol-level security.
