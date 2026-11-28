package com.dasi.ppt.agent.domain.agent.service.tool.editspec;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.model.enumeration.SlidePurpose;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class EditSpecRequest extends ToolRequest {

    private final String slideId;
    private final String core;
    private final SlidePurpose purpose;
    private final String layout;
    private final List<Spec.Element> elements;
}
