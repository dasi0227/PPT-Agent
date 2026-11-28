package com.dasi.ppt.agent.domain.agent.service.tool.editconstraint;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import org.springframework.stereotype.Component;

@Component("edit_constraint")
public class EditConstraintTool implements Tool<EditConstraintRequest> {

    @Override
    public ToolResult execute(EditConstraintRequest request) {
        throw new UnsupportedOperationException();
    }
}
