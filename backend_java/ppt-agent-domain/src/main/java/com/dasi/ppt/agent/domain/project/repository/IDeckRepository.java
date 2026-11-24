package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Deck;

public interface IDeckRepository {

    Deck readDeck(String projectId);
}
