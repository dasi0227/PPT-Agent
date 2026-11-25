package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.Locale;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.failed;

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

    private final PathResolver pathResolver;

    @Override
    public Path pickApplication() {
        String os = System.getProperty("os.name");
        try {
            ProcessBuilder picker;
            if (os.startsWith("Mac")) {
                picker = new ProcessBuilder("osascript", "-e", MAC_PICK_APPLICATION);
            } else if (os.startsWith("Windows")) {
                picker = powerShell(WINDOWS_PICK_APPLICATION);
            } else {
                throw new IOException("当前平台没有应用选择器");
            }
            String selected = run(picker);
            // osascript 在结果后输出一个换行符，路径本身可包含空白。
            if (os.startsWith("Mac") && selected.endsWith("\n")) {
                selected = selected.substring(0, selected.length() - 1);
            }
            return selected.isEmpty() ? null : Path.of(selected);
        } catch (IOException e) {
            throw failed(ErrorCode.APPLICATION_PICK_FAILED, e.getMessage());
        }
    }

    @Override
    public void openFile(String projectId, String relativePath, Path application) {
        Path file = pathResolver.resolveFile(projectId, relativePath);
        String os = System.getProperty("os.name");
        try {
            if (application != null) {
                String suffix = os.startsWith("Windows") ? ".exe" : ".app";
                if (!application.isAbsolute() || !application.toString().toLowerCase(Locale.ROOT).endsWith(suffix)) {
                    throw new IOException("无法使用所选应用打开文件：" + application);
                }
            }
            if (os.startsWith("Mac")) {
                ProcessBuilder opener = application == null
                        ? new ProcessBuilder("open", file.toString())
                        : new ProcessBuilder("open", "-a", application.toString(), file.toString());
                run(opener);
            } else if (os.startsWith("Windows")) {
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
                throw new IOException("当前平台不支持打开项目文件");
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
