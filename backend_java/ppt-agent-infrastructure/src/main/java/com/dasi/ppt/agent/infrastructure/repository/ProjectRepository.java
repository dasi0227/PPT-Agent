package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.FileUndoMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.FileUndoPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Propagation;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.util.List;
import java.util.stream.Stream;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
import static java.nio.file.LinkOption.NOFOLLOW_LINKS;

@Slf4j
@Repository
@RequiredArgsConstructor
public class ProjectRepository implements IProjectRepository, SmartInitializingSingleton {

    private final ProjectMapper projectMapper;
    private final SessionMapper sessionMapper;
    private final FileUndoMapper fileUndoMapper;
    private final FileStorer fileStorer;
    private final PathResolver pathResolver;
    private final TransactionTemplate transaction;
    private final ProjectWriteLock projectWriteLock;

    @Override
    public void afterSingletonsInstantiated() {
        for (String projectId : fileUndoMapper.listPendingProjectIds()) {
            synchronized (projectWriteLock.lockFor(projectId)) {
                try {
                    restoreFiles(projectId);
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
    public void updateProjectTime(String projectId, long updatedAt) {
        projectMapper.updateProjectTime(projectId, updatedAt);
    }

    @Override
    public List<String> listProjectSessionIds(String projectId) {
        return sessionMapper.listProjectSessionIds(projectId);
    }

    @Override
    public int deleteProject(String projectId) {
        int deleted = projectMapper.deleteProject(projectId);
        if (deleted > 0) {
            fileStorer.stageProjectDeletion(projectId);
        }
        return deleted;
    }

    @Override
    public boolean hasPendingFileOperation(String projectId) {
        return fileStorer.hasDeletingDirectory(projectId) || fileUndoMapper.hasFileUndo(projectId);
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
    public int updateProjectTheme(String projectId, String themeId, long updatedAt) {
        return projectMapper.updateProjectTheme(projectId, themeId, updatedAt);
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
    public int updateProjectLogo(String projectId, byte[] logo, long updatedAt) {
        return projectMapper.updateProjectLogo(projectId, logo, updatedAt);
    }

    @Override
    public byte[] readFile(Path path) {
        return fileStorer.readFile(path);
    }

    @Override
    public List<String> listDocumentFileNames(String projectId) {
        try (Stream<Path> files = Files.list(pathResolver.projectDirectory(projectId))) {
            return files.filter(path -> Files.isRegularFile(path, NOFOLLOW_LINKS))
                    .map(path -> path.getFileName().toString())
                    .filter(pathResolver::isDocumentFileName)
                    .sorted()
                    .toList();
        } catch (NoSuchFileException ex) {
            return List.of();
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    @Override
    @Transactional(propagation = Propagation.NEVER)
    public void commitFiles(String projectId, List<FileChange> changes, Runnable saveBusiness) {
        if (changes.isEmpty()) {
            transaction.executeWithoutResult(status -> saveBusiness.run());
            return;
        }
        transaction.executeWithoutResult(status -> {
            for (FileChange change : changes) {
                fileUndoMapper.insertFileUndo(toPo(projectId, change));
            }
        });
        try {
            for (FileChange change : changes) {
                fileStorer.applyFileChange(pathResolver.filePathByFileName(projectId, change.getFileName()), change.getAfter());
            }
            transaction.executeWithoutResult(status -> {
                saveBusiness.run();
                fileUndoMapper.deleteFileUndo(projectId);
            });
        } catch (RuntimeException ex) {
            // 提交结果不确定时重读 SQLite。若已提交，查询结果为空，磁盘上的新版应保留。
            try {
                if (!restoreFiles(projectId)) return;
            } catch (RuntimeException recoveryFailure) {
                ex.addSuppressed(recoveryFailure);
            }
            throw ex;
        }
    }

    private boolean restoreFiles(String projectId) {
        List<FileUndoPo> files = fileUndoMapper.listFileUndo(projectId);
        if (files.isEmpty()) return false;
        for (FileUndoPo file : files) {
            if (!pathResolver.isDocumentFileName(file.getFileName())) {
                throw illegal("撤销记录中的文稿文件名无效：" + file.getFileName());
            }
        }
        for (FileUndoPo file : files) {
            Path path = pathResolver.filePathByFileName(projectId, file.getFileName());
            fileStorer.applyFileChange(path, file.getBeforeContent());
        }
        fileUndoMapper.deleteFileUndo(projectId);
        return true;
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
