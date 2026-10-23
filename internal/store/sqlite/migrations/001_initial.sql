CREATE TABLE admin (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

CREATE TABLE session (
    session_key TEXT PRIMARY KEY,
    value BLOB,
    expires_at TEXT NOT NULL
) STRICT;

-- The app labels each family as "Family <id>" to keep recipients anonymous.
-- AUTOINCREMENT keeps SQLite from reusing a deleted family's ID, which would
-- hand its public label and URL to a different family.
CREATE TABLE family (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

-- Columns mirror the paper intake sheet that admins transcribe. Optional
-- text fields use '' for blank. A member's needs and wishes live in
-- wishlist_item.
CREATE TABLE family_member (
    id INTEGER PRIMARY KEY,
    family_id INTEGER NOT NULL REFERENCES family (id) ON DELETE CASCADE,
    label TEXT NOT NULL,
    age INTEGER,
    gender TEXT NOT NULL DEFAULT '',
    shirt_size TEXT NOT NULL DEFAULT '',
    pant_size TEXT NOT NULL DEFAULT '',
    shoe_size TEXT NOT NULL DEFAULT '',
    preferred_brands_colors TEXT NOT NULL DEFAULT '',
    preferred_scents TEXT NOT NULL DEFAULT '',
    prefers_unscented INTEGER NOT NULL DEFAULT 0 CHECK (
        prefers_unscented IN (0, 1)
    ),
    preferred_gift_cards TEXT NOT NULL DEFAULT '',
    additional_info TEXT NOT NULL DEFAULT '',
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

CREATE INDEX family_member_family_id_idx ON family_member (family_id);

-- One row per line in the sheet's "Need" or "Wish" section.
CREATE TABLE wishlist_item (
    id INTEGER PRIMARY KEY,
    family_member_id INTEGER NOT NULL
    REFERENCES family_member (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('need', 'wish')),
    name TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

CREATE INDEX wishlist_item_family_member_id_idx
ON wishlist_item (family_member_id);

-- Each row is one giver's claim on one family. Releasing a claim deletes the
-- row. The UNIQUE constraints enforce the one-giver-per-family and
-- one-family-per-giver rules.
CREATE TABLE claim (
    id INTEGER PRIMARY KEY,
    family_id INTEGER NOT NULL UNIQUE REFERENCES family (id) ON DELETE CASCADE,
    giver_name TEXT NOT NULL,
    giver_email TEXT NOT NULL UNIQUE COLLATE nocase,
    giver_phone TEXT NOT NULL,
    -- SHA-256 of the token in the giver's private link. The plaintext token
    -- is never stored, so admins rotate it to re-send a link.
    access_token_hash TEXT NOT NULL UNIQUE,
    created_time TEXT NOT NULL
) STRICT;
