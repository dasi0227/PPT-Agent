package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class InstructionReference {
    private InstructionReferenceType type;
    private String assetId;
    private String slideId;
    private String attachmentId;
    private String originalName;
    private DomSelection selection;
}
