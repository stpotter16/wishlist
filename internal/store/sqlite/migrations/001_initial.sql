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

-- The app labels each household as "Household <id>". AUTOINCREMENT keeps
-- SQLite from reusing a deleted household's ID, which would hand its public
-- label and URL to a different household. A 'family' is a mom and her
-- children. A 'single' household is one woman living on her own.
CREATE TABLE household (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL CHECK (kind IN ('family', 'single')),
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

-- Columns mirror the paper intake sheet that admins transcribe. Optional
-- text fields use '' for blank. A member's needs and wishes live in
-- wishlist_item. To limit identifying data, the head of the household stores
-- only her first name and last initial, and children store only a first name
-- (last_initial is '').
CREATE TABLE household_member (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL REFERENCES household (id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_initial TEXT NOT NULL DEFAULT '',
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

CREATE INDEX household_member_household_id_idx
ON household_member (household_id);

-- One row per line in the sheet's "Need" or "Wish" section.
CREATE TABLE wishlist_item (
    id INTEGER PRIMARY KEY,
    household_member_id INTEGER NOT NULL
    REFERENCES household_member (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('need', 'wish')),
    name TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_time TEXT NOT NULL,
    last_modified_time TEXT NOT NULL
) STRICT;

CREATE INDEX wishlist_item_household_member_id_idx
ON wishlist_item (household_member_id);

-- Each row is one giver's claim on one household. Releasing a claim deletes the
-- row. The UNIQUE constraints enforce the one-giver-per-household and
-- one-household-per-giver rules.
CREATE TABLE claim (
    id INTEGER PRIMARY KEY,
    household_id INTEGER NOT NULL UNIQUE
    REFERENCES household (id) ON DELETE CASCADE,
    giver_name TEXT NOT NULL,
    giver_email TEXT NOT NULL UNIQUE COLLATE nocase,
    giver_phone TEXT NOT NULL,
    -- SHA-256 of the token in the giver's private link. The plaintext token
    -- is never stored, so admins rotate it to re-send a link.
    access_token_hash TEXT NOT NULL UNIQUE,
    created_time TEXT NOT NULL
) STRICT;
