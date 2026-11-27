package com.dasi.ppt.agent.domain.shared.service;

import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.nio.file.Path;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.ATTACHMENT_PATH_PREFIX;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.THUMBNAIL_DIRECTORY_FORMAT;

@Slf4j
@Component
public class PathService {

    private static final String DELETING_DIRECTORY_PREFIX = ".deleting-";
    private static final String META = ".meta.json";
    private static final String CONSTRAINT = ".constraint.json";
    private static final String OUTLINE = ".outline.json";
    private static final String SPEC_SUFFIX = ".json";
    private static final String HTML_SUFFIX = ".html";
    private static final String ATTACHMENT_SUFFIX = ".png";
    private static final String THUMBNAIL_SUFFIX = ".webp";
    // 仅页面文件匹配第一个捕获组，内容为 slideId。
    public static final String PROJECT_FILE_NAME_REGEX = "\\.(?:meta|constraint|outline)\\.json|(sli_[A-Za-z0-9_-]+)\\.(?:json|html)";
    private static final Pattern PROJECT_FILE_NAME = Pattern.compile(PROJECT_FILE_NAME_REGEX);

    @Value("${ppt.root}")
    private Path root;

    public Path projectsRoot() {
        return root.resolve("projects");
    }

    public Path projectDirectory(String projectId) {
        return projectsRoot().resolve(projectId);
    }

    public Path deletingDirectory(String projectId) {
        return projectsRoot().resolve(DELETING_DIRECTORY_PREFIX + projectId);
    }

    public Path meta(String projectId) {
        return projectDirectory(projectId).resolve(META);
    }

    public Path constraint(String projectId) {
        return projectDirectory(projectId).resolve(CONSTRAINT);
    }

    public Path outline(String projectId) {
        return projectDirectory(projectId).resolve(OUTLINE);
    }

    public Path slideSpec(String projectId, String slideId) {
        return projectDirectory(projectId).resolve(slideId + SPEC_SUFFIX);
    }

    public Path slideHtml(String projectId, String slideId) {
        return projectDirectory(projectId).resolve(slideId + HTML_SUFFIX);
    }

    public Path attachmentOriginal(String projectId, String attachmentId) {
        return projectDirectory(projectId).resolve(ATTACHMENT_PATH_PREFIX + attachmentId + ATTACHMENT_SUFFIX);
    }

    public Path attachmentThumbnail(String projectId, String attachmentId) {
        return projectDirectory(projectId).resolve(ATTACHMENT_PATH_PREFIX + attachmentId + THUMBNAIL_SUFFIX);
    }

    public Path slideThumbnailDirectory(String projectId, String slideId) {
        return projectDirectory(projectId).resolve(THUMBNAIL_DIRECTORY_FORMAT.formatted(slideId));
    }

    public Path slideThumbnail(String projectId, String slideId, String renderHash) {
        return slideThumbnailDirectory(projectId, slideId).resolve(renderHash + THUMBNAIL_SUFFIX);
    }

    public boolean isProjectFile(Path projectDirectory, Path path) {
        return projectDirectory.toAbsolutePath().equals(path.toAbsolutePath().getParent())
                && PROJECT_FILE_NAME.matcher(path.getFileName().toString()).matches();
    }

    public String slideIdFromFileName(String fileName) {
        Matcher match = PROJECT_FILE_NAME.matcher(fileName);
        return match.matches() ? match.group(1) : null;
    }

    public String projectIdFromDeletingDirectory(String directoryName) {
        if (!directoryName.startsWith(DELETING_DIRECTORY_PREFIX)) return null;
        String projectId = directoryName.substring(DELETING_DIRECTORY_PREFIX.length());
        if (projectId.isEmpty() || projectId.equals(".") || projectId.equals("..")) {
            log.warn("跳过无效的项目删除暂存目录：{}", directoryName);
            return null;
        }
        return projectId;
    }
}
