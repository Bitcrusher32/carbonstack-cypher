# Storage Model

CarbonStackCypher storage exists to route encrypted envelopes and operate the instance.

It must not become a plaintext archive, identity oracle, or hidden recovery service.

## MVP Storage Objects

### Account Record

May contain:

- account identifier
- registration state
- invite reference
- account status
- creation time
- basic abuse-control metadata

Must not contain:

- message plaintext
- user private keys
- local vault recovery secrets

### Device Record

May contain:

- device identifier
- account identifier
- public identity material
- public prekey material where protocol requires it
- device status
- revocation state
- last seen time
- registration time

Must not contain:

- device private keys
- local vault keys
- message plaintext

### Envelope Record

May contain:

- envelope identifier
- routing metadata
- protocol version
- message type
- ciphertext
- delivery state
- server timestamps
- expiry policy

Must not contain:

- plaintext body
- plaintext subject
- plaintext attachment
- rendered preview

### Invite Record

May contain:

- invite identifier
- invite status
- creation time
- expiry time
- redemption state
- admin creator reference

### Audit Log Record

May contain:

- admin action
- timestamp
- actor
- target object
- result
- reason code

Audit logs should avoid storing sensitive plaintext.

## Retention

CarbonStackCypher should support narrow retention policies.

Possible retention behavior:

- delete delivered envelopes after acknowledgement
- expire undelivered envelopes after configured time
- retain audit logs separately
- avoid indefinite message queue storage by default

## Database Dump Assumption

A database dump should reveal operational metadata but not message contents.

CarbonStack does not initially claim strong metadata privacy.

## Backups

Server backups must be treated as sensitive.

Backups should not include plaintext messages because the server should never have them.

Backups may include:

- encrypted envelopes
- account metadata
- device public material
- invites
- audit logs
- instance configuration

Backups must not include:

- client private keys
- user vault keys
- message plaintext

## Core Principle

Server storage should be useful for delivery and useless for reading.
