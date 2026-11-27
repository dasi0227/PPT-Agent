package com.dasi.ppt.agent.domain.agent.service.hook.event;

import com.dasi.ppt.agent.domain.agent.model.entity.Plan;
import com.dasi.ppt.agent.domain.agent.model.entity.RunScope;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.entity.PreviewAssessment;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class BeforeRunCompletedEvent {
    private RunMode mode;
    private InstructionDecision intent;
    private String candidateReply;
    private Plan plan;
    private RunScope scope;
    private List<DeckChange> changes;
    private List<PreviewAssessment> previews;
}
