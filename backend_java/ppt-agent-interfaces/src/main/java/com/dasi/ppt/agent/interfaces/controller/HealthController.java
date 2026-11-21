package com.dasi.ppt.agent.interfaces.controller;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.boot.availability.ApplicationAvailability;
import org.springframework.boot.availability.ReadinessState;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class HealthController {

    private final ApplicationAvailability availability;
    private final IProjectRepository projects;

    @GetMapping("/api/healthz")
    public ResponseEntity<Void> health() {
        if (availability.getReadinessState() == ReadinessState.ACCEPTING_TRAFFIC && projects.isDatabaseAvailable()) {
            return ResponseEntity.ok().build();
        }
        return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).build();
    }
}
