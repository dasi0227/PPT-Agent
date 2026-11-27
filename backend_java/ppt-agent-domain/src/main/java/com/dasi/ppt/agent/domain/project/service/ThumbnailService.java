package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.service.SchemaService;
import com.dasi.ppt.agent.domain.shared.util.ImageUtil;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.EnumMap;
import java.util.Map;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.META_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.OUTLINE_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.SLIDE_HTML_FILE_NAME_FORMAT;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.THUMBNAIL_DIRECTORY_FORMAT;

@Slf4j
@Service
@RequiredArgsConstructor
public class ThumbnailService {

    private final IFileGateway fileGateway;
    private final PathService pathService;
    private final IRenderGateway renderGateway;
    private final IProjectRepository projectRepository;
    private final AssetService assetService;
    private final SchemaService schemaService;
    private final TransactionTemplate transactionTemplate;
    private final LockService lockService;

    public byte[] readThumbnail(String projectId, String slideId) {
        // 1. 给 slide 上锁
        synchronized (lockService.slideLock(projectId, slideId)) {
            IRenderGateway.RenderMaterial renderMaterial;
            Path thumbnailDirectory;
            String thumbnailName;
            Path thumbnailFile;
            // 2. 读取页面内容，按渲染哈希查找缓存
            synchronized (lockService.projectLock(projectId)) {
                renderMaterial = getRenderMaterial(projectId, slideId);
                if (renderMaterial == null) return null;
                thumbnailDirectory = pathService.projectDirectory(projectId).resolve(THUMBNAIL_DIRECTORY_FORMAT.formatted(slideId));
                thumbnailName = renderGateway.renderHash(renderMaterial) + ".webp";
                thumbnailFile = thumbnailDirectory.resolve(thumbnailName);
                byte[] cached = fileGateway.readFile(thumbnailFile);
                if (cached != null) return cached;
            }
            // 3. 执行渲染并压缩
            byte[] screenshot = renderGateway.render(renderMaterial).getScreenshot();
            byte[] thumbnail = ImageUtil.compressPng(screenshot);

            synchronized (lockService.projectLock(projectId)) {
                // 4. 保存前确保项目还在
                if (!projectRepository.existsProject(projectId)) return null;
                // 5. 先保存新的
                fileGateway.replaceFile(thumbnailFile, thumbnail);
                // 6. 再删除旧的
                for (String fileName : fileGateway.listFileNames(thumbnailDirectory)) {
                    if (fileName.endsWith(".webp") && !fileName.equals(thumbnailName)) {
                        fileGateway.deleteFile(thumbnailDirectory.resolve(fileName));
                    }
                }
            }
            return thumbnail;
        }
    }

    private IRenderGateway.RenderMaterial getRenderMaterial(String projectId, String slideId) {
        return transactionTemplate.execute(status -> {
            Project project = projectRepository.findProjectAppearance(projectId);
            if (project == null) return null;
            Path projectDirectory = pathService.projectDirectory(projectId);
            byte[] outlineBytes = fileGateway.readFile(projectDirectory.resolve(OUTLINE_FILE_NAME));
            Outline outline = schemaService.decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);
            if (outline == null) return null;
            Outline.PagePosition position = outline.findPagePosition(slideId);
            if (position == null) return null;
            byte[] html = fileGateway.readFile(projectDirectory.resolve(SLIDE_HTML_FILE_NAME_FORMAT.formatted(slideId)));
            if (html == null) return null;
            byte[] metaBytes = fileGateway.readFile(projectDirectory.resolve(META_FILE_NAME));
            Meta meta = schemaService.decode(DeckFileType.META, metaBytes, Meta.class);
            Map<DecorationType, DecorationPosition> decorations = new EnumMap<>(DecorationType.class);
            decorations.put(DecorationType.SLIDE_PAGINATION, project.getSlidePaginationPosition());
            decorations.put(DecorationType.DECK_TITLE, project.getDeckTitlePosition());
            decorations.put(DecorationType.CHAPTER_TITLE, project.getChapterTitlePosition());
            decorations.put(DecorationType.LOGO, project.getLogoPosition());
            return IRenderGateway.RenderMaterial.builder()
                    .slideId(slideId)
                    .html(new String(html, StandardCharsets.UTF_8))
                    .themeCss(project.getTheme() == null ? null : assetService.getAssetDetail(project.getTheme()).getContent())
                    .positions(decorations)
                    .deckTitle(meta == null ? null : meta.getTitle())
                    .chapterTitle(position.getChapterTitle())
                    .pageNumber(position.getPageNumber())
                    .totalPages(position.getTotalPages())
                    .logo(project.isLogoPresent() ? projectRepository.findProjectLogo(projectId) : null)
                    .build();
        });
    }
}
