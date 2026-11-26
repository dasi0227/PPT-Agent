package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.FileUndoPo;

import java.util.List;

public interface FileUndoMapper {

    List<String> listPendingProjectIds();

    boolean hasFileUndo(String projectId);

    List<FileUndoPo> listFileUndo(String projectId);

    void insertFileUndo(FileUndoPo file);

    void deleteFileUndo(String projectId);
}
