package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.attribute.BasicFileAttributes;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.IS_MAC;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.IS_WINDOWS;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.failed;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Component
@RequiredArgsConstructor
public class PlatformGateway implements IPlatformGateway {

    private static final String MAC_PICK_APPLICATION = """
            try
                set appFile to choose file of type {"com.apple.application-bundle"} with prompt "选择打开文件的应用" default location (path to applications folder)
                return POSIX path of appFile
            on error number -128
                return ""
            end try
            """;
    private static final String WINDOWS_PICK_APPLICATION = """
            Add-Type -AssemblyName System.Windows.Forms
            $dialog = New-Object System.Windows.Forms.OpenFileDialog
            $dialog.Filter = 'Applications (*.exe)|*.exe'
            if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
                [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
                [Console]::Write($dialog.FileName)
            }
            """;
    private static final String WINDOWS_OPEN_DOCUMENT = """
            $info = New-Object System.Diagnostics.ProcessStartInfo
            $info.FileName = $env:PPT_AGENT_DOCUMENT
            $info.UseShellExecute = $true
            $info.Verb = 'open'
            [void][System.Diagnostics.Process]::Start($info)
            """;

    private final PathService pathService;

    @Override
    public Path pickApplication() {
        try {
            String selected;
            if (IS_MAC) {
                selected = run(new ProcessBuilder("osascript", "-e", MAC_PICK_APPLICATION));
                if (selected.endsWith("\n")) {
                    selected = selected.substring(0, selected.length() - 1);
                }
            } else if (IS_WINDOWS) {
                selected = run(powerShell(WINDOWS_PICK_APPLICATION));
            } else {
                throw failed(ErrorCode.APPLICATION_PICK_FAILED, "当前平台没有应用选择器");
            }
            return selected.isEmpty() ? null : Path.of(selected);
        } catch (IOException e) {
            throw failed(ErrorCode.APPLICATION_PICK_FAILED, e.getMessage());
        }
    }

    @Override
    public void openFile(String projectId, String fileName, Path application) {
        if (!pathService.isProjectFile(fileName)) {
            throw invalid("只能打开项目根目录下的文稿文件");
        }
        Path file;
        try {
            Path directory = pathService.projectsRoot().toRealPath().resolve(projectId);
            file = pathService.filePathByFileName(projectId, fileName).toRealPath();
            if (!directory.equals(file.getParent()) || !pathService.isProjectFile(file.getFileName().toString()) || !Files.readAttributes(file, BasicFileAttributes.class).isRegularFile()) {
                throw invalid("只能打开项目根目录下的普通文稿文件");
            }
        } catch (NoSuchFileException e) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e.getMessage());
        }
        try {
            if (IS_MAC) {
                ProcessBuilder opener = application == null
                        ? new ProcessBuilder("open", file.toString())
                        : new ProcessBuilder("open", "-a", application.toString(), file.toString());
                run(opener);
            } else if (IS_WINDOWS) {
                if (application == null) {
                    ProcessBuilder opener = powerShell(WINDOWS_OPEN_DOCUMENT);
                    opener.environment().put("PPT_AGENT_DOCUMENT", file.toString());
                    run(opener);
                } else {
                    new ProcessBuilder(application.toString(), file.toString())
                            .redirectOutput(ProcessBuilder.Redirect.DISCARD)
                            .redirectError(ProcessBuilder.Redirect.DISCARD)
                            .start();
                }
            } else {
                throw failed(ErrorCode.FILE_OPEN_FAILED, "当前平台不支持打开项目文件");
            }
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e.getMessage());
        }
    }

    private static ProcessBuilder powerShell(String script) {
        Path executable = Path.of(System.getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe");
        return new ProcessBuilder(executable.toString(), "-NoProfile", "-NonInteractive", "-STA",
                "-Command", "$ErrorActionPreference = 'Stop'\n" + script);
    }

    private static String run(ProcessBuilder command) throws IOException {
        Process process = command.redirectError(ProcessBuilder.Redirect.DISCARD).start();
        try {
            String output = new String(process.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
            if (process.waitFor() != 0) {
                throw new IOException("桌面命令执行失败：" + command.command().get(0));
            }
            return output;
        } catch (InterruptedException e) {
            process.destroyForcibly();
            Thread.currentThread().interrupt();
            throw new IOException("桌面操作被中断", e);
        }
    }
}
