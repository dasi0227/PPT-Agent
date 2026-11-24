package com.dasi.ppt.agent.domain.shared.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum RunActivity {
    RUN_PREPARING("run.preparing"),
    RUN_ANALYZING("run.analyzing"),
    RUN_RETRYING("run.retrying"),
    RUN_CANCELING("run.canceling"),
    MODEL_FALLBACK("model.fallback"),
    PLAN_PREPARING("plan.preparing"),
    PRESENTATION_META_READING("presentation.meta.reading"),
    PRESENTATION_META_UPDATING("presentation.meta.updating"),
    PRESENTATION_STRUCTURE_READING("presentation.structure.reading"),
    PRESENTATION_STRUCTURE_UPDATING("presentation.structure.updating"),
    PRESENTATION_CONSTRAINT_READING("presentation.constraint.reading"),
    PRESENTATION_CONSTRAINT_UPDATING("presentation.constraint.updating"),
    SLIDE_CONTENT_READING("slide.content.reading"),
    REFERENCE_INSPECTING("reference.inspecting"),
    SLIDE_UPDATING("slide.updating"),
    SLIDE_LAYOUT_CHECKING("slide.layout.checking"),
    RESOURCE_PREPARING("resource.preparing"),
    COMMAND_EXECUTING("command.executing"),
    PRESENTATION_REVIEWING("presentation.reviewing");

    @JsonValue
    private final String value;

    @JsonCreator
    public static RunActivity fromValue(String value) {
        for (RunActivity candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(RunActivity.class, value);
    }
}
