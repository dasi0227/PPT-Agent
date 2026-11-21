package com.dasi.ppt.agent.interfaces.controller.project.dto;

import org.springframework.web.multipart.MultipartFile;

public record SaveProjectLogoDto(String projectId, MultipartFile file) {
}
