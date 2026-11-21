package com.dasi.ppt.agent.interfaces.controller.file.dto;

import org.springframework.web.multipart.MultipartFile;

public record UploadAttachmentDto(String projectId, MultipartFile file) {
}
