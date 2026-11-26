package com.dasi.ppt.agent.domain.shared.gateway;

public interface IFileGateway {

    byte[] read(String projectId, String relativePath);

    void create(String projectId, String relativePath, byte[] content);

    void replace(String projectId, String relativePath, byte[] content);

    void delete(String projectId, String relativePath);
}
