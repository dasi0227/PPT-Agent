package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
)

const (
	renderViewportWidth  = 1600
	renderViewportHeight = 900
	maxRenderOutputBytes = 1024 * 1024
)

type RenderRequest struct {
	RunID          string `json:"run_id"`
	ProjectDir     string `json:"project_dir"`
	SlideID        string `json:"slide_id"`
	HTML           string `json:"html"`
	ScreenshotPath string `json:"screenshot_path"`
	ViewportWidth  int    `json:"viewport_width"`
	ViewportHeight int    `json:"viewport_height"`
	TimeoutMS      int    `json:"timeout_ms"`
}

type RenderDiagnostics struct {
	ScreenshotBytes int              `json:"screenshot_bytes"`
	ContentSize     map[string]int   `json:"content_size"`
	Overflow        map[string]bool  `json:"overflow"`
	Clipping        []map[string]any `json:"clipping"`
	ConsoleErrors   []string         `json:"console_errors"`
	FailedResources []string         `json:"failed_resources"`
	FontStatus      string           `json:"font_status"`
	DurationMS      int64            `json:"duration_ms"`
}

type SlideRenderer interface {
	Render(context.Context, RenderRequest) (RenderDiagnostics, error)
}

type NodeRendererConfig struct {
	NodePath   string
	WorkerPath string
	Timeout    time.Duration
}

type NodeSlideRenderer struct {
	config NodeRendererConfig
}

func NewNodeSlideRenderer(config NodeRendererConfig) *NodeSlideRenderer {
	if config.NodePath == "" {
		config.NodePath = envOr("PPT_RENDER_NODE", "node")
	}
	if config.WorkerPath == "" {
		config.WorkerPath = envOr("PPT_RENDER_WORKER", resolveWorkerPath())
	}
	if config.Timeout == 0 {
		config.Timeout = 25 * time.Second
	}
	return &NodeSlideRenderer{config: config}
}

func resolveWorkerPath() string {
	for _, candidate := range []string{
		filepath.Join("render-worker", "worker.mjs"),
		filepath.Join("backend", "render-worker", "worker.mjs"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join("render-worker", "worker.mjs")
}

func (r *NodeSlideRenderer) Health(ctx context.Context) error {
	commandCtx, cancel := context.WithTimeout(ctx, minDuration(r.config.Timeout, 10*time.Second))
	defer cancel()
	cmd := exec.CommandContext(commandCtx, r.config.NodePath, r.config.WorkerPath, "--health")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ()
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("render worker health check failed: %w: %s", err, compactSnippet(output.String(), 500))
	}
	if !strings.Contains(output.String(), `"ok":true`) {
		return fmt.Errorf("render worker health check returned an invalid response")
	}
	return nil
}

func (r *NodeSlideRenderer) Render(ctx context.Context, request RenderRequest) (RenderDiagnostics, error) {
	commandCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	raw, err := json.Marshal(request)
	if err != nil {
		return RenderDiagnostics{}, err
	}
	cmd := exec.CommandContext(commandCtx, r.config.NodePath, r.config.WorkerPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ()
	cmd.Stdin = bytes.NewReader(raw)
	var stdout, stderr limitedBuffer
	stdout.limit, stderr.limit = maxRenderOutputBytes, 64*1024
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if commandCtx.Err() != nil {
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return RenderDiagnostics{}, fmt.Errorf("render worker timed out: %w", commandCtx.Err())
		}
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(stdout.Bytes(), &failure) == nil && strings.TrimSpace(failure.Error) != "" {
			return RenderDiagnostics{}, errors.New(failure.Error)
		}
		details := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		return RenderDiagnostics{}, fmt.Errorf("render worker failed: %w: %s", err, compactSnippet(details, 1000))
	}
	if stdout.exceeded {
		return RenderDiagnostics{}, errors.New("render worker output exceeded the configured limit")
	}
	var result struct {
		OK          bool              `json:"ok"`
		Error       string            `json:"error"`
		Diagnostics RenderDiagnostics `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return RenderDiagnostics{}, fmt.Errorf("invalid render worker response: %w", err)
	}
	if !result.OK {
		return RenderDiagnostics{}, errors.New(result.Error)
	}
	return result.Diagnostics, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Buffer.Len()+len(p) > b.limit {
		remaining := b.limit - b.Buffer.Len()
		if remaining > 0 {
			_, _ = b.Buffer.Write(p[:remaining])
		}
		b.exceeded = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

type slideRenderTool struct {
	pack     contextengine.ContextPack
	renderer SlideRenderer
}

func (slideRenderTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "render_slide", Description: "Render one authorized slide in isolated Chromium and return screenshot-bound visual diagnostics.",
		Parameters: objectSchema([]string{"slide_id"}, map[string]any{
			"slide_id": map[string]any{
				"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`,
			},
		}),
	}
}

