# API Surface

CarbonStackCypher should expose a narrow API surface.

The API should support encrypted envelope routing, device registration, invite flows, revocation propagation, and administrative instance management.

It should not expose broad content, media, preview, or filesystem functionality.

## MVP API Categories

### Health

Purpose:

- service health
- version reporting
- basic compatibility checks

Example operations:

- get server status
- get supported protocol versions
- get instance policy

### Registration

Purpose:

- invite-only account creation
- device enrollment

Example operations:

- redeem invite
- register account
- register first device
- publish device public material
- fetch own device state

### Device Directory

Purpose:

- allow clients to retrieve public device material needed for session setup

Example operations:

- fetch device public bundle
- fetch contact device list
- fetch revocation status
- fetch key-change state where supported

The server must not be trusted as the final authority for identity.

### Envelope Submission

Purpose:

- submit encrypted message envelopes

Example operations:

- submit encrypted envelope
- submit delivery acknowledgement
- submit revocation event
- submit trust event where protocol permits

### Envelope Retrieval

Purpose:

- retrieve queued encrypted envelopes

Example operations:

- fetch pending envelopes
- acknowledge envelope receipt
- delete delivered envelope if policy allows
- retrieve delivery state

### Admin

Purpose:

- instance management

Example operations:

- create invite
- revoke invite
- suspend account
- view operational metrics
- view audit logs
- configure rate limits
- rotate server operational secrets

Admin operations must not expose plaintext or client private keys.

## Explicitly Excluded API Categories

CarbonStackCypher should not expose:

- link preview API
- media transcoding API
- file conversion API
- attachment scanning API
- browser rendering API
- cloud backup plaintext API
- password-based message recovery API
- arbitrary plugin API
- arbitrary webhook execution
- general bot framework

## API Security Requirements

The API should use:

- TLS
- authenticated client requests
- rate limiting
- narrow request schemas
- strict size limits
- structured audit logs
- clear error categories
- safe failure defaults

## Error Behavior

Errors should not leak sensitive state unnecessarily.

The API should avoid revealing:

- whether a specific person is registered, where possible
- detailed trust state to unauthorized clients
- operational internals
- database structure
- secret material

## Core Principle

The API should be boring, narrow, explicit, and hostile to feature creep.
