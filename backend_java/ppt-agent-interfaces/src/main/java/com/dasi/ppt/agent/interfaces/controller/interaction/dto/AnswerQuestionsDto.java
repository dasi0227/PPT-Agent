package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import java.util.List;

public record AnswerQuestionsDto(
        String runId,
        String toolCallId,
        String version,
        List<QuestionAnswerDto> answers
) {

    public record QuestionAnswerDto(
            String questionId,
            String selectedOptionId,
            String customText,
            Boolean skipped
    ) {
    }
}
