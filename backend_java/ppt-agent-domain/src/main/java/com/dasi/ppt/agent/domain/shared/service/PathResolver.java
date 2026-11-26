package com.dasi.ppt.agent.domain.shared.service;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.nio.file.Path;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.*;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;

@Component
public class PathResolver {

    private static final String DELETING = ".deleting-";
    private static final Set<String> PROJECT_DOCUMENTS = Set.of(META_FILE_NAME, CONSTRAINT_FILE_NAME, OUTLINE_FILE_NAME);
    private static final Pattern SLIDE_FILE = Pattern.compile("(sli_[A-Za-z0-9_-]+)\\.(?:json|html)");

    @Value("${ppt.root}")
    private Path dataRoot;

    public Path projectsRoot() {
        return dataRoot.resolve("projects").normalize();
    }

    public Path projectDirectory(String projectId) {
        return projectsRoot().resolve(projectId).normalize();
    }

    public Path metaPath(String projectId) {
        return projectDirectory(projectId).resolve(META_FILE_NAME).normalize();
    }

    public Path constraintPath(String projectId) {
        return projectDirectory(projectId).resolve(CONSTRAINT_FILE_NAME).normalize();
    }

    public Path outlinePath(String projectId) {
        return projectDirectory(projectId).resolve(OUTLINE_FILE_NAME).normalize();
    }

    public Path specPath(String projectId, String slideId) {
        return projectDirectory(projectId).resolve(slideId + ".json").normalize();
    }

    public Path htmlPath(String projectId, String slideId) {
        return projectDirectory(projectId).resolve(slideId + ".html").normalize();
    }

    public Path deletingDirectory(String projectId) {
        return projectsRoot().resolve(DELETING + projectId).normalize();
    }

    public String slideIdFromFileName(String name) {
        Matcher match = SLIDE_FILE.matcher(name);
        return match.matches() ? match.group(1) : null;
    }

    public boolean isDocumentFileName(String fileName) {
        return fileName != null && (PROJECT_DOCUMENTS.contains(fileName) || SLIDE_FILE.matcher(fileName).matches());
    }

    public boolean isDeletingDirectory(Path path) {
        return path.getFileName().toString().startsWith(DELETING);
    }

    public Path projectDirectoryFromDeletion(Path deletingDirectory) {
        String name = deletingDirectory.getFileName().toString();
        Path directory = projectDirectory(name.substring(DELETING.length()));
        if (!projectsRoot().equals(directory.getParent())) {
            throw illegal("项目删除暂存目录无效：" + deletingDirectory);
        }
        return directory;
    }

    public Path filePathByFileName(String projectId, String fileName) {
        return projectDirectory(projectId).resolve(fileName).normalize();
    }
}
