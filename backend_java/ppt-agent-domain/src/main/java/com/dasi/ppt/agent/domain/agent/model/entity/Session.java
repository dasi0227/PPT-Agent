package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.Plan;

public record Session(
        String id,
        String projectId,
        long revision,
        String title,
        Plan plan,
        long updatedAt
) {
}
