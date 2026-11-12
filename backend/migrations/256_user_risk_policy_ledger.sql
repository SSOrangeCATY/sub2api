-- A policy owns its counter generation and access restriction independently
-- of users.status. Different policies can coexist for the same user.
CREATE TABLE IF NOT EXISTS user_risk_cycles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    policy_key VARCHAR(96) NOT NULL,
    generation BIGINT NOT NULL DEFAULT 1 CHECK (generation > 0),
    reset_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (user_id, policy_key)
);

CREATE TABLE IF NOT EXISTS user_risk_events (
    policy_key VARCHAR(96) NOT NULL,
    event_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_id VARCHAR(128) NOT NULL DEFAULT '',
    group_id BIGINT,
    model VARCHAR(255) NOT NULL DEFAULT '',
    endpoint VARCHAR(255) NOT NULL DEFAULT '',
    counted BOOLEAN NOT NULL,
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    policy_snapshot JSONB NOT NULL CHECK (jsonb_typeof(policy_snapshot) = 'object'),
    result JSONB NOT NULL CHECK (jsonb_typeof(result) = 'object'),
    admitted_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (policy_key, event_id)
);

CREATE INDEX IF NOT EXISTS idx_user_risk_events_window
    ON user_risk_events(user_id, policy_key, generation, recorded_at DESC)
    WHERE counted;

CREATE TABLE IF NOT EXISTS user_risk_restrictions (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    policy_key VARCHAR(96) NOT NULL,
    scope VARCHAR(64) NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    version BIGINT NOT NULL CHECK (version > 0),
    decision_hit_count INT NOT NULL CHECK (decision_hit_count > 0),
    threshold INT NOT NULL CHECK (threshold > 0),
    admin_release BOOLEAN NOT NULL,
    restricted_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    cleared_at TIMESTAMPTZ,
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    policy_snapshot JSONB NOT NULL CHECK (jsonb_typeof(policy_snapshot) = 'object'),
    source_event_id VARCHAR(128) NOT NULL,
    PRIMARY KEY (user_id, policy_key),
    CHECK ((admin_release AND expires_at IS NULL) OR
           (NOT admin_release AND expires_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_user_risk_restrictions_active
    ON user_risk_restrictions(policy_key, restricted_at DESC, user_id)
    WHERE cleared_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_risk_restrictions_expiry
    ON user_risk_restrictions(expires_at, user_id)
    WHERE cleared_at IS NULL AND NOT admin_release;

-- Releases and expiry retain events, prior decisions and immutable action
-- snapshots. An operation ID deduplicates repeated administrator submissions.
CREATE TABLE IF NOT EXISTS user_risk_actions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    policy_key VARCHAR(96) NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    action VARCHAR(64) NOT NULL,
    operation_id VARCHAR(128) NOT NULL,
    source_event_id VARCHAR(128) NOT NULL DEFAULT '',
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason VARCHAR(1000) NOT NULL DEFAULT '',
    decision JSONB NOT NULL CHECK (jsonb_typeof(decision) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (policy_key, operation_id)
);

CREATE INDEX IF NOT EXISTS idx_user_risk_actions_history
    ON user_risk_actions(policy_key, id DESC);
CREATE INDEX IF NOT EXISTS idx_user_risk_actions_user_history
    ON user_risk_actions(user_id, policy_key, id DESC);
