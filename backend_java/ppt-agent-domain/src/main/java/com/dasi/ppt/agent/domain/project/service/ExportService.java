package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.model.entity.ExportFile;
import com.dasi.ppt.agent.domain.project.model.entity.ExportSnapshot;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.domain.shared.service.SchemaService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.Collections;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class ExportService {

    private final IProjectRepository projectRepository;
    private final SchemaService schemaService;
    private final AssetService assetService;
    private final PathService pathService;
    private final TransactionTemplate transactionTemplate;
    private final ProjectWriteLock projectWriteLock;
    private final IRenderGateway renderGateway;

    public ExportFile exportProject(String projectId, ExportFormat exportFormat) {
        ExportSnapshot exportSnapshot;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            exportSnapshot = findRenderSnapshot(projectId, null);
        }
        if (exportSnapshot.getPages().isEmpty()) {
            throw invalid("导出至少需要一页幻灯片");
        }
        for (ExportSnapshot.Page page : exportSnapshot.getPages()) {
            if (page.getHtml() == null) {
                throw invalid("幻灯片缺少 HTML：" + page.getTitle());
            }
        }
        return renderGateway.exportDeck(exportSnapshot, exportFormat);
    }

    // 调用方持有项目锁。缩略图只读取目标页源码，同时保留完整页序以确定页码。
    public ExportSnapshot findRenderSnapshot(String projectId, String targetSlideId) {
        return transactionTemplate.execute(status -> {
            Project project = projectRepository.findProjectAppearance(projectId);
            if (project == null) return null;

            Meta meta = schemaService.decode(DeckFileType.META, projectRepository.readFile(pathService.metaPath(projectId)), Meta.class);
            Outline outline = schemaService.decode(DeckFileType.OUTLINE, projectRepository.readFile(pathService.outlinePath(projectId)), Outline.class);

            List<ExportSnapshot.Page> pages = new ArrayList<>();
            if (outline != null) {
                outline.forEachPage((page, chapterTitle) -> {
                    byte[] html = targetSlideId == null || targetSlideId.equals(page.getId())
                            ? projectRepository.readFile(pathService.htmlPath(projectId, page.getId())) : null;
                    pages.add(ExportSnapshot.Page.builder()
                            .slideId(page.getId())
                            .title(page.getTitle())
                            .chapterTitle(chapterTitle)
                            .html(html == null ? null : new String(html, StandardCharsets.UTF_8))
                            .build());
                });
            }
            Map<DecorationType, DecorationPosition> positions = new EnumMap<>(DecorationType.class);
            positions.put(DecorationType.SLIDE_PAGINATION, project.getSlidePaginationPosition());
            positions.put(DecorationType.DECK_TITLE, project.getDeckTitlePosition());
            positions.put(DecorationType.CHAPTER_TITLE, project.getChapterTitlePosition());
            positions.put(DecorationType.LOGO, project.getLogoPosition());
            return ExportSnapshot.builder()
                    .projectName(project.getTitle())
                    .deckTitle(meta == null ? null : meta.getTitle())
                    .pages(List.copyOf(pages))
                    .themeCss(project.getTheme() == null ? null : assetService.getAssetDetail(project.getTheme()).getContent())
                    .positions(Collections.unmodifiableMap(positions))
                    .logo(project.isLogoPresent() ? projectRepository.findProjectLogo(projectId) : null)
                    .build();
        });
    }
}
