CREATE TABLE IF NOT EXISTS invites (
    invite_id TEXT PRIMARY KEY,
    invite_code_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    claimed_at TEXT,
    claimed_by_account_id TEXT,
    disabled_at TEXT
);

CREATE TABLE IF NOT EXISTS accounts (
    account_id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    created_at TEXT NOT NULL,
    disabled_at TEXT
);

CREATE TABLE IF NOT EXISTS devices (
    device_id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    device_label TEXT NOT NULL,
    public_identity_key TEXT NOT NULL,
    public_prekey_bundle TEXT,
    created_at TEXT NOT NULL,
    revoked_at TEXT,
    FOREIGN KEY(account_id) REFERENCES accounts(account_id)
);

CREATE TABLE IF NOT EXISTS envelopes (
    envelope_id TEXT PRIMARY KEY,
    sender_device_id TEXT NOT NULL,
    recipient_device_id TEXT NOT NULL,
    content_type TEXT NOT NULL,
    protocol_version TEXT NOT NULL,
    ciphertext_b64 TEXT NOT NULL,
    client_created_at TEXT,
    server_received_at TEXT NOT NULL,
    delivery_state TEXT NOT NULL,
    FOREIGN KEY(sender_device_id) REFERENCES devices(device_id),
    FOREIGN KEY(recipient_device_id) REFERENCES devices(device_id)
);

CREATE TABLE IF NOT EXISTS envelope_acks (
    ack_id TEXT PRIMARY KEY,
    envelope_id TEXT NOT NULL,
    recipient_device_id TEXT NOT NULL,
    acknowledged_at TEXT NOT NULL,
    FOREIGN KEY(envelope_id) REFERENCES envelopes(envelope_id),
    FOREIGN KEY(recipient_device_id) REFERENCES devices(device_id)
);
