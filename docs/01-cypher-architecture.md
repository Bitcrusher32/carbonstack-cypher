# CarbonStackCypher Architecture

CarbonStackCypher is the hostile-server relay and storage stack for CarbonStack.

Its job is routing and storage, not trust.

CarbonStackCypher must be designed as if it may be compromised, malicious, misconfigured, legally pressured, or operated by an untrusted party.

## Core Responsibilities

CarbonStackCypher is responsible for:

- accepting encrypted envelopes
- storing encrypted envelopes
- routing encrypted envelopes
- supporting delivery queues
- enforcing basic rate limits
- supporting invite-only registration
- propagating revocation events
- supporting small private communities first
- exposing a narrow admin plane
- producing operational audit logs

## Explicit Non-Responsibilities

CarbonStackCypher must not be responsible for:

- plaintext message handling
- user private key storage
- trusted identity replacement
- trusted group membership truth
- trusted message history truth
- client local vault recovery
- content scanning
- link preview generation
- attachment processing
- media processing

## Hostile Server Assumption

Clients must assume CarbonStackCypher may attempt to:

- drop messages
- delay messages
- reorder messages
- replay messages
- roll back visible state
- lie about available messages
- lie about device state
- provide stale keys
- provide incorrect keys
- hide revocation events
- selectively deliver messages
- attempt group membership confusion

CarbonStackCypher must be architected so these attacks are either impossible, detectable by clients, or documented as limitations.

## MVP Scope

The first server MVP should support:

- user records
- device records
- invite-only registration
- encrypted envelope submission
- encrypted envelope retrieval
- basic delivery queues
- basic rate limiting
- admin audit logging
- no plaintext message visibility
- no attachments
- no media handling
- no general file storage

## Future Scope

Future versions may support:

- group delivery
- group epochs
- append-only membership logs
- device revocation propagation
- hardware-key-signed enrollment
- hardware-key-signed revocation
- private invite flows
- server compromise banners
- federation research, if ever justified

Federation is not an MVP goal.

## Admin Plane

The admin plane should manage instance operation only.

Allowed admin functions may include:

- invite creation
- instance configuration
- rate limit settings
- user suspension
- operational metrics
- service health
- relay maintenance
- audit log review

Admin functions must not include:

- plaintext message reading
- private key export
- silent identity replacement
- silent group membership modification
- silent revocation suppression
- user vault recovery

## Storage Principle

Stored message data should be encrypted envelopes.

The server database should be useless for plaintext recovery.

A database dump should not reveal message contents.

## Core Principle

CarbonStackCypher is infrastructure, not authority.
