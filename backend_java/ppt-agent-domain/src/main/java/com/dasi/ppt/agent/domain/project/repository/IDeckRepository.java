package com.dasi.ppt.agent.domain.project.repository;

import java.nio.file.Path;
import java.util.Map;
import java.util.Set;

public interface IDeckRepository {

    byte[] readFile(Path path);

    Map<String, byte[]> readOrphanSlideFiles(String projectId, Set<String> slideIds);

    boolean isProjectBusy(String projectId);

    void saveManualFiles(String projectId, String operationId, Path path, byte[] before, byte[] after, Map<String, byte[]> orphanFiles, long updatedAt);

    void finishManualSave(String projectId, String operationId, boolean committed);
}
