package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanStepStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class PlanStep {
    private String title;
    private PlanStepStatus status;
}
