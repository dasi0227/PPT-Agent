package com.dasi.ppt.agent.domain.agent.service.tool.editconstraint;

import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResponse;
import org.springframework.stereotype.Component;

@Component("edit_constraint")
public class EditConstraintTool implements Tool<EditConstraintRequest> {

    @Override
    public ToolResponse execute(EditConstraintRequest request) {
        throw new UnsupportedOperationException();
    }
}
