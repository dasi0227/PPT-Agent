package com.dasi.ppt.agent.domain.agent.service.context;

import com.dasi.ppt.agent.domain.agent.model.entity.InstructionReference;
import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.repository.IRunRepository;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.AskPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.ChatPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.CreatePromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.context.assembler.PlanPromptAssembler;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.RequiredArgsConstructor;
import org.springframework.ai.tool.definition.ToolDefinition;
import org.springframework.stereotype.Service;

import java.util.List;

@Service
@RequiredArgsConstructor
public class ContextService {

    private final IRunRepository runRepository;
    private final ISessionRepository sessionRepository;
    private final DeckService deckService;
    private final AssetService assetService;
    private final ChatPromptAssembler chatPromptAssembler;
    private final AskPromptAssembler askPromptAssembler;
    private final PlanPromptAssembler planPromptAssembler;
    private final CreatePromptAssembler createPromptAssembler;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Turn {
        private RunState state;
        private List<Message> history;
        private List<InstructionReference> references;
        private Deck deck;
        private List<ToolDefinition> tools;
    }
}