func (t slideRenderTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	slideID := stringValue(input.Args["slide_id"])
	target := TargetRef{Type: "slide", SlideID: slideID}
	if !stableSlideID.MatchString(slideID) || slideID == "current" {
		return failedToolResult(CodeModelInvalid, "slide_id must be a stable slide identifier", false)
	}
	if !input.Scope.AllowsRead(target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested render target is outside the current run scope", false)
	}
	html, source, err := readArtifact(input.ProjectDir, input.Transaction, presentationSlideRef(slideID))
	if err != nil {
		if errorsIsNotExist(err) {
			return failedToolResult(CodeTargetNotFound, "slide HTML was not found", false)
		}
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	if _, htmlErr := validateHTML(html); htmlErr != nil {
		return failedToolResult(CodeRenderFailed, htmlErr.Error(), true)
	}
	if t.renderer == nil {
		return failedToolResult(CodeRenderFailed, "browser renderer is unavailable", true)
	}
	screenshotID := "shot_" + uuid.NewString()
	runID := input.RunID
	if runID == "" {
		runID = "adhoc"
	}
	screenshotDir := filepath.Join(input.ProjectDir, ".runtime", "renders", runID)
	if err := os.MkdirAll(screenshotDir, 0o700); err != nil {
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	screenshotPath := filepath.Join(screenshotDir, screenshotID+".png")
	request := RenderRequest{
		RunID: runID, ProjectDir: input.ProjectDir, SlideID: slideID, HTML: string(html),
		ScreenshotPath: screenshotPath, ViewportWidth: renderViewportWidth,
		ViewportHeight: renderViewportHeight, TimeoutMS: 15000,
	}
	started := time.Now()
	diagnostics, err := t.renderer.Render(ctx, request)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	if diagnostics.DurationMS == 0 {
		diagnostics.DurationMS = time.Since(started).Milliseconds()
	}
	if diagnostics.ScreenshotBytes <= 0 {
		if info, statErr := os.Stat(screenshotPath); statErr == nil {
			diagnostics.ScreenshotBytes = int(info.Size())
		}
	}
	if diagnostics.ScreenshotBytes <= 0 || diagnostics.ScreenshotBytes > 10*1024*1024 {
		_ = os.Remove(screenshotPath)
		return failedToolResult(CodeRenderFailed, "browser screenshot is missing or exceeds the output limit", true)
	}
	sourceHash, err := renderHashWithoutTransaction(t.pack, input, slideID)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	blocking, warnings := renderIssues(target, diagnostics)
	screenshotRef := "run:" + runID + "/screenshot:" + screenshotID
	screenshotURL := "/api/v1/runs/" + runID + "/screenshots/" + screenshotID
	data := map[string]any{
		"screenshot_ref": screenshotRef, "screenshot_url": screenshotURL,
		"slide_id": slideID, "source": source,
		"revision": presentationRevision(t.pack, input, slideID), "hash": sourceHash,
		"viewport":     map[string]int{"width": renderViewportWidth, "height": renderViewportHeight},
		"content_size": diagnostics.ContentSize, "overflow": diagnostics.Overflow,
		"clipping": diagnostics.Clipping, "console_errors": diagnostics.ConsoleErrors,
		"failed_resources": diagnostics.FailedResources, "font_status": diagnostics.FontStatus,
		"duration_ms": diagnostics.DurationMS, "blocking_issues": blocking, "warnings": warnings,
	}
	result := SuccessfulToolResult("slide rendered in isolated Chromium")
	result.Data = data
	result.Issues = append(blocking, warnings...)
	if len(blocking) > 0 {
		result.Evidence = []Evidence{newEvidence("render_diagnostic", target, sourceHash, data)}
		result.OK, result.Code, result.Retryable = false, CodeRenderFailed, true
		return result
	}
	result.Evidence = []Evidence{newEvidence("render", target, sourceHash, data)}
	return result
}

func renderHashWithoutTransaction(pack contextengine.ContextPack, input DomainToolInput, slideID string) (string, error) {
	if input.Transaction != nil {
		return renderSourceHash(pack, input.Transaction, slideID)
	}
	parts := []byte{}
	for _, ref := range []ArtifactRef{designRef(pack), blueprintSlideRef(slideID), presentationSlideRef(slideID)} {
		raw, _, err := readArtifact(input.ProjectDir, nil, ref)
		if err != nil {
			return "", err
		}
		parts = append(parts, []byte(ref.Key())...)
		parts = append(parts, 0)
		parts = append(parts, raw...)
		parts = append(parts, 0)
	}
	return hashBytes(parts), nil
}

func presentationRevision(pack contextengine.ContextPack, input DomainToolInput, slideID string) int {
	if input.Transaction != nil && input.Transaction.IsStaged(presentationSlideRef(slideID)) {
		return pack.Revisions.Presentations[slideID] + 1
	}
	return pack.Revisions.Presentations[slideID]
}

func renderIssues(target TargetRef, diagnostics RenderDiagnostics) ([]Issue, []Issue) {
	blocking, warnings := []Issue{}, []Issue{}
	if diagnostics.Overflow["horizontal"] || diagnostics.Overflow["vertical"] {
		blocking = append(blocking, Issue{
			Code: "RENDER_OVERFLOW", Severity: SeverityError, Target: target,
			Summary: "slide content overflows the fixed PPT viewport", Action: "edit the layout and render again",
		})
	}
	if len(diagnostics.Clipping) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_CLIPPING", Severity: SeverityError, Target: target,
			Summary: fmt.Sprintf("%d elements are clipped or outside the slide stage", len(diagnostics.Clipping)),
			Action:  "edit the out-of-bounds elements and render again",
		})
	}
	if len(diagnostics.ConsoleErrors) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_CONSOLE_ERROR", Severity: SeverityError, Target: target,
			Summary: strings.Join(diagnostics.ConsoleErrors, "; "), Action: "fix page runtime errors and render again",
		})
	}
	if len(diagnostics.FailedResources) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_RESOURCE_FAILED", Severity: SeverityError, Target: target,
			Summary: strings.Join(diagnostics.FailedResources, "; "), Action: "use controlled project resources and render again",
		})
	}
	if diagnostics.FontStatus != "loaded" {
		warnings = append(warnings, Issue{
			Code: "RENDER_FONT_STATUS", Severity: SeverityWarning, Target: target,
			Summary: "font readiness: " + diagnostics.FontStatus,
		})
	}
	return blocking, warnings
}
