package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PreviewIssue {
    @JsonProperty("expression_issue")
    EXPRESSION_ISSUE,
    @JsonProperty("layout_issue")
    LAYOUT_ISSUE,
    @JsonProperty("visual_issue")
    VISUAL_ISSUE,
    @JsonProperty("none")
    NONE
}
