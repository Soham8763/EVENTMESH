-- Core Tables
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY,
    api_key TEXT UNIQUE NOT NULL,
    tenant_id TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    workflow_name TEXT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_definitions (
    name TEXT PRIMARY KEY,
    steps JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_executions (
    id UUID PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    workflow_name TEXT NOT NULL,
    trigger_id TEXT NOT NULL,
    status TEXT NOT NULL,
    current_step INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS workflow_step_executions (
    id UUID PRIMARY KEY,
    workflow_execution_id UUID NOT NULL REFERENCES workflow_executions(id),
    step_name TEXT NOT NULL,
    status TEXT NOT NULL,
    retry_count INTEGER DEFAULT 0,
    last_error TEXT,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Seed Data
INSERT INTO api_keys (id, api_key, tenant_id, is_active)
VALUES ('550e8400-e29b-41d4-a716-446655440001', 'demo-api-key', 'tenant-1', TRUE)
ON CONFLICT DO NOTHING;

INSERT INTO rules (id, tenant_id, event_type, workflow_name, is_active)
VALUES ('550e8400-e29b-41d4-a716-446655440002', 'tenant-1', 'user_signed_up', 'welcome_user_workflow', TRUE)
ON CONFLICT DO NOTHING;

INSERT INTO workflow_definitions (name, steps)
VALUES ('welcome_user_workflow', '[{"step": "send_welcome_email"}, {"step": "provision_account"}]'::jsonb)
ON CONFLICT DO NOTHING;
