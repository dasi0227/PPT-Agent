package com.dasi.ppt.agent.interfaces.controller.checkpoint;

import com.dasi.ppt.agent.domain.agent.service.history.CheckpointService;
import lombok.RequiredArgsConstructor;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequiredArgsConstructor
public class CheckpointController {

    private final CheckpointService checkpointService;
}
