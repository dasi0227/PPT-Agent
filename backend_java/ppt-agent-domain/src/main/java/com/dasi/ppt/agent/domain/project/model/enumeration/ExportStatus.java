package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ExportStatus {
    @JsonProperty("snapshotting")
    SNAPSHOTTING,
    @JsonProperty("rendering")
    RENDERING,
    @JsonProperty("packaging")
    PACKAGING,
    @JsonProperty("ready")
    READY,
    @JsonProperty("failed")
    FAILED,
    @JsonProperty("canceled")
    CANCELED
}
