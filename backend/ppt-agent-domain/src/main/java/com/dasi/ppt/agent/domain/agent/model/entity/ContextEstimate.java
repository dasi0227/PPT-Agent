package com.dasi.ppt.agent.domain.agent.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ContextEstimate {
    private long current;
    private Details details;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Details {
        private List<Item> systemPrompt;
        private List<Item> runtime;
        private List<Item> chatHistory;
        private List<Item> readFile;
        private List<Item> other;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Item {
        private String name;
        private long tokens;
    }
}
