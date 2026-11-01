-- Monitor-driven ordered fallback groups.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS group_type VARCHAR(20) NOT NULL DEFAULT 'business',
    ADD COLUMN IF NOT EXISTS fallback_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE groups
SET group_type = 'business'
WHERE group_type IS NULL OR group_type = '';

-- fallback_group_id belongs to the independent Claude Code client restriction.
-- Never migrate it into a health-driven routing chain. Existing groups retain
-- their exact routing behavior until an administrator creates a fallback entry.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'groups_group_type_check'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_group_type_check
            CHECK (group_type IN ('business', 'fallback'));
    END IF;
END $$;
