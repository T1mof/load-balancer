CREATE TABLE IF NOT EXISTS rate_limits (
    client_id VARCHAR(255) PRIMARY KEY,
    capacity INTEGER NOT NULL,
    refill_rate FLOAT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rate_limits_client_id ON rate_limits(client_id);
