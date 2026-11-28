package com.dasi.ppt.agent.domain.agent.service.interaction;

import com.dasi.ppt.agent.domain.agent.model.entity.PendingInteraction;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.Decision;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import lombok.Builder;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.node.ObjectNode;

import java.util.UUID;
import java.util.concurrent.CancellationException;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.function.BooleanSupplier;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;

@Service
@RequiredArgsConstructor
public class InteractionService {

    private final EventPublisher eventPublisher;
    private final ObjectMapper objectMapper;
    private final ConcurrentMap<CallKey, PendingPermission> permissions = new ConcurrentHashMap<>();

    public PermissionAnswer awaitCommandPermission(Run run, String toolCallId, String command, BooleanSupplier cancelled) {
        CallKey key = new CallKey(run.getId(), toolCallId);
        ObjectNode request = objectMapper.createObjectNode();
        request.putObject("permission").put("command", command);
        PendingInteraction form = PendingInteraction.builder()
                .runId(run.getId())
                .toolCallId(toolCallId)
                .type(InteractionType.PERMISSION)
                .version(UUID.randomUUID().toString())
                .request(request)
                .build();
        PendingPermission pending = new PendingPermission(form, new CompletableFuture<>());
        if (permissions.putIfAbsent(key, pending) != null) throw conflict(ErrorCode.INTERACTION_CLOSED);
        SseEvent.InteractionChanged event = SseEvent.InteractionChanged.builder()
                .runId(run.getId())
                .build();
        try {
            eventPublisher.publish(run.getSessionId(), event);
            while (!cancelled.getAsBoolean()) {
                try {
                    return pending.answer().get(100, TimeUnit.MILLISECONDS);
                } catch (TimeoutException waitingForUser) {
                    // Keep waiting until the user answers or the Run is cancelled.
                }
            }
            throw new CancellationException();
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new CancellationException();
        } catch (ExecutionException ex) {
            throw new IllegalStateException(ex.getCause());
        } finally {
            permissions.remove(key, pending);
            eventPublisher.publish(run.getSessionId(), event);
        }
    }

    public PendingInteraction getPending(String runId, String toolCallId) {
        return pending(new CallKey(runId, toolCallId)).form();
    }

    public void answerPermission(String runId, String toolCallId, String version, Decision decision, String feedback) {
        CallKey key = new CallKey(runId, toolCallId);
        PendingPermission pending = pending(key);
        if (!pending.form().getVersion().equals(version)) throw conflict(ErrorCode.STALE_INTERACTION);
        if (!permissions.remove(key, pending)) throw conflict(ErrorCode.INTERACTION_CLOSED);
        pending.answer().complete(PermissionAnswer.builder()
                .decision(decision)
                .feedback(decision == Decision.REFUSE ? feedback : null)
                .build());
    }

    private PendingPermission pending(CallKey key) {
        PendingPermission pending = permissions.get(key);
        if (pending == null) throw conflict(ErrorCode.INTERACTION_CLOSED);
        return pending;
    }

    @Builder
    public record PermissionAnswer(Decision decision, String feedback) {}

    private record CallKey(String runId, String toolCallId) {}
    private record PendingPermission(PendingInteraction form, CompletableFuture<PermissionAnswer> answer) {}
}
