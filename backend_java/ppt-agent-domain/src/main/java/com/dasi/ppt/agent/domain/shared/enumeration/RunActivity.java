package com.dasi.ppt.agent.domain.shared.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum RunActivity {
    @JsonProperty("run.preparing")
    RUN_PREPARING,
    @JsonProperty("run.analyzing")
    RUN_ANALYZING,
    @JsonProperty("run.retrying")
    RUN_RETRYING,
    @JsonProperty("run.canceling")
    RUN_CANCELING,
    @JsonProperty("model.fallback")
    MODEL_FALLBACK,
    @JsonProperty("plan.preparing")
    PLAN_PREPARING,
    @JsonProperty("presentation.meta.reading")
    PRESENTATION_META_READING,
    @JsonProperty("presentation.meta.updating")
    PRESENTATION_META_UPDATING,
    @JsonProperty("presentation.structure.reading")
    PRESENTATION_STRUCTURE_READING,
    @JsonProperty("presentation.structure.updating")
    PRESENTATION_STRUCTURE_UPDATING,
    @JsonProperty("presentation.constraint.reading")
    PRESENTATION_CONSTRAINT_READING,
    @JsonProperty("presentation.constraint.updating")
    PRESENTATION_CONSTRAINT_UPDATING,
    @JsonProperty("slide.content.reading")
    SLIDE_CONTENT_READING,
    @JsonProperty("reference.inspecting")
    REFERENCE_INSPECTING,
    @JsonProperty("slide.updating")
    SLIDE_UPDATING,
    @JsonProperty("slide.layout.checking")
    SLIDE_LAYOUT_CHECKING,
    @JsonProperty("resource.preparing")
    RESOURCE_PREPARING,
    @JsonProperty("command.executing")
    COMMAND_EXECUTING,
    @JsonProperty("presentation.reviewing")
    PRESENTATION_REVIEWING
}
