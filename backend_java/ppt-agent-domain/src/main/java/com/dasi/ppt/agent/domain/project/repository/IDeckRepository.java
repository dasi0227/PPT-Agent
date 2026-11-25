package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Deck;

import java.util.Set;

public interface IDeckRepository {

    Deck readDeck(String projectId);

    byte[] readFile(String projectId, String path);

    boolean isProjectBusy(String projectId);

    void saveManualFiles(String projectId, String operationId, String path, byte[] before,
                         byte[] after, Set<String> removedSlides, long updatedAt);

    void finishManualSave(String projectId, String operationId, boolean committed);
}
