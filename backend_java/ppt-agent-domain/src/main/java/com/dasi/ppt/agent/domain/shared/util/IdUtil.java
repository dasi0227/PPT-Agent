package com.dasi.ppt.agent.domain.shared.util;

import lombok.AccessLevel;
import lombok.NoArgsConstructor;

import java.security.SecureRandom;
import java.util.UUID;

@NoArgsConstructor(access = AccessLevel.PRIVATE)
public final class IdUtil {

    private static final SecureRandom RANDOM = new SecureRandom();
    private static final String ID_CHARACTERS = "abcdefghijklmnopqrstuvwxyz0123456789";
    private static final int ID_LENGTH = 8;

    public final static String genOperationId() {
        return genUuid();
    }

    public final static String genSessionId() {
        return genRandomId();
    }

    public static String genProjectId() {
        return genRandomId();
    }

    public static String genAssetId() {
        return genRandomId();
    }

    public static String genTagId() {
        return genRandomId();
    }

    public static String genModelId() {
        return genRandomId();
    }

    private static String genUuid() {
        return UUID.randomUUID().toString().replace("-", "");
    }

    private static String genUuid(String prefix) {
        return prefix + genUuid();
    }

    private static String genRandomId() {
        StringBuilder id = new StringBuilder(ID_LENGTH);
        for (int i = 0; i < ID_LENGTH; i++) {
            int index = RANDOM.nextInt(ID_CHARACTERS.length());
            id.append(ID_CHARACTERS.charAt(index));
        }
        return id.toString();
    }

    private static String genRandomId(String prefix) {
        return prefix + genRandomId();
    }
}
