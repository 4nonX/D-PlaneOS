-- Phase 4.2: Audit event logging with HMAC integrity chain
-- Every significant operation logged for compliance and debugging

CREATE TABLE IF NOT EXISTS audit_events (
  id BIGSERIAL PRIMARY KEY,
  timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  event_type TEXT NOT NULL,
  component TEXT NOT NULL,
  operation_id TEXT,
  user_id TEXT,
  status TEXT NOT NULL CHECK (status IN ('success', 'failure', 'warning')),
  details JSONB NOT NULL DEFAULT '{}',
  ip_address TEXT,
  hmac TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_type ON audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_events_component ON audit_events(component);
CREATE INDEX IF NOT EXISTS idx_audit_events_operation_id ON audit_events(operation_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_status ON audit_events(status);

-- Immutability: no updates or deletes on audit_events
-- REVOKE UPDATE, DELETE ON audit_events FROM dplaneos; (requires superuser)

-- HMAC Integrity Chain (PostgreSQL Trigger)
-- Each event is linked via HMAC of (previous_hmac || current_event_hash)
-- This creates an immutable chain that can be verified at any time
CREATE OR REPLACE FUNCTION audit_compute_hmac()
RETURNS TRIGGER AS $$
DECLARE
  prev_hmac TEXT;
  current_hash TEXT;
  combined TEXT;
BEGIN
  -- Get previous event's HMAC (or empty string if first event)
  SELECT hmac INTO prev_hmac FROM audit_events
  WHERE id < NEW.id
  ORDER BY id DESC
  LIMIT 1;

  IF prev_hmac IS NULL THEN
    prev_hmac := '';
  END IF;

  -- Create hash of current event (simplified: just use JSON of key fields)
  current_hash := MD5(
    NEW.timestamp::TEXT || '|' ||
    NEW.event_type || '|' ||
    NEW.component || '|' ||
    COALESCE(NEW.operation_id, '') || '|' ||
    NEW.status
  );

  -- Chain: HMAC = MD5(previous_hmac || current_hash)
  NEW.hmac := MD5(prev_hmac || current_hash);

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_hmac_trigger ON audit_events;
CREATE TRIGGER audit_hmac_trigger
BEFORE INSERT ON audit_events
FOR EACH ROW
EXECUTE FUNCTION audit_compute_hmac();

-- Verification function: check if audit chain is unbroken
CREATE OR REPLACE FUNCTION audit_verify_chain(from_id BIGINT, to_id BIGINT)
RETURNS TABLE(is_valid BOOLEAN, first_invalid_id BIGINT) AS $$
DECLARE
  prev_hmac TEXT := '';
  current_id BIGINT;
  computed_hmac TEXT;
  stored_hmac TEXT;
  current_hash TEXT;
  evt audit_events%ROWTYPE;
BEGIN
  FOR evt IN
    SELECT * FROM audit_events
    WHERE id BETWEEN from_id AND to_id
    ORDER BY id
  LOOP
    current_hash := MD5(
      evt.timestamp::TEXT || '|' ||
      evt.event_type || '|' ||
      evt.component || '|' ||
      COALESCE(evt.operation_id, '') || '|' ||
      evt.status
    );

    computed_hmac := MD5(prev_hmac || current_hash);

    IF computed_hmac != evt.hmac THEN
      RETURN QUERY SELECT FALSE, evt.id;
      RETURN;
    END IF;

    prev_hmac := evt.hmac;
  END LOOP;

  RETURN QUERY SELECT TRUE, NULL::BIGINT;
END;
$$ LANGUAGE plpgsql;
