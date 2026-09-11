ALTER TABLE steering_inbox ADD COLUMN references_json TEXT NOT NULL DEFAULT '{"attachments":[],"dom_selections":[],"reference_order":[]}';
ALTER TABLE steering_inbox DROP COLUMN attachments_json;
