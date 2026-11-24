package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.gateway.IFileGateway;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

@Service
@RequiredArgsConstructor
public class AttachmentService {

    private final IFileGateway fileGateway;
}
