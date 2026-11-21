package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;

public record InstructionReference(
        InstructionReferenceType type,
        String assetId,
        String slideId,
        String attachmentId,
        String originalName,
        DomSelection selection
) {
}
