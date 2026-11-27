package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AccessLevel;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import org.springframework.web.bind.annotation.BindParam;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.util.Arrays;
import java.util.HexFormat;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Builder
@Getter
@NoArgsConstructor(access = AccessLevel.PRIVATE)
@AllArgsConstructor
public final class SaveProjectLogoDto {
    private static final byte[] PNG_SIGNATURE = HexFormat.of().parseHex("89504e470d0a1a0a");

    @NotBlank
    @BindParam("project_id")
    private String projectId;
    @NotNull
    private MultipartFile file;

    public Project toEntity() throws IOException {
        if (file.getSize() > 2_000_000) {
            throw invalid("Logo 不能超过 2 MB");
        }
        byte[] bytes = file.getBytes();
        if (bytes.length <= PNG_SIGNATURE.length || !Arrays.equals(PNG_SIGNATURE, Arrays.copyOf(bytes, PNG_SIGNATURE.length))) {
            throw invalid("Logo 需为 PNG 图片");
        }
        return Project.builder()
                .id(projectId)
                .logo(bytes)
                .build();
    }
}
