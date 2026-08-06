CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id VARCHAR(64) NOT NULL,
    team_id VARCHAR(64) NOT NULL,
    session_id VARCHAR(64) NOT NULL,
    model VARCHAR(64) NOT NULL,
    prompt_hash VARCHAR(64) NOT NULL,
    status_code INT NOT NULL,
    cost_usd NUMERIC(10, 6) DEFAULT 0.000000,
    blocked_reason VARCHAR(64) DEFAULT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_team_id ON audit_logs(team_id);
CREATE INDEX idx_audit_created_at ON audit_logs(created_at);
