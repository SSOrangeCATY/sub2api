-- NULL inherits the actual monitored target. Existing unused entry rates are not activated.
ALTER TABLE groups ADD COLUMN IF NOT EXISTS fallback_rate_multiplier DECIMAL(10,4);
ALTER TABLE groups ADD CONSTRAINT groups_fallback_rate_multiplier_check CHECK (fallback_rate_multiplier IS NULL OR (group_type = 'fallback' AND fallback_rate_multiplier > 0 AND fallback_rate_multiplier <> 'NaN'::numeric));
