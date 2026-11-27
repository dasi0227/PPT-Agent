package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.constant.SystemConstant;
import com.dasi.ppt.agent.domain.shared.exception.ShellExecutionException;
import com.dasi.ppt.agent.domain.shared.gateway.IShellGateway;
import com.dasi.ppt.agent.infrastructure.properties.ShellProperties;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.Base64;
import java.util.concurrent.CancellationException;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.FutureTask;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.BooleanSupplier;

@Slf4j
@Component
@RequiredArgsConstructor
public class ShellGateway implements IShellGateway {

    private final ShellProperties shellProperties;

    @Override
    public CommandResult execute(Path workingDirectory, String command, BooleanSupplier cancellationRequested) {
        checkCancellation(cancellationRequested);
        ProcessBuilder shell = shell(command).directory(workingDirectory.toFile());
        try {
            Process process = shell.start();
            try {
                try (OutputStream stdin = process.getOutputStream()) {
                    if (!SystemConstant.IS_WINDOWS) stdin.write(command.getBytes(StandardCharsets.UTF_8));
                }
                AtomicLong outputBytes = new AtomicLong();
                FutureTask<String> stdout = readOutput(process.getInputStream(), outputBytes);
                FutureTask<String> stderr = readOutput(process.getErrorStream(), outputBytes);
                long deadline = System.nanoTime() + shellProperties.getTimeout().toNanos();
                while (process.isAlive() || !stdout.isDone() || !stderr.isDone()) {
                    checkCancellation(cancellationRequested);
                    checkOutputLimit(outputBytes.get());
                    if (System.nanoTime() - deadline >= 0) {
                        throw new ShellExecutionException("COMMAND_TIMEOUT", "命令执行超时", null);
                    }
                    TimeUnit.MILLISECONDS.sleep(50);
                }
                checkCancellation(cancellationRequested);
                checkOutputLimit(outputBytes.get());
                return CommandResult.builder()
                        .exitCode(process.exitValue())
                        .stdout(stdout.get())
                        .stderr(stderr.get())
                        .build();
            } finally {
                if (process.isAlive()) {
                    try {
                        process.descendants().forEach(ProcessHandle::destroyForcibly);
                    } catch (RuntimeException ex) {
                        log.warn("停止命令子进程失败，pid={}", process.pid(), ex);
                    } finally {
                        process.destroyForcibly();
                    }
                }
            }
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new CancellationException();
        } catch (IOException | ExecutionException ex) {
            throw new ShellExecutionException("COMMAND_NOT_AVAILABLE", "无法启动 Shell 或读取命令输出", ex);
        }
    }

    private ProcessBuilder shell(String command) {
        if (SystemConstant.IS_WINDOWS) {
            Path powershell = Path.of(System.getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe");
            String script = "[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)\n" + command;
            String encoded = Base64.getEncoder().encodeToString(script.getBytes(StandardCharsets.UTF_16LE));
            return new ProcessBuilder(powershell.toString(), "-NoLogo", "-NoProfile", "-NonInteractive",
                    "-OutputFormat", "Text", "-EncodedCommand", encoded);
        }
        // Pass the script as UTF-8 independently of the JVM's native argument encoding.
        // Bash consumes the script before exec so the command itself sees EOF on stdin.
        return new ProcessBuilder("/bin/bash", "-c", "IFS= read -r -d '' script; exec /bin/bash -c \"$script\"");
    }

    private FutureTask<String> readOutput(InputStream stream, AtomicLong outputBytes) {
        FutureTask<String> reader = new FutureTask<>(() -> {
            try (stream) {
                ByteArrayOutputStream output = new ByteArrayOutputStream();
                byte[] buffer = new byte[8192];
                int count;
                while ((count = stream.read(buffer)) != -1) {
                    checkOutputLimit(outputBytes.addAndGet(count));
                    output.write(buffer, 0, count);
                }
                return output.toString(StandardCharsets.UTF_8);
            }
        });
        Thread thread = new Thread(reader, "shell-output");
        thread.setDaemon(true);
        thread.start();
        return reader;
    }

    private void checkOutputLimit(long bytes) {
        if (bytes > shellProperties.getMaxOutputBytes()) {
            throw new ShellExecutionException("COMMAND_OUTPUT_LIMIT", "命令输出超过上限", null);
        }
    }

    private void checkCancellation(BooleanSupplier cancellationRequested) {
        if (Thread.currentThread().isInterrupted() || cancellationRequested.getAsBoolean()) {
            throw new CancellationException();
        }
    }
}
