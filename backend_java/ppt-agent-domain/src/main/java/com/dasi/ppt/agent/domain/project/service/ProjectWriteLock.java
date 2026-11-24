package com.dasi.ppt.agent.domain.project.service;

import org.springframework.stereotype.Component;

import java.util.stream.IntStream;

@Component
public class ProjectWriteLock {

    private final Object[] locks = IntStream.range(0, 64)
            .mapToObj(index -> new Object())
            .toArray();

    public void withLock(String projectId, Runnable action) {
        synchronized (locks[Math.floorMod(projectId.hashCode(), locks.length)]) {
            action.run();
        }
    }
}
