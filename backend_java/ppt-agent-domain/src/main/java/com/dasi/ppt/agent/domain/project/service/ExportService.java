package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.entity.ExportFile;
import com.dasi.ppt.agent.domain.project.model.entity.ExportSnapshot;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class ExportService {

    private final IProjectRepository projectRepository;
    private final DeckService deckService;
    private final ProjectWriteLock projectWriteLock;
    private final IRenderGateway renderGateway;

    public ExportFile exportProject(String projectId, ExportFormat format) {
        ExportSnapshot snapshot;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            snapshot = deckService.readRenderSnapshot(projectId, null);
            if (snapshot.getPages().isEmpty()) throw invalid("导出至少需要一页幻灯片");
        }
        for (ExportSnapshot.Page page : snapshot.getPages()) {
            if (page.getHtml() == null) throw invalid("幻灯片缺少 HTML：" + page.getTitle());
        }
        return renderGateway.exportDeck(snapshot, format);
    }
}
