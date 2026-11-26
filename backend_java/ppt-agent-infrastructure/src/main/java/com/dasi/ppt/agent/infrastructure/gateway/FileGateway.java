package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.List;

@Component
@RequiredArgsConstructor
public class FileGateway implements IFileGateway {

    private final PathResolver pathResolver;
    private final FileStorer fileStorer;

    @Override
    public byte[] read(String projectId, String relativePath) {
        throw new UnsupportedOperationException();
    }

    @Override
    public void create(String projectId, String relativePath, byte[] content) {
        throw new UnsupportedOperationException();
    }

    @Override
    public void replace(String projectId, String relativePath, byte[] content) {
        throw new UnsupportedOperationException();
    }

    @Override
    public List<String> list(String projectId, String relativeDirectory) {
        throw new UnsupportedOperationException();
    }

    @Override
    public void delete(String projectId, String relativePath) {
        throw new UnsupportedOperationException();
    }
}
