package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;

import java.util.Map;

public record Project(
        String id,
        String title,
        String theme,
        Map<DecorationType, DecorationPosition> positions,
        byte[] logo,
        long updatedAt
) {
}
