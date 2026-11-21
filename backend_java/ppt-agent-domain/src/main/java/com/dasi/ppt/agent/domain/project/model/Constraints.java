package com.dasi.ppt.agent.domain.project.model;

import java.util.List;

public record Constraints(Rules content, Rules visual) {

    public record Rules(List<String> requirements, List<String> prohibitions) {
    }
}
