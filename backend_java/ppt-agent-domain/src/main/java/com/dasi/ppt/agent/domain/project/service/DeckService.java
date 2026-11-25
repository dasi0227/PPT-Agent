package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.CONSTRAINT_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.META_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.OUTLINE_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class DeckService {

    private final TransactionTemplate transactionTemplate;
    private final DeckDocuments deckDocuments;
    private final IProjectRepository projectRepository;
    private final IDeckRepository deckRepository;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;

    public Deck readDeck(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return deckRepository.readDeck(projectId);
    }

    public String saveMeta(String projectId, String baseHash, Meta meta) {
        String fileName = META_FILE_NAME;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readSaveBase(projectId, fileName, baseHash);
            byte[] after = deckDocuments.encode(ChangePart.META, meta);
            return saveDocument(projectId, fileName, before, after, Set.of());
        }
    }

    public String saveOutline(String projectId, String baseHash, Outline outline) {
        String filePath = OUTLINE_FILE_NAME;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readSaveBase(projectId, filePath, baseHash);
            byte[] after = deckDocuments.encode(ChangePart.OUTLINE, outline);
            Outline previous = deckDocuments.decode(ChangePart.OUTLINE, before, Outline.class);
            Set<String> removedSlides = getRemovedSlides(previous, outline);
            return saveDocument(projectId, filePath, before, after, removedSlides);
        }
    }

    public String saveConstraint(String projectId, String baseHash, Constraint constraint) {
        String filePath = CONSTRAINT_FILE_NAME;
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readSaveBase(projectId, filePath, baseHash);
            byte[] after = deckDocuments.encode(ChangePart.CONSTRAINT, constraint);
            return saveDocument(projectId, filePath, before, after, Set.of());
        }
    }

    public String saveSlideSpec(String projectId, String slideId, String baseHash, Spec spec) {
        String filePath = slideId + ".json";
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readSaveBase(projectId, filePath, baseHash);
            byte[] after = deckDocuments.encode(ChangePart.SPEC, spec);
            requireSlideInOutline(projectId, slideId);
            return saveDocument(projectId, filePath, before, after, Set.of());
        }
    }

    public String saveSlideHtml(String projectId, String slideId, String baseHash, String html) {
        String filePath = slideId + ".html";
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readSaveBase(projectId, filePath, baseHash);
            byte[] after = html.getBytes(StandardCharsets.UTF_8);
            requireSlideInOutline(projectId, slideId);
            return saveDocument(projectId, filePath, before, after, Set.of());
        }
    }

    private void requireSlideInOutline(String projectId, String slideId) {
        byte[] outlineBytes = deckRepository.readFile(projectId, OUTLINE_FILE_NAME);
        Outline outline = deckDocuments.decode(ChangePart.OUTLINE, outlineBytes, Outline.class);
        if (outline == null || !outline.slideIds().contains(slideId)) {
            throw invalid("幻灯片不在当前大纲中");
        }
    }

    private byte[] readSaveBase(String projectId, String filePath, String baseHash) {
        byte[] before = deckRepository.readFile(projectId, filePath);
        if (!Objects.equals(baseHash, HashUtil.contentHash(before))) {
            throw conflict(ErrorCode.CONTENT_CONFLICT);
        }
        return before;
    }

    private Set<String> getRemovedSlides(Outline previous, Outline current) {
        Map<String, Outline.Chapter> chapters = new HashMap<>();
        Set<String> sectionIds = new HashSet<>();
        Set<String> ids = new HashSet<>();
        for (Outline.Chapter chapter : current.getChapters()) {
            addNode(ids, chapter.getId());
            chapters.put(chapter.getId(), chapter);
            if (!chapter.getSlides().isEmpty() && !chapter.getSections().isEmpty()) {
                throw invalid("章不能同时包含直属幻灯片和节");
            }
            chapter.getSlides().forEach(page -> addNode(ids, page.getId()));
            for (Outline.Section section : chapter.getSections()) {
                addNode(ids, section.getId());
                sectionIds.add(section.getId());
                section.getSlides().forEach(page -> addNode(ids, page.getId()));
            }
        }
        if (previous == null) return Set.of();
        for (Outline.Chapter chapter : previous.getChapters()) {
            Outline.Chapter next = chapters.get(chapter.getId());
            if (next == null && (!chapter.getSlides().isEmpty() || !chapter.getSections().isEmpty())) {
                throw invalid("不能删除非空章");
            }
            for (Outline.Section section : chapter.getSections()) {
                if (section.getSlides().isEmpty() || sectionIds.contains(section.getId())) continue;
                Set<String> direct = new HashSet<>();
                next.getSlides().forEach(page -> direct.add(page.getId()));
                if (chapter.getSections().size() != 1 || !section.getSlides().stream().allMatch(page -> direct.contains(page.getId()))) {
                    throw invalid("删除非空节时须将唯一节的全部幻灯片保留在原章下");
                }
            }
        }
        Set<String> removed = previous.slideIds();
        removed.removeAll(ids);
        return removed;
    }

    private static void addNode(Set<String> ids, String id) {
        if (!ids.add(id)) {
            throw invalid("大纲节点 ID 重复：" + id);
        }
    }

    private String saveDocument(String projectId, String fileName, byte[] before, byte[] after, Set<String> removedSlides) {
        // 判断哈希值是否一致
        String hash = HashUtil.contentHash(after);
        if (Arrays.equals(before, after)) return hash;
        String operationId = IdUtil.genOperationId();
        try {
            transactionTemplate.executeWithoutResult(status -> deckRepository.saveManualFiles(projectId, operationId, fileName, before, after, removedSlides, System.currentTimeMillis()));
        } catch (RuntimeException ex) {
            deckRepository.finishManualSave(projectId, operationId, false);
            throw ex;
        }
        // 确保提交
        deckRepository.finishManualSave(projectId, operationId, true);
        // 发送内容变更通知到会话
        SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                .areas(List.of(ProjectChangeArea.CONTENT))
                .build();
        projectRepository.listProjectSessionIds(projectId).forEach(id -> eventPublisher.publish(id, event));
        return hash;
    }
}
