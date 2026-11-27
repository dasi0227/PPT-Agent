package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.project.model.entity.ExportResult;
import com.dasi.ppt.agent.domain.project.model.entity.ExportSnapshot;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import com.dasi.ppt.agent.domain.shared.exception.RenderUnavailableException;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import lombok.RequiredArgsConstructor;
import org.jsoup.Jsoup;
import org.jsoup.nodes.DataNode;
import org.jsoup.nodes.Document;
import org.jsoup.nodes.Element;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;
import org.springframework.util.FileSystemUtils;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

import java.io.BufferedInputStream;
import java.io.DataInputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CancellationException;
import java.util.concurrent.TimeUnit;
import java.util.regex.Pattern;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.ATTACHMENT_PATH_PREFIX;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Component
@RequiredArgsConstructor
public class RenderGateway implements IRenderGateway {

    private static final Pattern CSS_REFERENCE = Pattern.compile("(?i)(?:url\\(\\s*|@import\\s+)[\"']?([^\"')\\s]+)");
    private static final Pattern SRCSET_REFERENCE = Pattern.compile("(?:^|,\\s*)(data:\\S+|[^,\\s]+)", Pattern.CASE_INSENSITIVE);
    private static final Pattern ATTACHMENT = Pattern.compile("(?i)(?:^|/)(?:" + Pattern.quote(ATTACHMENT_PATH_PREFIX)
            + "|api/(?:v1/)?projects/[^/]+/attachments/|attachments/[0-9a-f]{32}\\b)");
    private static final Pattern SCHEME = Pattern.compile("^[a-zA-Z][a-zA-Z0-9+.-]*:");

    private final ObjectMapper objectMapper;
    private final String baseCss = resource("base.css");
    private final String decorations = resource("decorations.js");

    @Value("${PPT_RENDER_WORKER:render-worker/worker.mjs}")
    private String worker;
    @Value("${PPT_RENDER_NODE:node}")
    private String node;

    @Override
    public String renderHash(RenderMaterial renderMaterial) {
        return HashUtil.contentHash(objectMapper.writeValueAsBytes(List.of(baseCss, decorations, renderMaterial)));
    }

    @Override
    public RenderResult render(RenderMaterial renderMaterial) {
        Path directory = temporaryDirectory();
        try {
            Path output = directory.resolve("slide.png");
            Map<String, Object> job = Map.of("type", "snapshot",
                    "html", html(renderMaterial),
                    "frame", frame(renderMaterial),
                    "styles", styles(renderMaterial.getThemeCss()),
                    "decorations_js", decorations,
                    "screenshot_path", output.toString());
            JsonNode diagnostics = execute(directory, job);
            List<String> issues = new ArrayList<>();
            for (String field : List.of("out_of_bounds", "text_clipping", "failed_resources", "console_errors")) {
                for (JsonNode item : diagnostics.path(field)) {
                    issues.add(field + ": " + item.toString());
                }
            }
            return RenderResult.builder()
                    .screenshot(Files.readAllBytes(output))
                    .layoutIssues(issues)
                    .build();
        } catch (IOException ex) {
            throw new RenderUnavailableException("无法取得页面截图", ex);
        } finally {
            FileSystemUtils.deleteRecursively(directory.toFile());
        }
    }

    @Override
    public ExportResult export(ExportSnapshot snapshot, ExportFormat format) {
        List<Map<String, Object>> pages = new ArrayList<>();
        for (int index = 0; index < snapshot.getSlides().size(); index++) {
            RenderMaterial renderMaterial = snapshot.renderInput(index);
            Map<String, Object> page = new LinkedHashMap<>();
            page.put("id", renderMaterial.getSlideId());
            page.put("title", snapshot.getSlides().get(index).getTitle());
            page.put("chapter", renderMaterial.getChapterTitle());
            page.put("html", html(renderMaterial));
            pages.add(page);
        }
        Map<String, Object> request = Map.of("format", format.getValue(), "title", snapshot.getProjectName(),
                "frame", commonFrame(snapshot.renderInput(0)), "pages", pages,
                "styles", styles(snapshot.getThemeCss()), "decorations_js", decorations);
        Process process = null;
        try {
            process = workerProcess("--export").start();
            try (OutputStream stdin = process.getOutputStream()) {
                objectMapper.writeValue(stdin, request);
            }
            // worker 完成生成后，先发送四字节长度和 JSON 文件信息，随后直接发送成品。
            DataInputStream content = new DataInputStream(new BufferedInputStream(process.getInputStream()));
            JsonNode header = objectMapper.readTree(content.readNBytes(content.readInt()));
            return ExportResult.builder()
                    .filename(header.required("filename").asString())
                    .size(header.required("size").asLong())
                    .content(content)
                    .build();
        } catch (IOException | RuntimeException ex) {
            if (process != null) {
                process.destroy();
            }
            throw new RenderUnavailableException("无法生成导出文件", ex);
        }
    }

