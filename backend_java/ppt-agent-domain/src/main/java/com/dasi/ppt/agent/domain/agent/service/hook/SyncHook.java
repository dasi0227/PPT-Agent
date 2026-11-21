package com.dasi.ppt.agent.domain.agent.service.hook;

public interface SyncHook<E, R> {

    R execute(E event, R currentResult);
}
