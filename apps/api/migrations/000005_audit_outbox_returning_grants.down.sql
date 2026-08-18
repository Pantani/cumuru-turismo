BEGIN;

REVOKE SELECT (id) ON TABLE platform.outbox_events FROM app_runtime;
REVOKE SELECT (id) ON TABLE platform.audit_events FROM app_runtime;

COMMIT;
