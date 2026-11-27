package com.dasi.ppt.agent.domain.shared.service;

import org.springframework.stereotype.Service;

import java.util.stream.IntStream;

@Service
public class LockService {

    // 缩略图处理会在持有页面锁时获取项目锁，两组锁必须独立。
    private final Object[] projectLocks = IntStream.range(0, 64).mapToObj(index -> new Object()).toArray();
    private final Object[] slideLocks = IntStream.range(0, 64).mapToObj(index -> new Object()).toArray();

    public Object projectLock(String projectId) {
        return projectLocks[Math.floorMod(projectId.hashCode(), projectLocks.length)];
    }

    public Object slideLock(String projectId, String slideId) {
        String key = projectId + "/" + slideId;
        return slideLocks[Math.floorMod(key.hashCode(), slideLocks.length)];
    }
}
