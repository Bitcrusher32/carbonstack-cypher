# Envelope Model

CarbonStackCypher stores and routes encrypted envelopes.

The envelope model defines what the server can see, what it must not see, and what clients must validate.

## Envelope Goals

Encrypted envelopes should support:

- routing
- delivery
- replay detection support
- version negotiation
- future group support
- future revocation propagation
- hostile-server testing

Encrypted envelopes should avoid:

- plaintext message content
- rich content processing
- server-parsed message bodies
- server-generated previews
- server-trusted identity changes

## MVP Envelope Fields

A minimal server-visible envelope may include:

- envelope_id
- protocol_version
- sender_account_id or sender_device_id
- recipient_account_id or recipient_device_id
- conversation_id
- message_type
- created_at_server
- received_at_server
- delivery_state
- ciphertext
- ciphertext_length
- client_supplied_nonce_or_message_id where protocol requires it

The exact field names are future implementation details.

## Future Group-Aware Fields

Future group support may require:

- group_id
- group_epoch
- sender_device_id
- membership_event_reference
- revocation_event_reference
- sequence or ordering hint
- server-visible delivery fanout state

Group metadata should be minimized where possible.

The server must not become the authority on group truth.

## Payload Boundary

CarbonStackCypher must treat ciphertext as opaque.

It must not:

- parse plaintext
- inspect message bodies
- generate previews
- scan links
- scan attachments
- mutate payload content
- normalize text
- validate user text
- render content

Text validation belongs to CarbonStackComms.

## Server-Side Validation

CarbonStackCypher may validate:

- envelope size limits
- required routing fields
- known protocol version ranges
- rate limits
- account/device existence
- invite or registration state
- abuse-control metadata

CarbonStackCypher must not require plaintext access to perform validation.

## Message Types

Potential server-visible message types:

- encrypted_text
- trust_event
- device_revocation
- delivery_ack
- group_epoch_event
- protocol_control

Message type visibility should be minimized and revisited during protocol design.

## Rejection Behavior

The server may reject envelopes that are:

- oversized
- malformed at envelope layer
- from suspended accounts
- from revoked devices
- above rate limits
- using unsupported protocol versions

The server should not reject based on plaintext content because it must not know plaintext content.

## Core Principle

The server routes envelopes.

The client owns meaning.
