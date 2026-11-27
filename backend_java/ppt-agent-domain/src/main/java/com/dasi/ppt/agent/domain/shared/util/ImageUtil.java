package com.dasi.ppt.agent.domain.shared.util;

import lombok.AccessLevel;
import lombok.NoArgsConstructor;

import javax.imageio.ImageIO;
import javax.imageio.ImageReader;
import javax.imageio.stream.MemoryCacheImageInputStream;
import java.awt.Graphics2D;
import java.awt.RenderingHints;
import java.awt.image.BufferedImage;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.UncheckedIOException;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@SuppressWarnings("unused")
@NoArgsConstructor(access = AccessLevel.PRIVATE)
public final class ImageUtil {

    public static byte[] compressPng(byte[] png) {
        ImageReader reader = ImageIO.getImageReadersByFormatName("png").next();
        BufferedImage original;
        try (MemoryCacheImageInputStream input = new MemoryCacheImageInputStream(new ByteArrayInputStream(png))) {
            reader.setInput(input, true, true);
            if ((long) reader.getWidth(0) * reader.getHeight(0) > 40_000_000) {
                throw invalid("图片不能超过 4000 万像素");
            }
            original = reader.read(0);
        } catch (IOException ex) {
            throw invalid("需提供有效的 PNG 图片");
        } finally {
            reader.dispose();
        }

        double scale = Math.min(1.0, 512.0 / Math.max(original.getWidth(), original.getHeight()));
        int width = Math.max(1, (int) Math.round(original.getWidth() * scale));
        int height = Math.max(1, (int) Math.round(original.getHeight() * scale));
        BufferedImage thumbnail = new BufferedImage(width, height, BufferedImage.TYPE_INT_ARGB);
        Graphics2D graphics = thumbnail.createGraphics();
        try {
            graphics.setRenderingHint(RenderingHints.KEY_INTERPOLATION, RenderingHints.VALUE_INTERPOLATION_BILINEAR);
            graphics.drawImage(original, 0, 0, width, height, null);
        } finally {
            graphics.dispose();
        }
        try (ByteArrayOutputStream output = new ByteArrayOutputStream()) {
            if (!ImageIO.write(thumbnail, "webp", output)) {
                throw new IllegalStateException("WebP 编码器不可用");
            }
            return output.toByteArray();
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }
}
