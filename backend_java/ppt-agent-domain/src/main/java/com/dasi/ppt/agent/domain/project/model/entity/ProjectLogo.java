package com.dasi.ppt.agent.domain.project.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.util.zip.CRC32;
import javax.imageio.ImageIO;
import javax.imageio.ImageReadParam;
import javax.imageio.ImageReader;
import javax.imageio.stream.MemoryCacheImageInputStream;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public final class ProjectLogo {

    private static final int MAX_BYTES = 10 * 1024 * 1024;
    private static final long MAX_PIXELS = 40_000_000;
    private static final int IEND = 0x49454e44;
    private byte[] bytes;

    // ImageIO 校验签名并解码像素，但会跳过 PNG 块的 CRC。
    private boolean hasCompleteChunks() {
        ByteBuffer png = ByteBuffer.wrap(bytes);
        CRC32 checksum = new CRC32();
        for (int offset = 8; offset <= bytes.length - 12; ) {
            int length = png.getInt(offset);
            if (length < 0 || length > bytes.length - offset - 12) {
                return false;
            }
            checksum.reset();
            checksum.update(bytes, offset + 4, length + 4);
            if ((int) checksum.getValue() != png.getInt(offset + 8 + length)) {
                return false;
            }
            if (png.getInt(offset + 4) == IEND) {
                return length == 0 && offset + 12 == bytes.length;
            }
            offset += length + 12;
        }
        return false;
    }

    public void validate() {
        if (bytes.length > MAX_BYTES) {
            throw invalid("Logo 不能超过 10 MiB");
        }
        if (!hasCompleteChunks()) {
            throw invalid("Logo 需为完整的 PNG 图片");
        }
        ImageReader reader = ImageIO.getImageReadersByFormatName("PNG").next();
        try (MemoryCacheImageInputStream input = new MemoryCacheImageInputStream(new ByteArrayInputStream(bytes))) {
            reader.setInput(input, true, true);
            int width = reader.getWidth(0);
            int height = reader.getHeight(0);
            if (width <= 0 || height <= 0 || (long) width * height > MAX_PIXELS) {
                throw invalid("Logo 尺寸无效或超过 4000 万像素");
            }
            // 解码检查图片内容，采样可避免为大图分配整张位图。
            ImageReadParam params = reader.getDefaultReadParam();
            params.setSourceSubsampling(Math.max(1, width / 512), Math.max(1, height / 512), 0, 0);
            reader.read(0, params);
        } catch (IOException | IllegalArgumentException ex) {
            throw invalid("Logo 需为有效的 PNG 图片");
        } finally {
            reader.dispose();
        }
    }
}
