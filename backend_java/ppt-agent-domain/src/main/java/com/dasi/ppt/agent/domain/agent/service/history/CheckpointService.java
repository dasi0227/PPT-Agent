package com.dasi.ppt.agent.domain.agent.service.history;

import com.dasi.ppt.agent.domain.agent.model.CheckpointSnapshot;
import com.dasi.ppt.agent.domain.agent.model.entity.Session;
import com.dasi.ppt.agent.domain.agent.repository.ICheckpointRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ProjectWriteLock;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.function.BiConsumer;
import java.util.function.Supplier;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;

@Service
@RequiredArgsConstructor
public class CheckpointService {

    private final ICheckpointRepository checkpointRepository;
    private final ISessionRepository sessionRepository;
    private final DeckService deckService;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
    private final IProjectRepository projectRepository;

    // 在锁内读取目标快照；首次回退可在这里采集返回快照，随 restoreHistory 一起提交。
    public void restoreCheckpointFiles(String projectId, String sessionId, long expectedRevision, Supplier<CheckpointSnapshot> readCheckpoint,
                                       BiConsumer<CheckpointSnapshot, List<FileChange>> restoreHistory) {
        boolean contentChanged;
        synchronized (projectWriteLock.lockFor(projectId)) {
            Session session = sessionRepository.findSession(sessionId);
            if (session == null || !projectId.equals(session.getProjectId()) || session.getRevision() != expectedRevision) {
                throw conflict(ErrorCode.STALE_REVISION);
            }
            if (session.isHasActiveCommands() || projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            CheckpointSnapshot snapshot = readCheckpoint.get();
            Map<String, byte[]> replacements = new LinkedHashMap<>(snapshot.getFiles());
            for (String name : projectRepository.listDocumentFileNames(projectId)) {
                replacements.putIfAbsent(name, null);
            }
            contentChanged = deckService.commitDocumentFiles(projectId, replacements, Map.of(), changes -> {
                restoreHistory.accept(snapshot, changes);
                if (checkpointRepository.advanceSessionRevision(sessionId, expectedRevision) != 1) {
                    throw conflict(ErrorCode.STALE_REVISION);
                }
            });
        }
        if (contentChanged) deckService.notifyContentChanged(projectId);
        eventPublisher.publish(sessionId, SseEvent.Reset.builder().build());
    }
}
