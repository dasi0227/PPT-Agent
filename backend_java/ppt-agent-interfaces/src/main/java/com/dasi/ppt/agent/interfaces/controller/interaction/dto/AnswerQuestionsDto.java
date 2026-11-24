package com.dasi.ppt.agent.interfaces.controller.interaction.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AnswerQuestionsDto {
    private String runId;
    private String toolCallId;
    private String version;
    private List<QuestionAnswerDto> answers;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class QuestionAnswerDto {
        private String questionId;
        private String selectedOptionId;
        private String customText;
        private Boolean skipped;
    }
}
