package com.dasi.ppt.agent.domain.agent.service.context;

public interface PromptAssembler<I, O> {

    O assemble(I input);
}
