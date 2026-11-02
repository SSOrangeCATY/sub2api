-- Billing identity is independent of authorization role and group access.
-- Existing overrides are migrated only by the explicit administrator action.
ALTER TABLE users ADD COLUMN IF NOT EXISTS billing_identity VARCHAR(20) NOT NULL DEFAULT 'standard';
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_billing_identity_check' AND conrelid = 'users'::regclass) THEN
  ALTER TABLE users ADD CONSTRAINT users_billing_identity_check CHECK (billing_identity IN ('standard', 'purchase'));
 END IF;
END $$;
CREATE TABLE IF NOT EXISTS billing_identity_migrations (
 id BIGSERIAL PRIMARY KEY,
 actor_admin_id BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 backup JSONB NOT NULL
);
