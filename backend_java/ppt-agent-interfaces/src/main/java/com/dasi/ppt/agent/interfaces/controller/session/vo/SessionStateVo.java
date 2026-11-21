package com.dasi.ppt.agent.interfaces.controller.session.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanStepStatus;

import java.util.List;

public record SessionStateVo(
        String revision,
        PlanVo plan,
        ContextWindowVo context,
        String activeRunId,
        List<String> activeCommandIds,
        List<String> suggestions
) {

    public record PlanVo(String runId, List<PlanStepVo> steps) {
    }

    public record PlanStepVo(String title, PlanStepStatus status) {
    }

    public record ContextWindowVo(
            long current,
            long total,
            long thresholdMin,
            long thresholdMax,
            ContextDetailsVo details
    ) {
    }

    public record ContextDetailsVo(
            List<TokenBreakdownItemVo> systemPrompt,
            List<TokenBreakdownItemVo> runtime,
            List<TokenBreakdownItemVo> chatHistory,
            List<TokenBreakdownItemVo> readFile,
            List<TokenBreakdownItemVo> other
    ) {
    }

    public record TokenBreakdownItemVo(String name, long tokens) {
    }
}
