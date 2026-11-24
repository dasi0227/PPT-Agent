package com.dasi.ppt.agent.domain.agent.repository;

import com.dasi.ppt.agent.domain.agent.model.entity.Message;
import com.dasi.ppt.agent.domain.agent.model.entity.Session;

import java.util.List;

public interface ISessionRepository {

    Session findSession(String sessionId);

    List<Message> listMessages(String sessionId);
}
