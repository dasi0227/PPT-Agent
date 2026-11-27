package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.FileUndoMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.po.FileUndoPo;
import com.dasi.ppt.agent.infrastructure.persistence.po.ProjectPo;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.util.FileSystemUtils;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.util.List;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;

@Slf4j
@Repository
@RequiredArgsConstructor
public class ProjectRepository implements IProjectRepository {

    private final ProjectMapper projectMapper;
    private final SessionMapper sessionMapper;
    private final FileUndoMapper fileUndoMapper;
    private final IFileGateway fileGateway;
    private final PathService pathService;
    private final TransactionTemplate transaction;
    private final LockService lockService;

    public void recoverProjectDeletions() {
        Path projectsRoot = pathService.projectsRoot();
        if (Files.notExists(projectsRoot)) {
            return;
        }
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(projectsRoot)) {
            for (Path entry : entries) {
                String projectId = pathService.projectIdFromDeletingDirectory(entry.getFileName().toString());
                if (projectId == null) continue;
                if (projectMapper.existsProject(projectId)) {
                    // 恢复未提交删除的项目
                    Files.move(entry, pathService.projectDirectory(projectId), ATOMIC_MOVE);
                } else {
                    // 清理已删除项目的暂存目录
                    FileSystemUtils.deleteRecursively(entry);
                }
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void recoverPendingFiles() {
        for (String projectId : fileUndoMapper.listPendingProjectIds()) {
            synchronized (lockService.projectLock(projectId)) {
                try {
                    restoreFiles(projectId, fileUndoMapper.listFileUndo(projectId));
                    fileUndoMapper.deleteFileUndo(projectId);
                } catch (RuntimeException ex) {
                    log.error("文稿恢复失败，项目保留撤销记录：{}", projectId, ex);
                }
            }
        }
    }

    @Override
    public List<Project> listProjects() {
        return projectMapper.listProjects()
                .stream()
                .map(ProjectRepository::toEntity)
                .toList();
    }

    @Override
    public void insertProject(Project project) {
        projectMapper.insertProject(toPo(project));
    }

    @Override
    public int updateProjectTitle(String projectId, String title, long updatedAt) {
        return projectMapper.updateProjectTitle(projectId, title, updatedAt);
    }

    @Override
    public List<String> listProjectSessionIds(String projectId) {
        return sessionMapper.listProjectSessionIds(projectId);
    }

    @Override
    @Transactional(propagation = Propagation.MANDATORY)
    public int deleteProject(String projectId) {
        int deleted = projectMapper.deleteProject(projectId);
        if (deleted > 0) {
            // 1. 拿到两个路径
            Path projectDirectory = pathService.projectDirectory(projectId);
            Path deletingDirectory = pathService.deletingDirectory(projectId);
            if (Files.notExists(projectDirectory, NOFOLLOW_LINKS)) {
                return deleted;
            }
            // 2. 将项目目录原子重命名为暂存目录
            try {
                Files.move(projectDirectory, deletingDirectory, ATOMIC_MOVE);
            } catch (IOException ex) {
                throw new UncheckedIOException(ex);
            }
            // 3. 给事务注册一个回调方法
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override
                public void afterCompletion(int status) {
                    try {
                        if (status == STATUS_ROLLED_BACK) {
                            Files.move(deletingDirectory, projectDirectory, ATOMIC_MOVE);
                        } else if (status == STATUS_COMMITTED) {
                            FileSystemUtils.deleteRecursively(deletingDirectory);
                        }
                    } catch (IOException ex) {
                        log.error("项目删除暂存目录处理失败，将在下次启动时重试：{}", deletingDirectory, ex);
                    }
                }
            });
        }
        return deleted;
    }

    @Override
    public boolean hasPendingFileOperation(String projectId) {
        return Files.exists(pathService.deletingDirectory(projectId), NOFOLLOW_LINKS) || fileUndoMapper.hasFileUndo(projectId);
    }

    @Override
    public boolean isProjectBusy(String projectId) {
        return projectMapper.hasActiveDocumentOperation(projectId);
    }

    @Override
    public boolean existsProject(String projectId) {
        return projectMapper.existsProject(projectId);
    }

    @Override
    public Project findProjectAppearance(String projectId) {
        return toEntity(projectMapper.findProjectAppearance(projectId));
    }

    @Override
    public int updateProjectTheme(Project project) {
        return projectMapper.updateProjectTheme(project.getId(), project.getTheme(), project.getUpdatedAt());
    }

