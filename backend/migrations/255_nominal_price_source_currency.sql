-- Provenance of numbers imported 1:1 into USD price overrides. No FX, repricing,
-- balance conversion or historical ledger mutation is performed by this migration.
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS source_currency VARCHAR(3) NOT NULL DEFAULT ''
    CHECK (source_currency = '' OR source_currency ~ '^[A-Z]{3}$');
ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS source_currency VARCHAR(3) NOT NULL DEFAULT ''
    CHECK (source_currency = '' OR source_currency ~ '^[A-Z]{3}$');
