package com.dasi.ppt.agent.interfaces.controller.file;

import com.dasi.ppt.agent.domain.project.service.AttachmentService;
import com.dasi.ppt.agent.domain.project.service.SlidePreviewService;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class FileController {

    private final AttachmentService attachmentService;
    private final SlidePreviewService slidePreviewService;
    private final SettingService settingService;
}
