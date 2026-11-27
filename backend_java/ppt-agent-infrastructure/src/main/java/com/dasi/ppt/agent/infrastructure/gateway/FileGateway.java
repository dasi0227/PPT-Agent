package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

@Component
@RequiredArgsConstructor
public class FileGateway implements IFileGateway {

    private final PathService pathService;
    private final FileStorer fileStorer;

    @Override
    public byte[] read(String projectId, String relativePath) {
        return fileStorer.readFile(pathService.filePathByFileName(projectId, relativePath));
    }

    @Override
    public void create(String projectId, String relativePath, byte[] content) {
        fileStorer.createFile(pathService.filePathByFileName(projectId, relativePath), content);
    }

    @Override
    public void replace(String projectId, String relativePath, byte[] content) {
        fileStorer.applyFileChange(pathService.filePathByFileName(projectId, relativePath), content);
    }

    @Override
    public void delete(String projectId, String relativePath) {
        fileStorer.deleteFile(pathService.filePathByFileName(projectId, relativePath));
    }
}
