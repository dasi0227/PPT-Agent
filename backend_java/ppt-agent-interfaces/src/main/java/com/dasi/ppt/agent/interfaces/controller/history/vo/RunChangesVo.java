package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffRowType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffType;
import com.dasi.ppt.agent.domain.project.model.enumeration.OutlineNodeType;
import com.fasterxml.jackson.annotation.JsonUnwrapped;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RunChangesVo {
    private List<RunChangeDetailVo> items;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ChangeTargetVo {
        private ChangePart part;
        private String slideId;
        private String path;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class RunChangeDetailVo {
        private ChangeTargetVo target;
        private ChangeStatus status;
        @JsonUnwrapped
        private ChangeDiffVo diff;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ChangeDiffVo {
        private DiffType type;
        private List<FieldDiffVo> fields;
        private List<OutlineDiffGroupVo> groups;
        private List<TextDiffHunkVo> hunks;
        private String reason;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class FieldDiffVo {
        private String field;
        private String label;
        private List<FieldDiffRowVo> rows;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class FieldDiffRowVo {
        private DiffRowType type;
        private String value;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class OutlineDiffGroupVo {
        private List<OutlineDiffRowVo> rows;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class OutlineDiffRowVo {
        private DiffRowType type;
        private OutlineNodeType node;
        private int depth;
        private String title;
        private Integer order;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class TextDiffHunkVo {
        private int oldStart;
        private int oldCount;
        private int newStart;
        private int newCount;
        private List<TextDiffRowVo> rows;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class TextDiffRowVo {
        private DiffRowType type;
        private String text;
        private Integer oldLine;
        private Integer newLine;
    }
}
