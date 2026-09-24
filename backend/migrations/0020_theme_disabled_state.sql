ALTER TABLE resource_states RENAME TO resource_states_previous;
CREATE TABLE resource_states (
    resource_type TEXT NOT NULL CHECK (resource_type IN ('theme', 'component', 'skill', 'prompt')),
    resource_id   TEXT NOT NULL,
    disabled      INTEGER NOT NULL DEFAULT 0 CHECK (disabled IN (0, 1)),
    updated_at    INTEGER NOT NULL,
    PRIMARY KEY (resource_type, resource_id)
);
INSERT INTO resource_states SELECT resource_type, resource_id, disabled, updated_at FROM resource_states_previous;
DROP TABLE resource_states_previous;
