package com.dasi.ppt.agent.domain.agent.model.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolName;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Set;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class BeforeModelCallResult {
    private Set<ToolName> tools;
}
