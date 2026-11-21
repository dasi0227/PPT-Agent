package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ExportFormat {
    @JsonProperty("png")
    PNG,
    @JsonProperty("pdf")
    PDF,
    @JsonProperty("html")
    HTML
}