    private String html(RenderMaterial renderMaterial) {
        Document document = Jsoup.parse(renderMaterial.getHtml());
        document.outputSettings().prettyPrint(false);
        document.select("link#base-link, link#theme-link, link[href$=/common/base.css], link[href$=/common/tokens.css]").remove();
        String css = styles(renderMaterial.getThemeCss());
        Element base = document.selectFirst("base[href]");
        boolean remoteBase = base != null && base.attr("href").matches("(?i)^(?:https?:)?//.+");
        for (Element element : document.select("[src], [href], [srcset], [poster], object[data], [style], style")) {
            for (String attr : List.of("src", "href", "poster")) {
                if (element.hasAttr(attr)) checkReference(element.attr(attr), remoteBase);
            }
            if (element.normalName().equals("object")) checkReference(element.attr("data"), remoteBase);
            SRCSET_REFERENCE.matcher(element.attr("srcset")).results()
                    .forEach(match -> checkReference(match.group(1), remoteBase));
            String inlineCss = element.normalName().equals("style") ? element.data() : element.attr("style");
            checkCssReferences(inlineCss, remoteBase);
        }
        checkCssReferences(css, remoteBase);
        document.head().prependElement("style").appendChild(new DataNode(css));
        // 同一份页面既会进入本地离线播放器，也会进入 worker 的隔离 iframe。
        document.head().prependElement("meta").attr("http-equiv", "Content-Security-Policy").attr("content",
                "default-src https: http: data: blob:; script-src https: http: data: blob: 'unsafe-inline'; "
                + "style-src https: http: data: 'unsafe-inline'; object-src 'none'; base-uri https: http:");
        return "<!doctype html>" + document.selectFirst("html").outerHtml();
    }

    private void checkCssReferences(String css, boolean remoteBase) {
        CSS_REFERENCE.matcher(css).results()
                .forEach(match -> checkReference(match.group(1), remoteBase));
    }

    private void checkReference(String value, boolean remoteBase) {
        String url = value.strip();
        if (url.isEmpty() || url.startsWith("#") || url.regionMatches(true, 0, "data:", 0, 5)) return;
        if (ATTACHMENT.matcher(url).find() || url.regionMatches(true, 0, "file:", 0, 5)
                || url.matches("^[A-Za-z]:[/\\\\].*") || url.startsWith("\\\\")
                || (!remoteBase && !url.startsWith("//") && !SCHEME.matcher(url).find())) {
            throw invalid("幻灯片不能引用会话附件或本地文件");
        }
    }

    private String styles(String themeCss) {
        String css = themeCss == null ? baseCss : baseCss + "\n" + themeCss;
        return css.replaceAll("(?i)</style", "<\\\\/style");
    }

    private Map<String, Object> frame(RenderMaterial renderMaterial) {
        Map<String, Object> frame = commonFrame(renderMaterial);
        frame.put("slide_id", renderMaterial.getSlideId());
        frame.put("ordinal", renderMaterial.getPageNumber());
        frame.put("chapter", renderMaterial.getChapterTitle() == null ? null : Map.of("title", renderMaterial.getChapterTitle()));
        return frame;
    }

    private Map<String, Object> commonFrame(RenderMaterial renderMaterial) {
        Map<String, Object> frame = new LinkedHashMap<>();
        frame.put("canvas", Map.of("width", 1920, "height", 1080, "aspect_ratio", "16:9"));
        frame.put("total", renderMaterial.getTotalPages());
        frame.put("deck_title", renderMaterial.getDeckTitle());
        frame.put("decorations", renderMaterial.getPositions());
        frame.put("logo", renderMaterial.getLogo() == null ? null : Map.of("src", "data:image/png;base64," + Base64.getEncoder().encodeToString(renderMaterial.getLogo())));
        return frame;
    }

    private static String resource(String name) {
        try {
            return new ClassPathResource("runtime/" + name).getContentAsString(StandardCharsets.UTF_8);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private JsonNode execute(Path directory, Map<String, Object> job) throws IOException {
        Path request = directory.resolve("request.json");
        Path response = directory.resolve("response.json");
        Files.write(request, objectMapper.writeValueAsBytes(job));
        Process process = workerProcess("--job", request.toString()).redirectOutput(response.toFile()).start();
        try {
            if (!process.waitFor(60, TimeUnit.SECONDS)) {
                throw new IOException("渲染进程超时");
            }
            if (Files.size(response) == 0) throw new IOException("渲染进程未返回结果");
            JsonNode result;
            try {
                result = objectMapper.readTree(Files.readAllBytes(response));
            } catch (JacksonException ex) {
                throw new RenderUnavailableException("无法解析渲染进程返回的结果", ex);
            }
            if (!result.path("ok").asBoolean()) {
                throw new RenderUnavailableException(result.path("error").asString("渲染失败"), null);
            }
            return result.path("diagnostics");
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            throw new CancellationException();
        } finally {
            if (process.isAlive()) {
                process.descendants().forEach(ProcessHandle::destroyForcibly);
                process.destroyForcibly();
            }
        }
    }

    private ProcessBuilder workerProcess(String... arguments) {
        List<String> command = new ArrayList<>(List.of(node, Path.of(worker).toAbsolutePath().normalize().toString()));
        command.addAll(List.of(arguments));
        ProcessBuilder builder = new ProcessBuilder(command).redirectError(ProcessBuilder.Redirect.INHERIT);
        if ("1".equals(System.getenv("PPT_RENDER_ELECTRON"))) {
            builder.environment().put("ELECTRON_RUN_AS_NODE", "1");
        }
        return builder;
    }

    private Path temporaryDirectory() {
        try {
            return Files.createTempDirectory("ppt-render-");
        } catch (IOException ex) {
            throw new RenderUnavailableException("无法创建渲染工作目录", ex);
        }
    }
}
