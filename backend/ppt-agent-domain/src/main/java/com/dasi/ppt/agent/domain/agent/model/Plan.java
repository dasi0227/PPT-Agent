package com.dasi.ppt.agent.domain.agent.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class Plan {
    private String runId;
    private List<PlanStep> steps;
}
