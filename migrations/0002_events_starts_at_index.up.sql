-- Matches the ORDER BY of GET /events, so each page is an index range scan.
CREATE INDEX events_starts_at_id_idx ON events (starts_at, id);
