package com.dasi.ppt.agent.interfaces.controller.file;

import com.dasi.ppt.agent.domain.project.service.AttachmentService;
import com.dasi.ppt.agent.domain.project.service.ProjectService;
import com.dasi.ppt.agent.domain.project.service.SlidePreviewService;
import com.dasi.ppt.agent.interfaces.controller.file.dto.OpenProjectFileDto;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/files")
@RequiredArgsConstructor
public class FileController {

    private final AttachmentService attachmentService;
    private final SlidePreviewService slidePreviewService;
    private final ProjectService projectService;

    @PostMapping("/open")
    public void open(@Valid @RequestBody OpenProjectFileDto request) {
        projectService.openFile(request.getProjectId(), request.getPath());
    }
}
