package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import lombok.Getter;

import java.util.List;

@Getter
public final class AnswerQuestionsDto {
    private String runId;
    private String toolCallId;
    private String version;
    private List<QuestionAnswerDto> answers;

    @Getter
    public static final class QuestionAnswerDto {
        private String questionId;
        private String selectedOptionId;
        private String customText;
        private Boolean skipped;
    }
}
