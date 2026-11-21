package com.dasi.ppt.agent.domain.agent.model;

import java.util.List;

public record ContextEstimate(long current, Details details) {

    public record Details(
            List<Item> systemPrompt,
            List<Item> runtime,
            List<Item> chatHistory,
            List<Item> readFile,
            List<Item> other
    ) {
    }

    public record Item(String name, long tokens) {
    }
}
