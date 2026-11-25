package com.dasi.ppt.agent.interfaces.controller.project;

import com.dasi.ppt.agent.domain.project.service.DeckService;
import com.dasi.ppt.agent.domain.project.service.ProjectService;
import com.dasi.ppt.agent.interfaces.controller.IdVo;
import com.dasi.ppt.agent.interfaces.controller.project.dto.CreateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.DeleteProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveConstraintDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveMetaDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveOutlineDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectLogoDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectPositionsDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveProjectThemeDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveSlideHtmlDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.SaveSlideSpecDto;
import com.dasi.ppt.agent.interfaces.controller.project.dto.UpdateProjectDto;
import com.dasi.ppt.agent.interfaces.controller.project.vo.ContentHashVo;
import com.dasi.ppt.agent.interfaces.controller.project.vo.DeckInfoVo;
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
import org.springframework.web.bind.annotation.RestController;

import java.io.IOException;

@RestController
@RequiredArgsConstructor
public class ProjectController {

    private final DeckService deckService;
    private final ProjectService projectService;

    @GetMapping("/projects")
    public ProjectListVo listProjects() {
        return ProjectListVo.builder()
                .items(projectService.listProjects()
                        .stream()
                        .map(ProjectInfoVo::toVo)
                        .toList())
                .build();
    }

    @GetMapping("/projects/{projectId}")
    public DeckInfoVo readDeck(@PathVariable String projectId) {
        return DeckInfoVo.toVo(deckService.readDeck(projectId));
    }

    @PostMapping("/projects/create")
    public IdVo createProject(@Valid @RequestBody CreateProjectDto request) {
        return IdVo.builder()
                .id(projectService.createProject(request.toEntity()))
                .build();
    }

    @PostMapping("/projects/update")
    public void updateProject(@Valid @RequestBody UpdateProjectDto request) {
        projectService.updateProject(request.getProjectId(), request.getTitle());
    }

    @PostMapping("/projects/delete")
    public void deleteProject(@Valid @RequestBody DeleteProjectDto request) {
        projectService.deleteProject(request.getProjectId());
    }

    @GetMapping("/projects/{projectId}/appearance")
    public ProjectAppearanceVo getProjectAppearance(@PathVariable String projectId) {
        return ProjectAppearanceVo.toVo(projectService.getProjectAppearance(projectId));
    }

    @PostMapping("/projects/theme/save")
    public void saveProjectTheme(@Valid @RequestBody SaveProjectThemeDto request) {
        projectService.saveProjectTheme(request.getProjectId(), request.getThemeId());
    }

    @PostMapping("/projects/positions/save")
    public void saveProjectPositions(@Valid @RequestBody SaveProjectPositionsDto request) {
        projectService.saveProjectPositions(request.toEntity());
    }

    @GetMapping("/projects/{projectId}/logo")
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

    @PostMapping(value = "/projects/logo/save", consumes = MediaType.MULTIPART_FORM_DATA_VALUE)
    public void saveProjectLogo(@Valid @ModelAttribute SaveProjectLogoDto request) throws IOException {
        projectService.saveProjectLogo(request.getProjectId(), request.readPng());
    }

    @PostMapping("/projects/logo/delete")
    public void deleteProjectLogo(@Valid @RequestBody DeleteProjectLogoDto request) {
        projectService.deleteProjectLogo(request.getProjectId());
    }

    @PostMapping("/projects/meta/save")
    public ContentHashVo saveMeta(@Valid @RequestBody SaveMetaDto request) {
        return ContentHashVo.builder()
                .hash(deckService.saveMeta(request.getProjectId(), request.getBaseHash(), request.getMeta()))
                .build();
    }

    @PostMapping("/projects/outline/save")
    public ContentHashVo saveOutline(@Valid @RequestBody SaveOutlineDto request) {
        return ContentHashVo.builder()
                .hash(deckService.saveOutline(request.getProjectId(), request.getBaseHash(), request.getOutline()))
                .build();
    }

    @PostMapping("/projects/constraint/save")
    public ContentHashVo saveConstraint(@Valid @RequestBody SaveConstraintDto request) {
        return ContentHashVo.builder()
                .hash(deckService.saveConstraint(request.getProjectId(), request.getBaseHash(), request.getConstraint()))
                .build();
    }

    @PostMapping("/slides/spec/save")
    public ContentHashVo saveSlideSpec(@Valid @RequestBody SaveSlideSpecDto request) {
        return ContentHashVo.builder()
                .hash(deckService.saveSlideSpec(request.getProjectId(), request.getSlideId(), request.getBaseHash(), request.getSpec()))
                .build();
    }

    @PostMapping("/slides/html/save")
    public ContentHashVo saveSlideHtml(@Valid @RequestBody SaveSlideHtmlDto request) {
        return ContentHashVo.builder()
                .hash(deckService.saveSlideHtml(request.getProjectId(), request.getSlideId(), request.getBaseHash(), request.getHtml()))
                .build();
    }
}
