package com.dasi.ppt.agent.domain.agent.service.hook;

public interface AsyncHook<E> {

    void execute(E event);
}
