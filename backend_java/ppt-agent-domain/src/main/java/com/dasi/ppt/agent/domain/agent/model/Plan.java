package com.dasi.ppt.agent.domain.agent.model;

import java.util.List;

public record Plan(String runId, List<PlanStep> steps) {
}