    @Override
    public int updateProjectPositions(Project project) {
        return projectMapper.updateProjectPositions(toPo(project));
    }

    @Override
    public byte[] findProjectLogo(String projectId) {
        ProjectPo po = projectMapper.findProjectLogo(projectId);
        return po == null ? null : po.getLogo();
    }

    @Override
    public int updateProjectLogo(Project project) {
        return projectMapper.updateProjectLogo(project.getId(), project.getLogo(), project.getUpdatedAt());
    }

    @Override
    @Transactional(propagation = Propagation.NEVER)
    public void commitFiles(String projectId, List<FileChange> fileChanges) {
        // 1. 插入撤回记录
        transaction.executeWithoutResult(status -> {
            for (FileChange fileChange : fileChanges) {
                fileUndoMapper.insertFileUndo(toPo(projectId, fileChange));
            }
        });
        try {
            // 2. 逐一更改磁盘
            for (FileChange fileChange : fileChanges) {
                applyFileContent(projectId, fileChange.getFileName(), fileChange.getAfter());
            }
            // 3. 删除撤回记录
            transaction.executeWithoutResult(status -> {
                projectMapper.updateProjectTime(projectId, System.currentTimeMillis());
                fileUndoMapper.deleteFileUndo(projectId);
            });
        } catch (RuntimeException ex) {
            try {
                // 4. 如果找不到撤回记录，说明文件更改已经完成，保留磁盘更改
                List<FileUndoPo> fileUndoList = fileUndoMapper.listFileUndo(projectId);
                if (fileUndoList.isEmpty()) {
                    return;
                }
                // 5. 验证整批文件名后逐一还原文件
                restoreFiles(projectId, fileUndoList);
                // 6. 清除记录
                fileUndoMapper.deleteFileUndo(projectId);
            } catch (RuntimeException recoveryFailure) {
                ex.addSuppressed(recoveryFailure);
            }
            throw ex;
        }
    }

    private void restoreFiles(String projectId, List<FileUndoPo> fileUndoList) {
        Path directory = pathService.projectDirectory(projectId);
        for (FileUndoPo fileUndo : fileUndoList) {
            String fileName = fileUndo.getFileName();
            try {
                Path path = directory.resolve(fileName);
                if (!pathService.isProjectFile(directory, path) || !fileName.equals(path.getFileName().toString())) {
                    throw illegal("撤销记录中的文稿文件名无效：" + fileName);
                }
            } catch (InvalidPathException ex) {
                throw illegal("撤销记录中的文稿文件名无效：" + fileName);
            }
        }
        for (FileUndoPo fileUndo : fileUndoList) {
            applyFileContent(projectId, fileUndo.getFileName(), fileUndo.getBeforeContent());
        }
    }

    private void applyFileContent(String projectId, String fileName, byte[] content) {
        Path file = pathService.projectDirectory(projectId).resolve(fileName).normalize();
        if (content == null) {
            fileGateway.deleteFile(file);
        } else {
            fileGateway.replaceFile(file, content);
        }
    }

    private static FileUndoPo toPo(String projectId, FileChange change) {
        return FileUndoPo.builder()
                .projectId(projectId)
                .fileName(change.getFileName())
                .beforeContent(change.getBefore())
                .build();
    }

    private static Project toEntity(ProjectPo po) {
        if (po == null) return null;
        return Project.builder()
                .id(po.getId())
                .title(po.getTitle())
                .theme(po.getTheme())
                .slidePaginationPosition(position(po.getSlidePaginationPosition()))
                .deckTitlePosition(position(po.getDeckTitlePosition()))
                .chapterTitlePosition(position(po.getChapterTitlePosition()))
                .logoPosition(position(po.getProjectLogoPosition()))
                .logoPresent(po.isLogoPresent())
                .updatedAt(po.getUpdatedAt())
                .build();
    }

    private static ProjectPo toPo(Project project) {
        return ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .slidePaginationPosition(value(project.getSlidePaginationPosition()))
                .deckTitlePosition(value(project.getDeckTitlePosition()))
                .chapterTitlePosition(value(project.getChapterTitlePosition()))
                .projectLogoPosition(value(project.getLogoPosition()))
                .updatedAt(project.getUpdatedAt())
                .build();
    }

    private static DecorationPosition position(String value) {
        return value == null ? null : DecorationPosition.fromValue(value);
    }

    private static String value(DecorationPosition position) {
        return position == null ? null : position.getValue();
    }
}
