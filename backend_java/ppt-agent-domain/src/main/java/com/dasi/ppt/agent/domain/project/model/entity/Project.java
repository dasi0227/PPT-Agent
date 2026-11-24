package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.EnumSet;
import java.util.Map;
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
    private Map<DecorationType, DecorationPosition> positions;
    private boolean logoPresent;
    private long updatedAt;

    public void normalizeTitle() {
        title = title.strip();
        if (title.isEmpty() || title.codePointCount(0, title.length()) > 60) {
            throw invalid("项目名称需为 1–60 个字符");
        }
    }

    public void validateAppearance() {
        if (positions == null && theme == null) {
            throw invalid("需提供主题或装饰位置");
        }
        if (positions != null) {
            Set<DecorationPosition> used = EnumSet.noneOf(DecorationPosition.class);
            for (DecorationPosition position : positions.values()) {
                if (position != DecorationPosition.NONE && !used.add(position)) {
                    throw invalid("显示的装饰位置不能重叠");
                }
            }
        }
    }
}
