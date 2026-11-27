package com.dasi.ppt.agent.interfaces.controller.session.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeleteSessionDto {
    private String sessionId;
}
