# CarbonStackCypher MVP Roadmap

Status: current development roadmap
Component: CarbonStackCypher
Maturity: experimental / pre-release

## Current implemented base

Cypher currently implements the relay/server scaffold needed by the CarbonStack experimental backbone:

- HTTP JSON server;
- SQLite migrations;
- development invite/account/device records;
- opaque envelope submission;
- recipient inbox listing;
- envelope ack;
- OpenMLS artifact content types;
- payload metadata over decoded envelope bytes.

## Current role

Cypher is the relay/storage server.

It does not handle plaintext.

It does not parse MLS internals.

It does not decide trust.

It does not provide production identity or authentication.

## Near-term work

Near-term Cypher work should focus on:

- inbox/ack semantics cleanup;
- schema/API wording standardization;
- stronger development runbooks;
- clearer error contracts;
- release-facing docs alignment with `carbonstack`;
- avoiding production security claims.

## Later work

Later work may include:

- deployment configuration;
- stronger auth;
- PostgreSQL planning;
- hostile-server rollback/replay harnesses;
- metadata minimization;
- operational logging policy;
- production migration strategy.

## Nonclaims

Cypher is not production-certified.

Cypher is not externally audited.

Cypher is not a complete hostile-server-safe messaging server.

Cypher is one component in the current experimental CarbonStack backbone.
