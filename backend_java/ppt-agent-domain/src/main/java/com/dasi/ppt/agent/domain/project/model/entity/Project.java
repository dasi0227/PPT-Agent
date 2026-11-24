package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.EnumSet;
import java.util.List;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Project {
    private String id;
    private String title;
    private String theme;
    private DecorationPosition slidePaginationPosition;
    private DecorationPosition deckTitlePosition;
    private DecorationPosition sectionTitlePosition;
    private DecorationPosition logoPosition;
    private boolean logoPresent;
    private long updatedAt;

    public void normalize() {
        title = title.strip();
    }

    public void validatePositions() {
        Set<DecorationPosition> used = EnumSet.noneOf(DecorationPosition.class);
        for (DecorationPosition position : List.of(slidePaginationPosition, deckTitlePosition, sectionTitlePosition, logoPosition)) {
            if (position != DecorationPosition.NONE && !used.add(position)) {
                throw invalid("显示的装饰位置不能重叠");
            }
        }
    }
}
