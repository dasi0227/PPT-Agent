package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.project.model.ChangeTarget;
import com.dasi.ppt.agent.domain.project.model.DeckChange;

import java.util.Map;

public record RunChangeSet(Map<ChangeTarget, DeckChange> changes) {
}
