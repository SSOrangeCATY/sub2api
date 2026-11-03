-- QQ welfare uses separate points and immutable receipts; no changes to paid balances until a reward is claimed.
CREATE TABLE IF NOT EXISTS qq_welfare_config (
 id INTEGER PRIMARY KEY CHECK (id=1), enabled BOOLEAN NOT NULL DEFAULT true,
 group_number VARCHAR(32) NOT NULL DEFAULT '', group_open_id VARCHAR(128) NOT NULL DEFAULT '',
 bot_key VARCHAR(64) NOT NULL, group_code_hash VARCHAR(64), group_code_expires TIMESTAMPTZ,
 last_bot_seen_at TIMESTAMPTZ, updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_bind_codes (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 code_hash VARCHAR(64) NOT NULL UNIQUE, expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_accounts (
 user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
 qq_id VARCHAR(128) NOT NULL UNIQUE, group_id VARCHAR(128) NOT NULL,
 points BIGINT NOT NULL DEFAULT 0 CHECK(points>=0), verified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_checkins (
 user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, day DATE NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), PRIMARY KEY(user_id,day)
);
CREATE TABLE IF NOT EXISTS qq_welfare_rewards (
 id BIGSERIAL PRIMARY KEY, title VARCHAR(80) NOT NULL,
 points_cost INTEGER NOT NULL CHECK(points_cost>0), balance_value NUMERIC(20,8) NOT NULL CHECK(balance_value>0),
 stock INTEGER NOT NULL CHECK(stock>=0), version BIGINT NOT NULL DEFAULT 1, enabled BOOLEAN NOT NULL DEFAULT false,
 expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_coupons (
 id BIGSERIAL PRIMARY KEY, code VARCHAR(32) NOT NULL UNIQUE, title VARCHAR(80) NOT NULL,
 balance_value NUMERIC(20,8) NOT NULL CHECK(balance_value>0), max_claims INTEGER NOT NULL DEFAULT 0 CHECK(max_claims>=0),
 claimed INTEGER NOT NULL DEFAULT 0 CHECK(claimed>=0), enabled BOOLEAN NOT NULL DEFAULT false,
 expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_orders (
 id VARCHAR(32) PRIMARY KEY, request_key VARCHAR(64) NOT NULL UNIQUE,
 user_id BIGINT NOT NULL REFERENCES users(id), reward_id BIGINT REFERENCES qq_welfare_rewards(id),
 coupon_id BIGINT REFERENCES qq_welfare_coupons(id), title VARCHAR(80) NOT NULL,
 points_cost INTEGER NOT NULL CHECK(points_cost>=0), balance_value NUMERIC(20,8) NOT NULL CHECK(balance_value>0),
 redeem_code_id BIGINT NOT NULL REFERENCES redeem_codes(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS qq_welfare_orders_user_created ON qq_welfare_orders(user_id,created_at DESC);
CREATE TABLE IF NOT EXISTS qq_welfare_claims (
 coupon_id BIGINT NOT NULL REFERENCES qq_welfare_coupons(id), user_id BIGINT NOT NULL REFERENCES users(id),
 order_id VARCHAR(32) NOT NULL UNIQUE REFERENCES qq_welfare_orders(id), PRIMARY KEY(coupon_id,user_id)
);
CREATE TABLE IF NOT EXISTS qq_welfare_ledger (
 id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES users(id), delta BIGINT NOT NULL,
 reason VARCHAR(32) NOT NULL, reference VARCHAR(100) NOT NULL UNIQUE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS qq_welfare_limits (
 key VARCHAR(64) PRIMARY KEY, started_at TIMESTAMPTZ NOT NULL, count INTEGER NOT NULL
);
