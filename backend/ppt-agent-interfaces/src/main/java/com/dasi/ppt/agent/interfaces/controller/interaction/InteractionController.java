package com.dasi.ppt.agent.interfaces.controller.interaction;

import com.dasi.ppt.agent.domain.agent.model.entity.PendingInteraction;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.interfaces.controller.interaction.dto.AnswerPermissionDto;
import com.dasi.ppt.agent.interfaces.controller.interaction.vo.InteractionFormVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;
import tools.jackson.databind.ObjectMapper;

@RestController
@RequiredArgsConstructor
public class InteractionController {

    private final InteractionService interactionService;
    private final ObjectMapper objectMapper;

    @GetMapping("/runs/{runId}/interactions/{toolCallId}")
    public InteractionFormVo getInteraction(@PathVariable String runId, @PathVariable String toolCallId) {
        PendingInteraction pending = interactionService.getPending(runId, toolCallId);
        return InteractionFormVo.builder()
                .type(pending.getType())
                .version(pending.getVersion())
                .request(objectMapper.treeToValue(pending.getRequest(), InteractionFormVo.InteractionRequestVo.class))
                .build();
    }

    @PostMapping("/interactions/permission")
    public void answerPermission(@Valid @RequestBody AnswerPermissionDto request) {
        interactionService.answerPermission(request.getRunId(), request.getToolCallId(), request.getVersion(),
                request.getDecision(), request.getFeedback());
    }
}
