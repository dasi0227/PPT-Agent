package com.dasi.ppt.agent.domain.agent.model.entity;

import lombok.Builder;
import lombok.Getter;

@Getter
@Builder
public final class Component {
    private final String id;
    private final String name;
}
