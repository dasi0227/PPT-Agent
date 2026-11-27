package com.dasi.ppt.agent.interfaces.controller.file;

import com.dasi.ppt.agent.domain.project.service.AttachmentService;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ThumbnailService;
import com.dasi.ppt.agent.domain.shared.exception.RenderUnavailableException;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.file.dto.OpenProjectFileDto;
import com.dasi.ppt.agent.interfaces.controller.file.dto.UploadAttachmentDto;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.CacheControl;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.DigestUtils;
import org.springframework.web.bind.annotation.*;

import java.io.IOException;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Slf4j
@RestController
@RequiredArgsConstructor
public class FileController {

    private static final MediaType WEBP = MediaType.parseMediaType("image/webp");
    private final AttachmentService attachmentService;
    private final ThumbnailService thumbnailService;
    private final DeckService deckService;

    @GetMapping("/projects/{projectId}/slides/{slideId}/thumbnail")
    public ResponseEntity<byte[]> readThumbnail(@PathVariable String projectId, @PathVariable String slideId) {
        try {
            byte[] image = thumbnailService.readThumbnail(projectId, slideId);
            if (image == null) return ResponseEntity.notFound().build();
            return ResponseEntity.ok()
                    .contentType(WEBP)
                    .cacheControl(CacheControl.noCache().cachePrivate())
                    .eTag(DigestUtils.md5DigestAsHex(image))
                    .body(image);
        } catch (RenderUnavailableException ex) {
            log.warn("缩略图渲染失败，项目 {}，幻灯片 {}", projectId, slideId, ex);
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).build();
        }
    }

    @GetMapping("/projects/{projectId}/attachments/{attachmentId:[a-zA-Z0-9_-]+}")
    public ResponseEntity<byte[]> readAttachment(@PathVariable String projectId, @PathVariable String attachmentId, @RequestParam(defaultValue = "thumbnail") String variant) {
        boolean original = switch (variant) {
            case "original" -> true;
            case "thumbnail" -> false;
            default -> throw invalid();
        };
        byte[] image = attachmentService.readAttachment(projectId, attachmentId, original);
        if (image == null) return ResponseEntity.notFound().build();
        return ResponseEntity.ok().contentType(original ? MediaType.IMAGE_PNG : WEBP).body(image);
    }

    @PostMapping(value = "/attachments/upload", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public IdVo uploadAttachment(@Valid @ModelAttribute UploadAttachmentDto request) throws IOException {
        return IdVo.builder()
                .id(attachmentService.uploadAttachment(request.getProjectId(), request.getFile().getBytes()))
                .build();
    }

    @PostMapping("/files/open")
    public void open(@Valid @RequestBody OpenProjectFileDto request) {
        deckService.openFile(request.getProjectId(), request.getFileName());
    }
}
