package com.dasi.ppt.agent.domain.setting.model.entity;

import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import lombok.*;

@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class ModelConfigSettings {

    private String id;
    private String name;
    private LlmProtocol protocol;
    private ModelProvider provider;
    private String baseUrl;
    private String model;
    private String key;

    public boolean hasNoChanges() {
        return name == null && protocol == null && provider == null && baseUrl == null && model == null && key == null;
    }

    public void removeTrailingSlashes() {
        if (baseUrl != null) {
            baseUrl = baseUrl.replaceFirst("([^/])/+$", "$1");
        }
    }
}
