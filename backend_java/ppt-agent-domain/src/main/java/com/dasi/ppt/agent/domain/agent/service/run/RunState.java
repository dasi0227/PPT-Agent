package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.agent.model.ContextEstimate;
import com.dasi.ppt.agent.domain.agent.model.RunChangeSet;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.setting.model.RunConfig;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.Setter;

import java.util.Map;

@Getter
@RequiredArgsConstructor
public class RunState {

    private final Run run;
    private final RunConfig config;
    private final Map<String, byte[]> reviewBaseline;
    private final RunChangeSet changes;

    @Setter
    private LlmConfig currentRoute;

    @Setter
    private volatile ContextEstimate contextEstimate;
}
