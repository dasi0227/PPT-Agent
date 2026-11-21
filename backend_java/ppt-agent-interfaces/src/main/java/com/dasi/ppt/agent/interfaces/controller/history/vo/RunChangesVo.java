package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffRowType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DiffType;
import com.dasi.ppt.agent.domain.project.model.enumeration.OutlineNodeType;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonUnwrapped;

import java.util.List;

public record RunChangesVo(List<RunChangeDetailVo> items) {

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ChangeTargetVo(
            ChangePart part,
            String slideId,
            String path
    ) {
    }

    public record RunChangeDetailVo(
            ChangeTargetVo target,
            ChangeStatus status,
            @JsonUnwrapped ChangeDiffVo diff
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record ChangeDiffVo(
            DiffType type,
            List<FieldDiffVo> fields,
            List<OutlineDiffGroupVo> groups,
            List<TextDiffHunkVo> hunks,
            String reason
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record FieldDiffVo(
            String field,
            String label,
            List<FieldDiffRowVo> rows
    ) {
    }

    public record FieldDiffRowVo(DiffRowType type, String value) {
    }

    public record OutlineDiffGroupVo(List<OutlineDiffRowVo> rows) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record OutlineDiffRowVo(
            DiffRowType type,
            OutlineNodeType node,
            int depth,
            String title,
            Integer order
    ) {
    }

    public record TextDiffHunkVo(
            int oldStart,
            int oldCount,
            int newStart,
            int newCount,
            List<TextDiffRowVo> rows
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record TextDiffRowVo(
            DiffRowType type,
            String text,
            Integer oldLine,
            Integer newLine
    ) {
    }
}
