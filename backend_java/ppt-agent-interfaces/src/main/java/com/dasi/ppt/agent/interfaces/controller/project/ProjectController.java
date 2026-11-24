package com.dasi.ppt.agent.interfaces.controller.project;

import com.dasi.ppt.agent.domain.project.model.entity.ProjectLogo;
import com.dasi.ppt.agent.domain.project.service.ProjectService;
import com.dasi.ppt.agent.interfaces.controller.project.dto.CreateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectAppearanceDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.UpdateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ProjectAppearanceVo;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ProjectIdVo;
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
import java.util.List;

@RestController
@RequiredArgsConstructor
@RequestMapping("/projects")
public class ProjectController {

    private final ProjectService projectService;

    @GetMapping
    public ResponseEntity<ProjectListVo> list() {
        List<ProjectListVo.ProjectSummaryVo> items = projectService.listProjects()
                .stream()
                .map(project -> ProjectListVo.ProjectSummaryVo.builder()
                        .id(project.getId())
                        .title(project.getTitle())
                        .updatedAt(project.getUpdatedAt())
                        .build())
                .toList();
        return ResponseEntity.ok()
                .cacheControl(CacheControl.noStore())
                .body(ProjectListVo.builder().items(items).build());
    }

    @PostMapping("/create")
    public ProjectIdVo create(@Valid @RequestBody CreateProjectDto request) {
        String id = projectService.createProject(request.getTitle());
        return ProjectIdVo.builder().id(id).build();
    }

    @PostMapping("/update")
    public ProjectIdVo update(@Valid @RequestBody UpdateProjectDto request) {
        projectService.updateProject(request.getProjectId(), request.getTitle());
        return ProjectIdVo.builder().id(request.getProjectId()).build();
    }

    @PostMapping("/delete")
    public void delete(@Valid @RequestBody DeleteProjectDto request) {
        projectService.deleteProject(request.getProjectId());
    }

    @GetMapping("/{projectId}/appearance")
    public ProjectAppearanceVo appearance(@PathVariable String projectId) {
        return ProjectAppearanceVo.toVo(projectService.findAppearance(projectId));
    }

    @PostMapping("/appearance/save")
    public void saveAppearance(@Valid @RequestBody SaveProjectAppearanceDto request) {
        projectService.saveAppearance(request.toProject());
    }

    @PostMapping(value = "/logo/save", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public void saveLogo(@Valid @ModelAttribute SaveProjectLogoDto request) throws IOException {
        projectService.saveLogo(request.getProjectId(), ProjectLogo.builder()
                .bytes(request.getFile().getBytes())
                .build());
    }

    @PostMapping("/logo/delete")
    public void deleteLogo(@Valid @RequestBody DeleteProjectLogoDto request) {
        projectService.deleteLogo(request.getProjectId());
    }

    @GetMapping("/{projectId}/logo")
    public ResponseEntity<byte[]> logo(@PathVariable String projectId) {
        byte[] bytes = projectService.findLogo(projectId);
        if (bytes == null) {
            return ResponseEntity.notFound().build();
        }
        return ResponseEntity.ok()
                .contentType(MediaType.IMAGE_PNG)
                .cacheControl(CacheControl.noCache().cachePrivate())
                .eTag(DigestUtils.md5DigestAsHex(bytes))
                .body(bytes);
    }
}
