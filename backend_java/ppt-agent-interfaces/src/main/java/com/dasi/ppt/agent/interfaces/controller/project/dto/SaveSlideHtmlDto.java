package com.dasi.ppt.agent.interfaces.controller.project.dto;

public record SaveSlideHtmlDto(
        String projectId,
        String slideId,
        String baseHash,
        String html
) {
}
