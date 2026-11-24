package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.project.model.ChangeTarget;
import com.dasi.ppt.agent.domain.project.model.DeckChange;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RunChangeSet {
    private Map<ChangeTarget, DeckChange> changes;
}
