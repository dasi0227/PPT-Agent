package com.dasi.ppt.agent.interfaces.controller.session.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Pattern;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class UpdateSessionDto {
    @NotBlank
    private String sessionId;
    @NotBlank
    private String title;
    @Pattern(regexp = "[0-9]+", message = "expected_revision 需为非负十进制整数")
    private String expectedRevision;

    public Long readExpectedRevision() {
        return expectedRevision == null ? null : Long.valueOf(expectedRevision);
    }
}
