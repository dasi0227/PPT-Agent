package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ToolName {
    READ_RESOURCE("read_resource"),
    READ_IMAGE("read_image"),
    LOAD_SKILL("load_skill"),
    LOAD_COMPONENT("load_component"),
    EDIT_META("edit_meta"),
    EDIT_CONSTRAINTS("edit_constraints"),
    EDIT_OUTLINE("edit_outline"),
    EDIT_SPEC("edit_spec"),
    EDIT_HTML("edit_html"),
    RUN_COMMAND("run_command"),
    RENDER_SLIDE("render_slide"),
    REVIEW_PPT("review_ppt"),
    ASK_USER("ask_user"),
    CREATE_PLAN("create_plan"),
    UPDATE_PLAN("update_plan"),
    REQUEST_PRIVILEGE("request_privilege");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ToolName fromValue(String value) {
        for (ToolName candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ToolName.class, value);
    }
}
