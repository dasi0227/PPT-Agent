package com.dasi.ppt.agent.domain.project.model;

/**
 * A missing base hash means the file did not exist when read.
 * A null replacement requests deletion. An empty byte array writes an empty file.
 */
public record DeckEditCandidate(
        ChangeTarget target,
        String baseHash,
        byte[] replacement
) {
}
