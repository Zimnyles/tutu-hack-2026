UPDATE recommendation_requests
SET event_id = NULL
WHERE event_id IN (SELECT id FROM events WHERE is_demo);

UPDATE recommendation_options
SET event_id = NULL
WHERE event_id IN (SELECT id FROM events WHERE is_demo);

UPDATE trips
SET event_id = NULL
WHERE event_id IN (SELECT id FROM events WHERE is_demo);

DELETE FROM events WHERE is_demo;

DELETE FROM event_sources
WHERE code IN ('demo_catalog', 'deepseek_ai_demo')
  AND NOT EXISTS (
    SELECT 1 FROM events event WHERE event.source_id = event_sources.id
  );
