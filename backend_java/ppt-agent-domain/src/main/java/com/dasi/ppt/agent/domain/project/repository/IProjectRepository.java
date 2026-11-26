package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.model.entity.Project;

import java.nio.file.Path;
import java.util.List;

public interface IProjectRepository {

    List<Project> listProjects();

    void insertProject(Project project);

    int updateProjectTitle(String projectId, String title, long updatedAt);

    void updateProjectTime(String projectId, long updatedAt);

    List<String> listProjectSessionIds(String projectId);

    int deleteProject(String projectId);

    // 项目删除暂存目录或文稿撤销记录仍然存在。
    boolean hasPendingFileOperation(String projectId);

    // 项目有活跃的 Run 或 commit 命令。
    boolean isProjectBusy(String projectId);

    boolean existsProject(String projectId);

    Project findProjectAppearance(String projectId);

    int updateProjectTheme(String projectId, String themeId, long updatedAt);

    int updateProjectPositions(Project project);

    byte[] findProjectLogo(String projectId);

    int updateProjectLogo(String projectId, byte[] logo, long updatedAt);

    byte[] readFile(Path path);

    List<String> listDocumentFileNames(String projectId);

    // 调用方持有项目写锁。业务写入与删除撤销记录在同一 SQLite 事务完成。
    void commitFiles(String projectId, List<FileChange> changes, Runnable saveBusiness);
}
