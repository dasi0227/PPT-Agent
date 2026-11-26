package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import com.dasi.ppt.agent.domain.shared.util.ImageUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class AttachmentService {

    private final IFileGateway fileGateway;
    private final IProjectRepository projectRepository;
    private final ProjectWriteLock projectWriteLock;

    public byte[] readAttachment(String projectId, String attachmentId, boolean original) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) return null;
            return fileGateway.read(projectId, ".runtime/attachments/" + attachmentId + (original ? ".png" : ".webp"));
        }
    }

    public String uploadAttachment(String projectId, byte[] png) {
        byte[] thumbnail = ImageUtil.pngThumbnail(png);
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            String id = IdUtil.genAttachmentId();
            String path = ".runtime/attachments/" + id;
            fileGateway.create(projectId, path + ".png", png);
            try {
                fileGateway.create(projectId, path + ".webp", thumbnail);
            } catch (RuntimeException ex) {
                fileGateway.delete(projectId, path + ".png");
                throw ex;
            }
            return id;
        }
    }
}
