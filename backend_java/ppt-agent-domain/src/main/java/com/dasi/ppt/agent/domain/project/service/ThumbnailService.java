package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.entity.ExportSnapshot;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import com.dasi.ppt.agent.domain.shared.util.ImageUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import tools.jackson.core.type.TypeReference;
import tools.jackson.databind.ObjectMapper;

import java.util.LinkedHashMap;
import java.util.Map;

@Service
@RequiredArgsConstructor
public class ThumbnailService {

    private static final String INDEX = ".runtime/render-slides.json";
    private final IFileGateway fileGateway;
    private final IRenderGateway renderGateway;
    private final DeckService deckService;
    private final ObjectMapper objectMapper;
    private final ProjectWriteLock projectWriteLock;
    private final ProjectWriteLock renderLocks = new ProjectWriteLock();

    public byte[] readThumbnail(String projectId, String slideId) {
        String thumbnail = ".runtime/thumbnails/" + slideId + ".webp";
        synchronized (renderLocks.lockFor(projectId + "/" + slideId)) {
            while (true) {
                IRenderGateway.RenderMaterial renderMaterial;
                String hash;
                synchronized (projectWriteLock.lockFor(projectId)) {
                    renderMaterial = readPage(projectId, slideId);
                    if (renderMaterial == null) return null;
                    hash = renderGateway.renderHash(renderMaterial);
                    Map<String, String> entry = readIndex(projectId).get(slideId);
                    if (entry != null && hash.equals(entry.get("hash"))) {
                        byte[] cached = fileGateway.read(projectId, thumbnail);
                        if (cached != null) return cached;
                        byte[] screenshot = fileGateway.read(projectId, ".runtime/screenshots/" + entry.get("screenshot_id") + ".png");
                        if (screenshot != null) {
                            byte[] webp = ImageUtil.pngThumbnail(screenshot);
                            fileGateway.replace(projectId, thumbnail, webp);
                            return webp;
                        }
                    }
                }
                byte[] screenshot = renderGateway.render(renderMaterial, () -> false).getScreenshot();
                byte[] webp = ImageUtil.pngThumbnail(screenshot);
                synchronized (projectWriteLock.lockFor(projectId)) {
                    IRenderGateway.RenderMaterial current = readPage(projectId, slideId);
                    if (current == null) return null;
                    if (!hash.equals(renderGateway.renderHash(current))) continue;
                    Map<String, Map<String, String>> index = readIndex(projectId);
                    String screenshotId = IdUtil.genScreenshotId();
                    fileGateway.create(projectId, ".runtime/screenshots/" + screenshotId + ".png", screenshot);
                    fileGateway.delete(projectId, thumbnail);
                    index.put(slideId, Map.of("screenshot_id", screenshotId, "hash", hash));
                    fileGateway.replace(projectId, INDEX, objectMapper.writeValueAsBytes(index));
                    fileGateway.replace(projectId, thumbnail, webp);
                    return webp;
                }
            }
        }
    }

    private IRenderGateway.RenderMaterial readPage(String projectId, String slideId) {
        ExportSnapshot snapshot = deckService.readRenderSnapshot(projectId, slideId);
        if (snapshot != null) {
            for (int index = 0; index < snapshot.getPages().size(); index++) {
                ExportSnapshot.Page page = snapshot.getPages().get(index);
                if (slideId.equals(page.getSlideId()) && page.getHtml() != null) return snapshot.renderInput(index);
            }
        }
        return null;
    }

    private Map<String, Map<String, String>> readIndex(String projectId) {
        byte[] data = fileGateway.read(projectId, INDEX);
        return data == null ? new LinkedHashMap<>() : objectMapper.readValue(data, new TypeReference<>() {});
    }
}
