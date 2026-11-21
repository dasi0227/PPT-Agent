package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.Deck;

public interface IDeckRepository {

    Deck readDeck(String projectId);
}
