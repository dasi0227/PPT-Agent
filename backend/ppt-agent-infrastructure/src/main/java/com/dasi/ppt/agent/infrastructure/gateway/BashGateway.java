package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.shared.exception.BashExecutionException;
import com.dasi.ppt.agent.domain.shared.gateway.IBashGateway;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.infrastructure.properties.SandboxProperties;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CancellationException;
import java.util.concurrent.TimeUnit;
import java.util.function.BooleanSupplier;

@Component
@RequiredArgsConstructor
public class BashGateway implements IBashGateway {

    private final SandboxProperties sandboxProperties;
    private final ObjectMapper objectMapper;

    @Value("${PPT_BASH_WORKER:bash-worker/worker.mjs}")
    private Path worker;
    @Value("${PPT_BASH_NODE:node}")
    private String node;
    @Value("${ppt.root}")
    private Path applicationRoot;
    @Value("${ppt.database}")
    private Path database;

    @Override
    public BashResult execute(Path projectDirectory, String command, boolean writable, BooleanSupplier cancellationRequested) {
        checkCancellation(cancellationRequested);
        try {
            Path replyFile = Files.createTempFile("ppt-bash-reply-", ".json");
            try {
                runWorker(projectDirectory, command, writable, replyFile, cancellationRequested);
                WorkerReply reply = objectMapper.readValue(replyFile, WorkerReply.class);
                if ("RUN_CANCELED".equals(reply.code())) throw new CancellationException();
                if (!"".equals(reply.code()) && !"COMMAND_EXIT_NONZERO".equals(reply.code())) {
                    throw new BashExecutionException(reply.code(), reply.message(), null);
                }
                checkCancellation(cancellationRequested);
                return BashResult.builder()
                        .exitCode(reply.exitCode())
                        .stdout(reply.stdout())
                        .stderr(reply.stderr())
                        .durationMs(reply.durationMs())
                        .changes(reply.changes().stream()
                                .map(Change::toFileChange)
                                .toList())
                        .build();
            } finally {
                Files.deleteIfExists(replyFile);
            }
        } catch (IOException | JacksonException ex) {
            throw new BashExecutionException("COMMAND_NOT_AVAILABLE", "无法调用命令 worker 或读取执行结果", ex);
        }
    }

    private void runWorker(Path project, String command, boolean writable, Path replyFile, BooleanSupplier cancelled) throws IOException {
        Path script = worker.toAbsolutePath().normalize();
        ProcessBuilder builder = new ProcessBuilder(node, script.toString(), "--project")
                .directory(script.getParent().toFile())
                .redirectOutput(replyFile.toFile())
                .redirectError(ProcessBuilder.Redirect.INHERIT);
        Map<String, String> env = builder.environment();
        env.clear();
        for (String key : List.of("PATH", "SystemRoot", "ProgramData", "ProgramFiles", "PATHEXT",
                "USERPROFILE", "LOCALAPPDATA", "APPDATA", "TEMP", "TMP")) {
            String value = System.getenv(key);
            if (value != null) env.put(key, value);
        }
        env.put("LC_ALL", "en_US.UTF-8");
        long timeout = sandboxProperties.getExecution().getTimeout().toSeconds();
        Map<String, Object> settings = Map.of(
                "read_roots", sandboxProperties.getAllowedFilepaths().stream().map(Path::toString).toList(),
                "network", Map.of("allowed_domains", sandboxProperties.getAllowedDomains()),
                "execution", Map.of("timeout_seconds", timeout, "max_output_bytes", sandboxProperties.getExecution().getMaxOutputBytes()));
        byte[] request = objectMapper.writeValueAsBytes(Map.of(
                "original_root", project.toAbsolutePath().normalize().toString(), "command", command, "writable", writable,
                "host_home", System.getProperty("user.home"), "document_pattern", PathService.PROJECT_FILE_NAME_REGEX,
                "protected_read_paths", List.of(applicationRoot.toAbsolutePath().normalize().toString(),
                        database.toAbsolutePath().normalize().toString(), Path.of(".env").toAbsolutePath().normalize().toString(),
                        replyFile.toString()), "settings", settings));

        Process process = builder.start();
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(timeout + 35);
        try (OutputStream pipe = process.getOutputStream()) {
            pipe.write(request);
            pipe.write('\n');
            pipe.flush();
            while (!process.waitFor(100, TimeUnit.MILLISECONDS)) {
                checkCancellation(cancelled);
                if (System.nanoTime() >= deadline) throw new BashExecutionException("COMMAND_TIMEOUT", "命令执行超时", null);
            }
            checkCancellation(cancelled);
            if (process.exitValue() != 0) throw new IOException("命令 worker 异常退出：" + process.exitValue());
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new CancellationException();
        } finally {
            // Closing stdin requests cancellation, including command and workspace cleanup.
            if (process.isAlive()) {
                process.onExit().completeOnTimeout(null, 8, TimeUnit.SECONDS).join();
                if (process.isAlive()) process.destroyForcibly().onExit().join();
            }
        }
    }

    private void checkCancellation(BooleanSupplier cancelled) {
        if (Thread.currentThread().isInterrupted() || cancelled.getAsBoolean()) throw new CancellationException();
    }

    private record WorkerReply(String code, String message, int exitCode, long durationMs,
                               String stdout, String stderr, List<Change> changes) {}

    private record Change(String fileName, byte[] before, byte[] after) {
        FileChange toFileChange() {
            return FileChange.builder()
                    .fileName(fileName)
                    .before(before)
                    .after(after)
                    .build();
        }
    }
}
