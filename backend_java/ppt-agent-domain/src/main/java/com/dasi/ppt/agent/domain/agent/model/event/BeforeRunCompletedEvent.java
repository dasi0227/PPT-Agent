package com.dasi.ppt.agent.domain.agent.model.event;

import com.dasi.ppt.agent.domain.agent.model.Plan;
import com.dasi.ppt.agent.domain.agent.model.RunScope;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.result.PreviewAssessment;
import com.dasi.ppt.agent.domain.project.model.DeckChange;

import java.util.List;

public record BeforeRunCompletedEvent(
        RunMode mode,
        InstructionDecision intent,
        String candidateReply,
        Plan plan,
        RunScope scope,
        List<DeckChange> changes,
        List<PreviewAssessment> previews
) {
}
