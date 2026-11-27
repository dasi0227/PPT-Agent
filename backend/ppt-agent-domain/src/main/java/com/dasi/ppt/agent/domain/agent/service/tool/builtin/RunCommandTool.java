package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.shared.gateway.IBashGateway;
import com.dasi.ppt.agent.domain.shared.constant.SystemConstant;
import com.dasi.ppt.agent.domain.agent.model.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.result.ToolResult;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.AgentTool;
import com.dasi.ppt.agent.domain.project.service.DeckService;
import lombok.RequiredArgsConstructor;
import org.springframework.ai.tool.definition.ToolDefinition;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;

@Component("run_command")
@RequiredArgsConstructor
public class RunCommandTool implements AgentTool {

    private final IBashGateway bashGateway;
    private final DeckService deckService;
    private final InteractionService interactionService;
    private final ResourceLoader resourceLoader;
    private final ObjectMapper objectMapper;

    @Override
    public ToolName getName() {
        return ToolName.RUN_COMMAND;
    }

    @Override
    public ToolDefinition getDefinition() {
        String shell = SystemConstant.IS_WINDOWS ? "Windows PowerShell 5.1" : "Bash";
        String description = prompt("md", shell);
        String command = objectMapper.readTree(prompt("json", shell)).get("command").asString();
        Map<String, Object> schema = Map.of("type", "object", "additionalProperties", false,
                "required", List.of("command"), "properties", Map.of("command",
                        Map.of("type", "string", "minLength", 1, "pattern", "\\S", "description", command)));
        return ToolDefinition.builder()
                .name(getName().getValue())
                .description(SystemConstant.IS_WINDOWS
                        ? description + " Write HTML and JSON as UTF-8; PowerShell 5.1 redirection defaults to UTF-16LE."
                        : description)
                .inputSchema(objectMapper.writeValueAsString(schema))
                .build();
    }

    private String prompt(String extension, String shell) {
        try {
            return resourceLoader.getResource("classpath:prompts/tool/run_command." + extension)
                    .getContentAsString(StandardCharsets.UTF_8).strip().replace("{{shell}}", shell);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    @Override
    public ToolResult execute(ToolCall call, RunState runState) {
        throw new UnsupportedOperationException();
    }
}
