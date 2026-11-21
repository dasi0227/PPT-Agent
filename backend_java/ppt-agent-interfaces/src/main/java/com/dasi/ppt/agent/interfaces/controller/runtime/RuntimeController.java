package com.dasi.ppt.agent.interfaces.controller.runtime;

import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ResourceLoader;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class RuntimeController {

    private final ResourceLoader resourceLoader;
}
