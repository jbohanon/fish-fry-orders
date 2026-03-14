-- Close expired sessions that are still marked ACTIVE.
UPDATE sessions
SET status = 'CLOSED',
    closed_at = COALESCE(closed_at, NOW()),
    updated_at = NOW()
WHERE status = 'ACTIVE' AND expires_at <= NOW();

-- Keep only the newest unexpired ACTIVE session if multiple exist.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (ORDER BY started_at DESC, id DESC) AS rn
    FROM sessions
    WHERE status = 'ACTIVE' AND expires_at > NOW()
)
UPDATE sessions s
SET status = 'CLOSED',
    closed_at = COALESCE(closed_at, NOW()),
    updated_at = NOW()
FROM ranked r
WHERE s.id = r.id AND r.rn > 1;

-- Enforce single ACTIVE session at the database level.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_single_active
ON sessions (status)
WHERE status = 'ACTIVE';
