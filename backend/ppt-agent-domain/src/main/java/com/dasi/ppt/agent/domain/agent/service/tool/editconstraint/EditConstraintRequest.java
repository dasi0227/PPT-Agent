package com.dasi.ppt.agent.domain.agent.service.tool.editconstraint;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class EditConstraintRequest extends ToolRequest {

    private final String type;
    private final List<String> prohibitions;
    private final List<String> requirements;
}
