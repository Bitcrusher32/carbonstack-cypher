CREATE TABLE IF NOT EXISTS keypackage_publications (
    envelope_id TEXT PRIMARY KEY,
    sender_device_id TEXT NOT NULL,
    key_package_ref TEXT NOT NULL,
    payload_sha256 TEXT NOT NULL,
    relay_space_id TEXT NOT NULL,
    recipient_device_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY(envelope_id) REFERENCES envelopes(envelope_id),
    FOREIGN KEY(sender_device_id) REFERENCES devices(device_id),
    FOREIGN KEY(relay_space_id) REFERENCES relay_spaces(relay_space_id),
    FOREIGN KEY(recipient_device_id) REFERENCES devices(device_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_keypackage_publications_sender_ref
    ON keypackage_publications(sender_device_id, key_package_ref);

CREATE UNIQUE INDEX IF NOT EXISTS idx_keypackage_publications_sender_payload
    ON keypackage_publications(sender_device_id, payload_sha256);

CREATE INDEX IF NOT EXISTS idx_keypackage_publications_destination
    ON keypackage_publications(relay_space_id, recipient_device_id);
