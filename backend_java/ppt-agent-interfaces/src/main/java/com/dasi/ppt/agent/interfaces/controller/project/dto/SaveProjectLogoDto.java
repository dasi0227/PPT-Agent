package com.dasi.ppt.agent.interfaces.controller.project.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.springframework.web.multipart.MultipartFile;

@Builder
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public final class SaveProjectLogoDto {
    private String projectId;
    private MultipartFile file;
}
