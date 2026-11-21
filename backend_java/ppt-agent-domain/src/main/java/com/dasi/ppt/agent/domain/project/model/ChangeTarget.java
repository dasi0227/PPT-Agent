package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;

public record ChangeTarget(
        ChangePart part,
        String path,
        String slideId
) {
}
