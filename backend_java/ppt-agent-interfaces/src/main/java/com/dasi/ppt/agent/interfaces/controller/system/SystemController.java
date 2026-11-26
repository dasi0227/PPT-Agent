package com.dasi.ppt.agent.interfaces.controller.system;

import lombok.RequiredArgsConstructor;
import org.springframework.boot.availability.ApplicationAvailability;
import org.springframework.boot.availability.ReadinessState;
import org.springframework.core.io.Resource;
import org.springframework.core.io.ResourceLoader;
import org.springframework.dao.DataAccessException;
import org.springframework.http.*;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.util.DigestUtils;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

import java.io.IOException;
import java.io.InputStream;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@RestController
@RequiredArgsConstructor
public class SystemController {

    private final ApplicationAvailability availability;
    private final ResourceLoader resourceLoader;
    private final JdbcTemplate jdbcTemplate;
    private final Map<String, String> etags = new ConcurrentHashMap<>();

    @GetMapping("/healthz")
    public ResponseEntity<Void> health() {
        if (availability.getReadinessState() != ReadinessState.ACCEPTING_TRAFFIC) {
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).build();
        }

        try {
            jdbcTemplate.queryForObject("SELECT EXISTS (SELECT 1 FROM projects)", Integer.class);
            return ResponseEntity.ok().build();
        } catch (DataAccessException ex) {
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).build();
        }
    }

    @GetMapping("/runtime/{file}")
    public ResponseEntity<Resource> resource(@PathVariable String file) throws IOException {
        Resource resource = resourceLoader.getResource("classpath:runtime/" + file);
        if (!resource.isReadable()) {
            return ResponseEntity.notFound().build();
        }
        String etag = etags.get(file);
        if (etag == null) {
            try (InputStream stream = resource.getInputStream()) {
                etag = DigestUtils.md5DigestAsHex(stream);
                etags.put(file, etag);
            }
        }
        MediaType mediaType = MediaTypeFactory.getMediaType(file).orElse(MediaType.APPLICATION_OCTET_STREAM);
        return ResponseEntity.ok()
                .contentType(mediaType)
                .cacheControl(CacheControl.noCache())
                .eTag(etag)
                .body(resource);
    }
}
