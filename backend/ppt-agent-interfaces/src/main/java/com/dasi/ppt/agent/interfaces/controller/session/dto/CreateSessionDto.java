package com.dasi.ppt.agent.interfaces.controller.session.dto;

import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class CreateSessionDto {
    @NotBlank
    private String projectId;
    private String title;

    public Session toEntity() {
        return Session.builder()
                .projectId(projectId)
                .title(title)
                .build();
    }
}
