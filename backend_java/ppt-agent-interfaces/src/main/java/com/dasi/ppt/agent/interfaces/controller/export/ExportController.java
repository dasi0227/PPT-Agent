package com.dasi.ppt.agent.interfaces.controller.export;

import com.dasi.ppt.agent.domain.project.service.ExportService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class ExportController {

    private final ExportService exportService;
}
