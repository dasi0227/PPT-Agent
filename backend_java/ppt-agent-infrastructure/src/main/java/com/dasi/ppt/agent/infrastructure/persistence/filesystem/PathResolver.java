package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.nio.file.Path;

@Component
public class PathResolver {

    private final Path projectsRoot;

    public PathResolver(@Value("${ppt.root}") Path root) {
        this.projectsRoot = root.resolve("projects");
    }
}
