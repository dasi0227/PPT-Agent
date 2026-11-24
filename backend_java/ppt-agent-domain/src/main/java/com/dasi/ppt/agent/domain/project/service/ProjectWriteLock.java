package com.dasi.ppt.agent.domain.project.service;

import org.springframework.stereotype.Component;

import java.util.stream.IntStream;

@Component
public class ProjectWriteLock {

    // 预先创建好 64 个锁对象
    private final Object[] locks = IntStream.range(0, 64)
            .mapToObj(index -> new Object())
            .toArray();

    // 每个项目通过哈希取模拿到对应的锁对象
    public Object lockFor(String projectId) {
        return locks[Math.floorMod(projectId.hashCode(), locks.length)];
    }
}
