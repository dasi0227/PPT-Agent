package com.dasi.ppt.agent.interfaces.controller.project;

import com.dasi.ppt.agent.domain.project.service.ProjectService;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.project.dto.CreateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectPositionsDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectThemeDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.UpdateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ProjectAppearanceVo;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ProjectInfoVo;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ProjectListVo;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.CacheControl;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.DigestUtils;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.ModelAttribute;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.io.IOException;

@RestController
@RequiredArgsConstructor
@RequestMapping("/projects")
public class ProjectController {

    private final ProjectService projectService;

    @GetMapping
    public ProjectListVo listProjects() {
        return ProjectListVo.builder()
                .items(projectService.listProjects()
                        .stream()
                        .map(ProjectInfoVo::toVo)
                        .toList())
                .build();
    }

    @PostMapping("/create")
    public IdVo createProject(@Valid @RequestBody CreateProjectDto request) {
        return IdVo.builder()
                .id(projectService.createProject(request.toEntity()))
                .build();
    }

    @PostMapping("/update")
    public void updateProject(@Valid @RequestBody UpdateProjectDto request) {
        projectService.updateProject(request.getProjectId(), request.getTitle());
    }

    @PostMapping("/delete")
    public void deleteProject(@Valid @RequestBody DeleteProjectDto request) {
        projectService.deleteProject(request.getProjectId());
    }

    @GetMapping("/{projectId}/appearance")
    public ProjectAppearanceVo getProjectAppearance(@PathVariable String projectId) {
        return ProjectAppearanceVo.toVo(projectService.getProjectAppearance(projectId));
    }

    @PostMapping("/theme/save")
    public void saveProjectTheme(@Valid @RequestBody SaveProjectThemeDto request) {
        projectService.saveProjectTheme(request.getProjectId(), request.getThemeId());
    }

    @PostMapping("/positions/save")
    public void saveProjectPositions(@Valid @RequestBody SaveProjectPositionsDto request) {
        projectService.saveProjectPositions(request.toEntity());
    }

    @GetMapping("/{projectId}/logo")
    public ResponseEntity<byte[]> getProjectLogo(@PathVariable String projectId) {
        byte[] bytes = projectService.findProjectLogo(projectId);
        if (bytes == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok()
                .contentType(MediaType.IMAGE_PNG)
                .cacheControl(CacheControl.noCache().cachePrivate())
                .eTag(DigestUtils.md5DigestAsHex(bytes))
                .body(bytes);
    }

    @PostMapping(value = "/logo/save", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public void saveProjectLogo(@Valid @ModelAttribute SaveProjectLogoDto request) throws IOException {
        projectService.saveProjectLogo(request.getProjectId(), request.readPng());
    }

    @PostMapping("/logo/delete")
    public void deleteProjectLogo(@Valid @RequestBody DeleteProjectLogoDto request) {
        projectService.deleteProjectLogo(request.getProjectId());
    }
}
