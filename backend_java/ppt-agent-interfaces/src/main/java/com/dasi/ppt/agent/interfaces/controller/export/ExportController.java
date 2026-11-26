package com.dasi.ppt.agent.interfaces.controller.export;

import com.dasi.ppt.agent.domain.project.model.entity.ExportFile;
import com.dasi.ppt.agent.domain.project.service.ExportService;
import com.dasi.ppt.agent.interfaces.controller.export.dto.CreateExportDto;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.InputStreamResource;
import org.springframework.http.CacheControl;
import org.springframework.http.ContentDisposition;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

import java.nio.charset.StandardCharsets;

@RestController
@RequiredArgsConstructor
public class ExportController {

    private final ExportService exportService;

    @PostMapping("/exports/create")
    public ResponseEntity<InputStreamResource> exportProject(@Valid @RequestBody CreateExportDto request) {
        ExportFile exportFile = exportService.exportProject(request.getProjectId(), request.getFormat());
        MediaType contentType = switch (request.getFormat()) {
            case HTML -> MediaType.parseMediaType("text/html;charset=UTF-8");
            case PDF -> MediaType.APPLICATION_PDF;
            case PNG -> MediaType.parseMediaType("application/zip");
        };
        String disposition = ContentDisposition.attachment().filename(exportFile.getFilename(), StandardCharsets.UTF_8).build().toString();
        return ResponseEntity.ok()
                .contentType(contentType)
                .contentLength(exportFile.getSize())
                .cacheControl(CacheControl.noStore())
                .header(HttpHeaders.CONTENT_DISPOSITION, disposition)
                .body(new InputStreamResource(exportFile.getContent()));
    }
}
