package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanStepStatus;

public record PlanStep(String title, PlanStepStatus status) {
}
