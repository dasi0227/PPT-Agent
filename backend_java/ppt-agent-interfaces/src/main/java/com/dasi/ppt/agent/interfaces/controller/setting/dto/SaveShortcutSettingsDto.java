package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveShortcutSettingsDto {
    private static final String TRIGGER = "[/~!@#$%^&*>\\\\+]";
    private static final String MODIFIER = "(?:Primary|Option|Alt)";
    private static final String MODIFIER_KEY = "(?:Primary|Option|Alt(?:Left|Right|Graph)?|Shift(?:Left|Right)?|"
            + "Ctrl|Control(?:Left|Right)?|Command|Meta(?:Left|Right)?|OSLeft|OSRight|Fn)";
    private static final String SHORTCUT = "(?U)" + MODIFIER + "\\+(?!" + MODIFIER_KEY + "$)[^+\\s]+";

    @NotNull(message = "command_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "command_trigger 格式无效")
    private String commandTrigger;
    @NotNull(message = "reference_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "reference_trigger 格式无效")
    private String referenceTrigger;
    @NotNull(message = "component_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "component_trigger 格式无效")
    private String componentTrigger;
    @NotNull(message = "slide_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "slide_trigger 格式无效")
    private String slideTrigger;
    @NotNull(message = "snippet_trigger 不能为空")
    @Pattern(regexp = TRIGGER, message = "snippet_trigger 格式无效")
    private String snippetTrigger;
    @NotNull(message = "previous_slide 不能为空")
    @Pattern(regexp = SHORTCUT, message = "previous_slide 格式无效")
    private String previousSlide;
    @NotNull(message = "next_slide 不能为空")
    @Pattern(regexp = SHORTCUT, message = "next_slide 格式无效")
    private String nextSlide;
    @NotNull(message = "zoom_in 不能为空")
    @Pattern(regexp = SHORTCUT, message = "zoom_in 格式无效")
    private String zoomIn;
    @NotNull(message = "zoom_out 不能为空")
    @Pattern(regexp = SHORTCUT, message = "zoom_out 格式无效")
    private String zoomOut;
    @NotNull(message = "toggle_overview 不能为空")
    @Pattern(regexp = SHORTCUT, message = "toggle_overview 格式无效")
    private String toggleOverview;
    @NotNull(message = "export_deck 不能为空")
    @Pattern(regexp = SHORTCUT, message = "export_deck 格式无效")
    private String exportDeck;
    @NotNull(message = "toggle_view 不能为空")
    @Pattern(regexp = SHORTCUT, message = "toggle_view 格式无效")
    private String toggleView;
    @NotNull(message = "toggle_source 不能为空")
    @Pattern(regexp = SHORTCUT, message = "toggle_source 格式无效")
    private String toggleSource;
    @NotNull(message = "switch_theme 不能为空")
    @Pattern(regexp = SHORTCUT, message = "switch_theme 格式无效")
    private String switchTheme;
    @NotNull(message = "submit_request 不能为空")
    @Pattern(regexp = SHORTCUT, message = "submit_request 格式无效")
    private String submitRequest;
    @NotNull(message = "select_suggestion 不能为空")
    @Pattern(regexp = MODIFIER, message = "select_suggestion 格式无效")
    private String selectSuggestion;

    public ShortcutSettings toEntity() {
        return ShortcutSettings.builder()
                .commandTrigger(commandTrigger)
                .referenceTrigger(referenceTrigger)
                .componentTrigger(componentTrigger)
                .slideTrigger(slideTrigger)
                .snippetTrigger(snippetTrigger)
                .previousSlide(previousSlide)
                .nextSlide(nextSlide)
                .zoomIn(zoomIn)
                .zoomOut(zoomOut)
                .toggleOverview(toggleOverview)
                .exportDeck(exportDeck)
                .toggleView(toggleView)
                .toggleSource(toggleSource)
                .switchTheme(switchTheme)
                .submitRequest(submitRequest)
                .selectSuggestion(selectSuggestion)
                .build();
    }
}
