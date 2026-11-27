package com.dasi.ppt.agent.domain.agent.service.run;

import com.dasi.ppt.agent.domain.agent.model.entity.ContextEstimate;
import com.dasi.ppt.agent.domain.agent.model.entity.RunChangeSet;
import com.dasi.ppt.agent.domain.agent.model.entity.RunConfig;
import com.dasi.ppt.agent.domain.agent.model.entity.Run;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public class RunState {

    private Run run;
    private RunConfig config;
    private Map<String, byte[]> reviewBaseline;
    private RunChangeSet changes;

    @Setter
    private ModelConfigSettings currentRoute;

    @Setter
    private volatile ContextEstimate contextEstimate;

    public RunState(Run run, RunConfig config, Map<String, byte[]> reviewBaseline, RunChangeSet changes) {
        this(run, config, reviewBaseline, changes, null, null);
    }
}
