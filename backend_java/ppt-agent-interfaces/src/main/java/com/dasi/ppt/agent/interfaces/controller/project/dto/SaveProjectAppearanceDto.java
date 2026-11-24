package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.interfaces.controller.NotBlankIfPresent;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonSetter;
import com.fasterxml.jackson.annotation.Nulls;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Getter
// 走无参构造与字段绑定，使 Nulls.FAIL 仅作用于显式传入的 null。
@AllArgsConstructor(onConstructor_ = @JsonCreator(mode = JsonCreator.Mode.DISABLED))
@NoArgsConstructor
public final class SaveProjectAppearanceDto {
    @NotBlank
    private String projectId;
    @NotBlankIfPresent
    @JsonSetter(nulls = Nulls.FAIL)
    private String themeId;
    @Valid
    @JsonSetter(nulls = Nulls.FAIL)
    private DecorationPositionsDto positions;

    public Project toProject() {
        return Project.builder()
                .id(projectId)
                .theme(themeId)
                .positions(positions == null ? null : Map.of(
                        DecorationType.SLIDE_PAGINATION, positions.slidePagination,
                        DecorationType.DECK_TITLE, positions.deckTitle,
                        DecorationType.SECTION_TITLE, positions.sectionTitle,
                        DecorationType.LOGO, positions.logo))
                .build();
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class DecorationPositionsDto {
        @NotNull
        private DecorationPosition slidePagination;
        @NotNull
        private DecorationPosition deckTitle;
        @NotNull
        private DecorationPosition sectionTitle;
        @NotNull
        private DecorationPosition logo;
    }
}
