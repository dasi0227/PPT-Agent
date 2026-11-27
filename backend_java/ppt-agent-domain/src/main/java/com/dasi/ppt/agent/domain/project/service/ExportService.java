package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.model.entity.*;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.service.SchemaService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.META_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.OUTLINE_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.SLIDE_HTML_FILE_NAME_FORMAT;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class ExportService {

    private final IProjectRepository projectRepository;
    private final SchemaService schemaService;
    private final AssetService assetService;
    private final IFileGateway fileGateway;
    private final PathService pathService;
    private final TransactionTemplate transactionTemplate;
    private final LockService lockService;
    private final IRenderGateway renderGateway;

    public ExportResult exportProject(String projectId, ExportFormat exportFormat) {
        ExportSnapshot exportSnapshot;
        synchronized (lockService.projectLock(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            exportSnapshot = findExportSnapshot(projectId);
        }
        if (exportSnapshot.getSlides().isEmpty()) {
            throw invalid("导出至少需要一页幻灯片");
        }
        for (ExportSnapshot.Slide slide : exportSnapshot.getSlides()) {
            if (slide.getHtml() == null) {
                throw invalid("幻灯片缺少 HTML：" + slide.getTitle());
            }
        }
        return renderGateway.export(exportSnapshot, exportFormat);
    }

    public ExportSnapshot findExportSnapshot(String projectId) {
        return transactionTemplate.execute(status -> {
            // 1. 拿到完整的项目对象
            Project project = projectRepository.findProjectAppearance(projectId);

            // 2. 得到 meta（用于装饰）和 outline（用于页面顺序）
            Path projectDirectory = pathService.projectDirectory(projectId);
            Meta meta = schemaService.decode(DeckFileType.META, fileGateway.readFile(projectDirectory.resolve(META_FILE_NAME)), Meta.class);
            Outline outline = schemaService.decode(DeckFileType.OUTLINE, fileGateway.readFile(projectDirectory.resolve(OUTLINE_FILE_NAME)), Outline.class);

            // 3. 得到 slides
            List<ExportSnapshot.Slide> slides = new ArrayList<>();
            if (outline != null) {
                appendSlides(slides, outline.getPages(), null, projectDirectory);
                for (Outline.Chapter chapter : outline.getChapters()) {
                    appendSlides(slides, chapter.getPages(), chapter.getTitle(), projectDirectory);
                    for (Outline.Section section : chapter.getSections()) {
                        appendSlides(slides, section.getPages(), chapter.getTitle(), projectDirectory);
                    }
                }
            }

            // 4. 得到装饰
            Map<DecorationType, DecorationPosition> positions = new EnumMap<>(DecorationType.class);
            positions.put(DecorationType.SLIDE_PAGINATION, project.getSlidePaginationPosition());
            positions.put(DecorationType.DECK_TITLE, project.getDeckTitlePosition());
            positions.put(DecorationType.CHAPTER_TITLE, project.getChapterTitlePosition());
            positions.put(DecorationType.LOGO, project.getLogoPosition());

            // 5. 返回完整内存结构
            return ExportSnapshot.builder()
                    .projectName(project.getTitle())
                    .deckTitle(meta == null ? null : meta.getTitle())
                    .slides(List.copyOf(slides))
                    .themeCss(project.getTheme() != null ? assetService.getAssetDetail(project.getTheme()).getContent() : null)
                    .positions(positions)
                    .logo(project.isLogoPresent() ? projectRepository.findProjectLogo(projectId) : null)
                    .build();
        });
    }

    private void appendSlides(List<ExportSnapshot.Slide> slides, List<Outline.Page> pages, String chapterTitle, Path projectDirectory) {
        for (Outline.Page page : pages) {
            String slideId = page.getId();
            byte[] html = fileGateway.readFile(projectDirectory.resolve(SLIDE_HTML_FILE_NAME_FORMAT.formatted(slideId)));
            ExportSnapshot.Slide slide = ExportSnapshot.Slide.builder()
                    .slideId(slideId)
                    .title(page.getTitle())
                    .chapterTitle(chapterTitle)
                    .html(html == null ? null : new String(html, StandardCharsets.UTF_8))
                    .build();
            slides.add(slide);
        }
    }
}
