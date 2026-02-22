-- Switch deduplication key from (scryfall_set_code, product_type) to mtgstocks_id
ALTER TABLE collection_packs
    DROP CONSTRAINT IF EXISTS collection_packs_scryfall_set_code_product_type_key;

-- Unique index on mtgstocks_id (partial — only when set, to allow legacy rows)
CREATE UNIQUE INDEX IF NOT EXISTS collection_packs_mtgstocks_id_key
    ON collection_packs(mtgstocks_id)
    WHERE mtgstocks_id IS NOT NULL;
