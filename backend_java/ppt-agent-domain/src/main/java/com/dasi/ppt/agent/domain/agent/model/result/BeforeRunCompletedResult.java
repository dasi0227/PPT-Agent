package com.dasi.ppt.agent.domain.agent.model.result;

import java.util.List;

public record BeforeRunCompletedResult(boolean completed, List<String> issues) {
}
