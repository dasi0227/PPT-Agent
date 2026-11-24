package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.HashSet;
import java.util.List;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class ShortcutSettings {
    private String commandTrigger;
    private String referenceTrigger;
    private String componentTrigger;
    private String slideTrigger;
    private String snippetTrigger;
    private String previousSlide;
    private String nextSlide;
    private String zoomIn;
    private String zoomOut;
    private String toggleOverview;
    private String exportDeck;
    private String toggleView;
    private String toggleSource;
    private String switchTheme;
    private String submitRequest;
    private String selectSuggestion;

    private static final String TRIGGER_CHARACTERS = "/~!@#$%^&*>\\+";
    private static final Set<String> ALLOWED_FIRST_KEYS = Set.of("Primary", "Option", "Alt");
    private static final Set<String> DISALLOWED_SECOND_KEYS = Set.of(
            "Primary", "Option", "Alt", "AltLeft", "AltRight", "AltGraph",
            "Shift", "ShiftLeft", "ShiftRight", "Ctrl", "Control", "ControlLeft", "ControlRight",
            "Command", "Meta", "MetaLeft", "MetaRight", "OSLeft", "OSRight", "Fn"
    );
    private static final Set<String> RESERVED_BINDINGS = Set.of(
            "Primary+KeyA", "Primary+KeyC", "Primary+KeyV", "Primary+KeyX",
            "Primary+KeyZ", "Primary+KeyY", "Primary+KeyF", "Primary+KeyL",
            "Primary+KeyQ", "Primary+KeyW", "Primary+KeyR", "Primary+KeyN"
    );

    public void validate() {
        List<String> triggers = List.of(commandTrigger, referenceTrigger, componentTrigger, slideTrigger, snippetTrigger);
        Set<String> usedTriggers = new HashSet<>();
        for (String trigger : triggers) {
            if (trigger.length() != 1 || !TRIGGER_CHARACTERS.contains(trigger)) {
                throw invalid("触发符需为以下字符中的一个：" + TRIGGER_CHARACTERS);
            }
            if (!usedTriggers.add(trigger)) {
                throw invalid("触发符不能重复");
            }
        }

        List<String> bindings = List.of(
                selectSuggestion + "+Digit1", selectSuggestion + "+Digit2", selectSuggestion + "+Digit3",
                previousSlide, nextSlide, zoomIn, zoomOut, toggleOverview, exportDeck,
                toggleView, toggleSource, switchTheme, submitRequest
        );
        Set<String> usedBindings = new HashSet<>();
        for (String binding : bindings) {
            String[] keys = binding.split("\\+", -1);
            if (keys.length != 2 || !ALLOWED_FIRST_KEYS.contains(keys[0]) || !keys[1].matches("(?U)\\S+") || DISALLOWED_SECOND_KEYS.contains(keys[1])) {
                throw invalid("快捷键需为 Primary、Option 或 Alt 加一个普通键：" + binding);
            }
            String key = keys[0].equals("Option") ? "Alt+" + keys[1] : binding;
            if (RESERVED_BINDINGS.contains(key)) {
                throw invalid("不能使用保留快捷键：" + binding);
            }
            if (!usedBindings.add(key)) {
                throw invalid("快捷键重复：" + binding);
            }
        }
    }
}
