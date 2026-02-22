CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    google_id TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    name TEXT,
    role TEXT NOT NULL DEFAULT 'user',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS collection_packs (
    id SERIAL PRIMARY KEY,
    scryfall_set_code TEXT NOT NULL,
    name TEXT NOT NULL,
    set_name TEXT NOT NULL,
    product_type TEXT NOT NULL,
    mtgstocks_id INTEGER,
    quantity INTEGER NOT NULL DEFAULT 1,
    weight NUMERIC NOT NULL DEFAULT 1.0,
    notes TEXT,
    added_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(scryfall_set_code, product_type)
);
