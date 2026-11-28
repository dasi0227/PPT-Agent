package com.dasi.ppt.agent.domain.agent.service.tool.askuser;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolRequest;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;

import java.util.List;

@Getter
@SuperBuilder
public final class AskUserRequest extends ToolRequest {

    private final List<Question> questions;

    @Getter
    @Builder
    public static final class Question {

        private final String question;
        private final String reason;
        private final List<Option> options;
    }

    @Getter
    @Builder
    public static final class Option {

        private final String label;
        private final String description;
    }
}
