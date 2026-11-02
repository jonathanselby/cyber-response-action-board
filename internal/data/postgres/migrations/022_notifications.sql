-- CRAB migration 022: notifications (R-05, contract §3.5).
--
-- Authority classification:
--   crab.notification_requests  rebuildable projection of incident.notification_request records.
--   crab.notices                rebuildable projection of incident.notice records: the notices
--                               CRAB sent on its own for the five V1 events.
--   crab.notification_routes    durable supporting: one optional route per person. The address
--                               is security-supporting state, never in the ledger; the ledger holds
--                               security.notification_route_changed with a masked hint only.
--   crab.notice_cursor          durable supporting: how far the notice pass has read the ledger,
--                               and when notices started (events before that are not notified).

CREATE TABLE IF NOT EXISTS crab.notification_requests (
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    record_id text NOT NULL CHECK (btrim(record_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    communication_id text NOT NULL CHECK (btrim(communication_id) <> ''),
    channels jsonb NOT NULL,
    resend_of jsonb NOT NULL DEFAULT '[]'::jsonb,
    maker text NOT NULL CHECK (btrim(maker) <> ''),
    requested_at timestamptz NOT NULL,
    seq bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (incident_id, record_id)
);
CREATE INDEX IF NOT EXISTS notification_requests_communication_idx ON crab.notification_requests(incident_id, communication_id, requested_at);
COMMENT ON TABLE crab.notification_requests IS
    'CRAB authority=rebuildable; notification requests recorded before any attempt';

-- seq is the ledger position, so attempts at the same second keep their order.
ALTER TABLE crab.notification_outcomes ADD COLUMN IF NOT EXISTS request_id text NOT NULL DEFAULT '';
ALTER TABLE crab.notification_outcomes ADD COLUMN IF NOT EXISTS seq bigint NOT NULL DEFAULT 0;
UPDATE crab.notification_outcomes o SET seq = e.seq FROM crab.ledger_entries e
    WHERE o.seq = 0 AND convert_from(e.entry_bytes, 'UTF8')::jsonb #>> '{record,record_id}' = o.record_id
      AND convert_from(e.entry_bytes, 'UTF8')::jsonb #>> '{record,record_type}' = 'incident.notification_outcome';

CREATE TABLE IF NOT EXISTS crab.notices (
    notice_id text PRIMARY KEY CHECK (btrim(notice_id) <> ''),
    incident_id text NOT NULL CHECK (btrim(incident_id) <> ''),
    compartment text NOT NULL CHECK (btrim(compartment) <> ''),
    kind text NOT NULL CHECK (kind IN ('assignment','decision_needed','deadline','escalation','access_expiring')),
    recipient_human_id text NOT NULL DEFAULT '',
    shared_channel text NOT NULL DEFAULT '',
    channel text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('delivered','failed','suppressed','no_route')),
    detail text NOT NULL DEFAULT '',
    source_id text NOT NULL CHECK (btrim(source_id) <> ''),
    source_at timestamptz NOT NULL,
    attempted_at timestamptz NOT NULL,
    CHECK ((recipient_human_id <> '') <> (shared_channel <> ''))
);
CREATE INDEX IF NOT EXISTS notices_recipient_idx ON crab.notices(kind, recipient_human_id, incident_id, attempted_at DESC);
COMMENT ON TABLE crab.notices IS
    'CRAB authority=rebuildable; notices sent for the five V1 events; never an address';

CREATE TABLE IF NOT EXISTS crab.notification_routes (
    human_id text PRIMARY KEY CHECK (btrim(human_id) <> ''),
    channel text NOT NULL CHECK (btrim(channel) <> ''),
    address text NOT NULL CHECK (btrim(address) <> ''),
    updated_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.notification_routes IS
    'CRAB authority=supporting; one optional notification route per person; never in the ledger; deleted when the person is disabled';

CREATE TABLE IF NOT EXISTS crab.notice_cursor (
    id smallint PRIMARY KEY CHECK (id = 1),
    seq bigint NOT NULL CHECK (seq >= 0),
    started_at timestamptz NOT NULL
);
COMMENT ON TABLE crab.notice_cursor IS
    'CRAB authority=supporting; notice pass position in the ledger';

INSERT INTO crab.schema_migrations(version) VALUES (22)
ON CONFLICT (version) DO NOTHING;
