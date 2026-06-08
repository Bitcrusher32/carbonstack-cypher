CREATE TABLE IF NOT EXISTS relay_spaces (
    relay_space_id TEXT PRIMARY KEY,
    display_label TEXT NOT NULL DEFAULT '',
    created_by_account_id TEXT,
    created_by_device_id TEXT,
    created_at TEXT NOT NULL,
    disabled_at TEXT,
    FOREIGN KEY(created_by_account_id) REFERENCES accounts(account_id),
    FOREIGN KEY(created_by_device_id) REFERENCES devices(device_id)
);

CREATE TABLE IF NOT EXISTS relay_space_members (
    routing_member_id TEXT PRIMARY KEY,
    relay_space_id TEXT NOT NULL,
    account_id TEXT NOT NULL,
    device_id TEXT,
    display_label TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    joined_at TEXT NOT NULL,
    last_seen_at TEXT,
    disabled_at TEXT,
    FOREIGN KEY(relay_space_id) REFERENCES relay_spaces(relay_space_id),
    FOREIGN KEY(account_id) REFERENCES accounts(account_id),
    FOREIGN KEY(device_id) REFERENCES devices(device_id),
    CHECK(state IN ('active', 'disabled', 'left'))
);

CREATE TABLE IF NOT EXISTS relay_space_invites (
    relay_space_invite_id TEXT PRIMARY KEY,
    relay_space_id TEXT NOT NULL,
    invite_token_hash TEXT NOT NULL UNIQUE,
    display_code TEXT NOT NULL,
    word_code TEXT,
    created_by_member_id TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT,
    max_claims INTEGER,
    claim_count INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL,
    note TEXT,
    FOREIGN KEY(relay_space_id) REFERENCES relay_spaces(relay_space_id),
    FOREIGN KEY(created_by_member_id) REFERENCES relay_space_members(routing_member_id),
    CHECK(state IN ('active', 'claimed', 'disabled', 'expired')),
    CHECK(max_claims IS NULL OR max_claims >= 1),
    CHECK(claim_count >= 0)
);

ALTER TABLE envelopes ADD COLUMN relay_space_id TEXT REFERENCES relay_spaces(relay_space_id);

CREATE INDEX IF NOT EXISTS idx_relay_spaces_created_by_account_id
    ON relay_spaces(created_by_account_id);

CREATE INDEX IF NOT EXISTS idx_relay_space_members_relay_space_id
    ON relay_space_members(relay_space_id);

CREATE INDEX IF NOT EXISTS idx_relay_space_members_account_id
    ON relay_space_members(account_id);

CREATE INDEX IF NOT EXISTS idx_relay_space_members_device_id
    ON relay_space_members(device_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_relay_space_members_unique_device
    ON relay_space_members(relay_space_id, device_id)
    WHERE device_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_relay_space_invites_relay_space_id
    ON relay_space_invites(relay_space_id);

CREATE INDEX IF NOT EXISTS idx_relay_space_invites_created_by_member_id
    ON relay_space_invites(created_by_member_id);

CREATE INDEX IF NOT EXISTS idx_envelopes_relay_space_id
    ON envelopes(relay_space_id);
