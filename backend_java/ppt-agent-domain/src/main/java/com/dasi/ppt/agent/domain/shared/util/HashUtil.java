package com.dasi.ppt.agent.domain.shared.util;

import lombok.AccessLevel;
import lombok.NoArgsConstructor;

import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.HexFormat;

@NoArgsConstructor(access = AccessLevel.PRIVATE)
public final class HashUtil {

    private final static String HASH_ALGORITHM = "sha256:";

    public static String contentHash(byte[] bytes) {
        return bytes == null ? null : HASH_ALGORITHM + sha256(bytes);
    }

    public static String sha256(byte[] bytes) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(bytes));
        } catch (NoSuchAlgorithmException ex) {
            throw new IllegalStateException(ex);
        }
    }
}
