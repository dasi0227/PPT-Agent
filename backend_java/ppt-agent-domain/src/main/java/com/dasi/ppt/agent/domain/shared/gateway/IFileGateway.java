package com.dasi.ppt.agent.domain.shared.gateway;

import java.util.List;

public interface IFileGateway {

    byte[] read(String projectId, String relativePath);

    void create(String projectId, String relativePath, byte[] content);

    void replace(String projectId, String relativePath, byte[] content);

    List<String> list(String projectId, String relativeDirectory);

    void delete(String projectId, String relativePath);
}
