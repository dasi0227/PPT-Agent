package com.dasi.ppt.agent.domain.agent.service.tool.builtin;

import com.dasi.ppt.agent.domain.agent.model.entity.ToolCall;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PermissionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.agent.repository.ISessionRepository;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService;
import com.dasi.ppt.agent.domain.agent.service.interaction.InteractionService.PermissionAnswer;
import com.dasi.ppt.agent.domain.agent.service.run.RunState;
import com.dasi.ppt.agent.domain.agent.service.tool.Tool;
import com.dasi.ppt.agent.domain.agent.service.tool.result.RunCommandResult;
import com.dasi.ppt.agent.domain.shared.constant.SystemConstant;
import com.dasi.ppt.agent.domain.shared.exception.ShellExecutionException;
import com.dasi.ppt.agent.domain.shared.gateway.IShellGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import lombok.RequiredArgsConstructor;
import org.springframework.ai.tool.definition.ToolDefinition;
import org.springframework.core.io.ResourceLoader;
import org.springframework.stereotype.Component;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;
import tools.jackson.databind.node.ObjectNode;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.util.List;
import java.util.Map;
import java.util.function.BooleanSupplier;

@Component("run_command")
@RequiredArgsConstructor
public class RunCommandTool implements Tool {

    private final IShellGateway shellGateway;
    private final ISessionRepository sessionRepository;
    private final PathService pathService;
    private final InteractionService interactionService;
    private final ResourceLoader resourceLoader;
    private final ObjectMapper objectMapper;

    @Override
    public ToolDefinition getDefinition(ToolName name) {
        String shell = SystemConstant.IS_WINDOWS ? "Windows PowerShell 5.1" : "Bash";
        String description = prompt("md", shell);
        String command = objectMapper.readTree(prompt("json", shell)).get("command").asString();
        Map<String, Object> schema = Map.of("type", "object", "additionalProperties", false,
                "required", List.of("command"), "properties", Map.of("command",
                        Map.of("type", "string", "minLength", 1, "pattern", "\\S", "description", command)));
        return ToolDefinition.builder()
                .name(name.getValue())
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
    public RunCommandResult execute(ToolCall call, RunState runState) {
        Run run = runState.getRun();
        if (run.getMode() != RunMode.CREATE) {
            return RunCommandResult.builder()
                    .status(ToolResultStatus.BLOCKED)
                    .text("当前模式只读，无法执行本机命令")
                    .build();
        }
        String command;
        try {
            command = objectMapper.readTree(call.getArguments()).path("command").stringValue();
        } catch (JacksonException invalidArguments) {
            command = null;
        }
        if (command == null || command.isBlank() || command.indexOf(0) >= 0) {
            return RunCommandResult.builder()
                    .status(ToolResultStatus.FAILED)
                    .code("INVALID_ARGUMENT")
                    .text("command 必须是非空且不含 NUL 的字符串")
                    .build();
        }
        BooleanSupplier cancelled = () -> Thread.currentThread().isInterrupted()
                || run.getStatus() == RunStatus.CANCELLING || run.getStatus() == RunStatus.CANCELLED;
        PermissionAnswer answer = interactionService.awaitCommandPermission(
                run, call.getToolCallId(), command, cancelled);
        ObjectNode request = objectMapper.createObjectNode();
        request.putObject("permission").put("command", command);
        RunCommandResult.RunCommandResultBuilder<?, ?> result = RunCommandResult.builder()
                .request(request)
                .decision(answer.decision())
                .feedback(answer.feedback());
        if (answer.decision() == PermissionDecision.DENY) {
            return result
                    .status(ToolResultStatus.BLOCKED)
                    .text("用户拒绝执行命令")
                    .build();
        }
        String projectId = sessionRepository.findProjectId(run.getSessionId());
        if (projectId == null) {
            return result
                    .status(ToolResultStatus.FAILED)
                    .code("SESSION_NOT_FOUND")
                    .text("会话不存在")
                    .build();
        }
        try {
            IShellGateway.CommandResult output = shellGateway.execute(pathService.projectDirectory(projectId), command, cancelled);
            boolean success = output.getExitCode() == 0;
            return result
                    .status(success ? ToolResultStatus.COMPLETED : ToolResultStatus.FAILED)
                    .code(success ? null : "COMMAND_EXIT_NONZERO")
                    .exitCode(output.getExitCode())
                    .stdout(output.getStdout())
                    .stderr(output.getStderr())
                    .build();
        } catch (ShellExecutionException ex) {
            return result
                    .status(ToolResultStatus.FAILED)
                    .code(ex.getCode())
                    .text(ex.getMessage())
                    .build();
        }
    }
}
