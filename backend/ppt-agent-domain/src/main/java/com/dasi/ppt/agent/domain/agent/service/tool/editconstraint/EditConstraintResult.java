package com.dasi.ppt.agent.domain.agent.service.tool.editconstraint;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

@Getter
@SuperBuilder
public final class EditConstraintResult extends ToolResult {

    private final Constraint content;
}
