-- +goose Up
-- kacho-geo squashed baseline (epic kacho-geo S2 — Geography ported from
-- kacho-compute). Schema kacho_geo. Flat resources (no K8s envelope). Region/Zone
-- are global cluster-scoped read-only catalogs; admin CRUD via Internal* (:9091).
-- All id-columns are TEXT (admin-assigned "ru-central1" / "ru-central1-a").

CREATE SCHEMA IF NOT EXISTS kacho_geo;
SET search_path TO kacho_geo, public;

-- ---------------------------------------------------------------------------
-- regions — global region catalog (id = "ru-central1"). Admin-assigned PK.
-- ---------------------------------------------------------------------------
CREATE TABLE regions (
  id         TEXT         PRIMARY KEY,         -- "ru-central1"
  name       TEXT         NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- zones — availability zones (id = "ru-central1-a"), belong to a region.
-- zones.region_id → regions(id) ON DELETE RESTRICT: a region with zones cannot
-- be deleted (within-service invariant on the DB level, ban #10).
-- ---------------------------------------------------------------------------
CREATE TABLE zones (
  id         TEXT         PRIMARY KEY,                  -- "ru-central1-a"
  region_id  TEXT         NOT NULL
               REFERENCES regions (id) ON DELETE RESTRICT, -- FK RESTRICT
  status     TEXT         NOT NULL DEFAULT 'UP',        -- UP | DOWN | STATUS_UNSPECIFIED
  name       TEXT         NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX zones_region_idx ON zones (region_id);

-- ---------------------------------------------------------------------------
-- geo_outbox — audit-outbox for admin Region/Zone mutations (<domain>_outbox
-- convention, parity with compute_outbox / vpc_outbox). Schema matches the
-- corelib outbox.Emit helper: (sequence_no, resource_kind, resource_id,
-- event_type, payload, created_at). Rows are written ATOMICALLY in the admin
-- writer-tx (ban #10).
-- ---------------------------------------------------------------------------
CREATE TABLE geo_outbox (
  sequence_no   BIGSERIAL    PRIMARY KEY,
  resource_kind TEXT         NOT NULL,        -- Region | Zone
  resource_id   TEXT         NOT NULL,
  event_type    TEXT         NOT NULL,        -- CREATED | UPDATED | DELETED
  payload       JSONB        NOT NULL DEFAULT '{}'::jsonb,
  created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
  processed_at  TIMESTAMPTZ
);
CREATE INDEX geo_outbox_seq_idx  ON geo_outbox (sequence_no);
CREATE INDEX geo_outbox_kind_idx ON geo_outbox (resource_kind, sequence_no);

-- +goose StatementBegin
CREATE FUNCTION geo_outbox_notify() RETURNS trigger
  LANGUAGE plpgsql AS $$
BEGIN
  PERFORM pg_notify('geo_outbox', NEW.sequence_no::text);
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER geo_outbox_notify_trg AFTER INSERT ON geo_outbox
  FOR EACH ROW EXECUTE FUNCTION geo_outbox_notify();

-- ---------------------------------------------------------------------------
-- seed: ru-central1 + ru-central1-{a,b,d} (status UP). Idempotent: ON CONFLICT
-- so re-applies (and the S3 data-migration into the same rows) never collide.
-- created_at defaults to now() on first insert; preserved on conflict (DO NOTHING).
-- ---------------------------------------------------------------------------
INSERT INTO regions (id, name) VALUES
  ('ru-central1', 'Russia Central 1')
  ON CONFLICT (id) DO NOTHING;

INSERT INTO zones (id, region_id, status, name) VALUES
  ('ru-central1-a', 'ru-central1', 'UP', 'Russia Central 1 A'),
  ('ru-central1-b', 'ru-central1', 'UP', 'Russia Central 1 B'),
  ('ru-central1-d', 'ru-central1', 'UP', 'Russia Central 1 D')
  ON CONFLICT (id) DO NOTHING;

-- +goose Down
SET search_path TO kacho_geo, public;
DROP TRIGGER IF EXISTS geo_outbox_notify_trg ON geo_outbox;
DROP FUNCTION IF EXISTS geo_outbox_notify();
DROP TABLE IF EXISTS geo_outbox;
DROP TABLE IF EXISTS zones;
DROP TABLE IF EXISTS regions;
DROP SCHEMA IF EXISTS kacho_geo;
