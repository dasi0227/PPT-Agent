package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PreviewMismatch {
    @JsonProperty("meta_mismatch")
    META_MISMATCH,
    @JsonProperty("content_mismatch")
    CONTENT_MISMATCH,
    @JsonProperty("spec_mismatch")
    SPEC_MISMATCH,
    @JsonProperty("visual_mismatch")
    VISUAL_MISMATCH,
    @JsonProperty("none")
    NONE
}
