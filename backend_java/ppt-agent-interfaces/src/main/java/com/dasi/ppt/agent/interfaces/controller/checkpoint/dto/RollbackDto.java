package com.dasi.ppt.agent.interfaces.controller.checkpoint.dto;

public record RollbackDto(String runId, String expectedRevision) {
}
