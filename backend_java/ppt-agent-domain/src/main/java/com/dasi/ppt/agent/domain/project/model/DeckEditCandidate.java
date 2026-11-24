package com.dasi.ppt.agent.domain.project.model;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

/**
 * A missing base hash means the file did not exist when read.
 * A null replacement requests deletion. An empty byte array writes an empty file.
 */
@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DeckEditCandidate {
    private ChangeTarget target;
    private String baseHash;
    private byte[] replacement;
}
