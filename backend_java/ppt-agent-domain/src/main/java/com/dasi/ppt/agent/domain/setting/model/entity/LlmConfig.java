package com.dasi.ppt.agent.domain.setting.model.entity;

import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import lombok.Getter;
import lombok.RequiredArgsConstructor;

@Getter
@RequiredArgsConstructor
public class LlmConfig {

    private final String name;
    private final LlmProtocol protocol;
    private final String baseUrl;
    private final String model;
    private final String key;
}
