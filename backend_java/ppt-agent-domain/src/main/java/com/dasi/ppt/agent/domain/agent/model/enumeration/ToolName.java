package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ToolName {
    @JsonProperty("read_resource")
    READ_RESOURCE,
    @JsonProperty("read_image")
    READ_IMAGE,
    @JsonProperty("load_skill")
    LOAD_SKILL,
    @JsonProperty("load_component")
    LOAD_COMPONENT,
    @JsonProperty("edit_meta")
    EDIT_META,
    @JsonProperty("edit_constraints")
    EDIT_CONSTRAINTS,
    @JsonProperty("edit_outline")
    EDIT_OUTLINE,
    @JsonProperty("edit_spec")
    EDIT_SPEC,
    @JsonProperty("edit_html")
    EDIT_HTML,
    @JsonProperty("run_command")
    RUN_COMMAND,
    @JsonProperty("render_slide")
    RENDER_SLIDE,
    @JsonProperty("review_ppt")
    REVIEW_PPT,
    @JsonProperty("ask_user")
    ASK_USER,
    @JsonProperty("create_plan")
    CREATE_PLAN,
    @JsonProperty("update_plan")
    UPDATE_PLAN,
    @JsonProperty("request_privilege")
    REQUEST_PRIVILEGE
}
