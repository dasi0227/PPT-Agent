package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import com.dasi.ppt.agent.domain.shared.util.ImageUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.nio.file.Path;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class AttachmentService {

    private final IFileGateway fileGateway;
    private final PathService pathService;
    private final IProjectRepository projectRepository;
    private final LockService lockService;

    public byte[] readAttachment(String projectId, String attachmentId, boolean original) {
        synchronized (lockService.projectLock(projectId)) {
            if (!projectRepository.existsProject(projectId)) return null;
            Path file = original
                    ? pathService.attachmentOriginal(projectId, attachmentId)
                    : pathService.attachmentThumbnail(projectId, attachmentId);
            return fileGateway.readFile(file);
        }
    }

    public String uploadAttachment(String projectId, byte[] png) {
        byte[] thumbnail = ImageUtil.compressPng(png);
        synchronized (lockService.projectLock(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            String id = IdUtil.genAttachmentId();
            Path originalFile = pathService.attachmentOriginal(projectId, id);
            fileGateway.createFile(originalFile, png);
            try {
                fileGateway.createFile(pathService.attachmentThumbnail(projectId, id), thumbnail);
            } catch (RuntimeException ex) {
                fileGateway.deleteFile(originalFile);
                throw ex;
            }
            return id;
        }
    }
}
