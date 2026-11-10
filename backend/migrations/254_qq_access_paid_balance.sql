-- Existing receipts have no reliable payment provenance. Never backfill them as paid.
ALTER TABLE qq_welfare_config ADD COLUMN require_qq_for_api boolean NOT NULL DEFAULT false;
ALTER TABLE qq_welfare_coupons ADD COLUMN min_paid_balance numeric(20,8) NOT NULL DEFAULT 0 CHECK (min_paid_balance >= 0);
ALTER TABLE redeem_codes ADD COLUMN balance_source varchar(20) NOT NULL DEFAULT 'unknown'
    CHECK (balance_source IN ('unknown', 'gift', 'paid'));
ALTER TABLE redeem_codes ADD CONSTRAINT redeem_paid_balance_only CHECK
    (balance_source <> 'paid' OR (type IN ('balance', 'admin_balance') AND value > 0));

-- Retained independently of deletable receipts. A receipt is counted exactly once,
-- in the same transaction that consumes it and credits the balance.
CREATE TABLE paid_balance_credits (
    redeem_code_id bigint PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount numeric(20,8) NOT NULL CHECK (amount > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX paid_balance_credits_user_idx ON paid_balance_credits(user_id);

CREATE FUNCTION record_paid_balance_credit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    -- Preserve the existing users FK's ON DELETE SET NULL behavior.
    IF TG_OP = 'UPDATE' AND OLD.used_by IS NOT NULL AND NEW.used_by IS NULL AND
       NOT EXISTS (SELECT 1 FROM users WHERE id = OLD.used_by) THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' AND OLD.status = 'used' AND
       (OLD.type IN ('balance', 'admin_balance') OR NEW.type IN ('balance', 'admin_balance')) AND
       (NEW.balance_source IS DISTINCT FROM OLD.balance_source OR
        NEW.value IS DISTINCT FROM OLD.value OR NEW.type IS DISTINCT FROM OLD.type OR
        NEW.used_by IS DISTINCT FROM OLD.used_by OR NEW.status IS DISTINCT FROM OLD.status) THEN
        RAISE EXCEPTION 'Consumed receipt payment fields are immutable';
    END IF;
    IF NEW.status = 'used' AND NEW.balance_source = 'paid' THEN
        IF NEW.used_by IS NULL THEN
            RAISE EXCEPTION 'Paid receipt requires a recipient';
        END IF;
        -- Serialize against welfare claims and other balance mutations for this user.
        PERFORM id FROM users WHERE id = NEW.used_by FOR UPDATE;
        INSERT INTO paid_balance_credits(redeem_code_id,user_id,amount)
        VALUES(NEW.id,NEW.used_by,NEW.value) ON CONFLICT (redeem_code_id) DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER redeem_paid_balance_credit AFTER INSERT OR UPDATE ON redeem_codes
    FOR EACH ROW EXECUTE FUNCTION record_paid_balance_credit();
