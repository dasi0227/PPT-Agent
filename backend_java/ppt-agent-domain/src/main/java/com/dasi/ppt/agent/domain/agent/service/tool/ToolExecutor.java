package com.dasi.ppt.agent.domain.agent.service.tool;

import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.project.model.entity.DeckEditCandidate;
import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.Consumer;
import java.util.function.Supplier;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Component
@RequiredArgsConstructor
public class ToolExecutor {

    private final ToolGuard toolGuard;
    private final ToolRegistry toolRegistry;
    private final IRunRepository runRepository;
    private final DeckService deckService;
    private final InteractionService interactionService;
    private final EventPublisher eventPublisher;
    private final IProjectRepository projectRepository;
    private final ProjectWriteLock projectWriteLock;

    // 回调在锁内取得仍获授权的定稿，等待人工审批须在调用本方法前完成。
    public void commitApprovedFiles(String projectId, Supplier<List<DeckEditCandidate>> readApprovedCandidates,
                                    Consumer<List<FileChange>> saveToolResult) {
        boolean contentChanged;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            if (projectRepository.hasPendingFileOperation(projectId)) throw conflict(ErrorCode.PROJECT_BUSY);
            Map<String, byte[]> replacements = new LinkedHashMap<>();
            Map<String, String> baseHashes = new LinkedHashMap<>();
            for (DeckEditCandidate candidate : readApprovedCandidates.get()) {
                String name = candidate.getTarget().getPath();
                if (replacements.containsKey(name)) throw invalid("同一批修改重复指定文稿：" + name);
                replacements.put(name, candidate.getReplacement());
                baseHashes.put(name, candidate.getBaseHash());
            }
            contentChanged = deckService.commitDocumentFiles(projectId, replacements, baseHashes, saveToolResult);
        }
        if (contentChanged) deckService.notifyContentChanged(projectId);
    }
}
