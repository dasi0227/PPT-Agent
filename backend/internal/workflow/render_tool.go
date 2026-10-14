package workflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/branding"
	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/model"
	"github.com/dasi0227/PPT-Agent/backend/internal/renderimage"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimehtml"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
	"github.com/dasi0227/PPT-Agent/backend/internal/tooldefinition"
	"github.com/google/uuid"
)

const maxRenderOutputBytes = 1024 * 1024

var ErrRenderWorkerUnavailable = errors.New("render worker unavailable")

type RenderWorkerError struct {
	Operation string
	Cause     error
}

func (e *RenderWorkerError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Operation + ": " + ErrRenderWorkerUnavailable.Error()
	}
	return e.Operation + ": " + e.Cause.Error()
}

func (e *RenderWorkerError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *RenderWorkerError) Is(target error) bool {
	return target == ErrRenderWorkerUnavailable
}

func renderWorkerError(operation string, cause error) error {
	return &RenderWorkerError{Operation: operation, Cause: cause}
}

type RenderRequest struct {
	RuntimeAssetsDir string                   `json:"runtime_assets_dir"`
	Type             string                   `json:"type,omitempty"`
	RequestID        string                   `json:"request_id,omitempty"`
	RunID            string                   `json:"run_id"`
	ProjectDir       string                   `json:"project_dir"`
	SlideID          string                   `json:"slide_id"`
	HTML             string                   `json:"html"`
	ScreenshotPath   string                   `json:"screenshot_path"`
	ViewportWidth    int                      `json:"viewport_width"`
	ViewportHeight   int                      `json:"viewport_height"`
	TimeoutMS        int                      `json:"timeout_ms"`
	Frame            spec.RuntimeFrameContext `json:"frame"`
	BaseCSS          string                   `json:"base_css"`
	ThemeID          string                   `json:"theme_id"`
	ThemeCSS         string                   `json:"theme_css"`
}

type RenderDiagnostics struct {
	ScreenshotBytes    int               `json:"screenshot_bytes"`
	ContentSize        map[string]int    `json:"content_size"`
	Overflow           map[string]bool   `json:"overflow"`
	OutOfBounds        []map[string]any  `json:"out_of_bounds"`
	RuntimeDecorations []string          `json:"runtime_decorations"`
	ConsoleErrors      []string          `json:"console_errors"`
	FailedResources    []string          `json:"failed_resources"`
	FontStatus         string            `json:"font_status"`
	DurationMS         int64             `json:"duration_ms"`
	ResourceHashes     map[string]string `json:"resource_hashes"`
}

type PDFRequest struct {
	Type       string   `json:"type,omitempty"`
	RequestID  string   `json:"request_id,omitempty"`
	PNGPaths   []string `json:"png_paths"`
	OutputPath string   `json:"output_path"`
	TimeoutMS  int      `json:"timeout_ms"`
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
	config  NodeRendererConfig
	mu      sync.Mutex
	writeMu sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	pending map[string]chan workerRenderResponse
	ready   chan error
	done    chan struct{}
	closed  bool
	slots   chan struct{}
}

type workerRenderResponse struct {
	RequestID   string            `json:"request_id"`
	Type        string            `json:"type"`
	OK          bool              `json:"ok"`
	Error       string            `json:"error"`
	Diagnostics RenderDiagnostics `json:"diagnostics"`
	PageCount   int               `json:"page_count"`
	ErrorCode   string            `json:"error_code"`
}

// AssemblePDF asks the shared Chromium worker to place an already rendered PNG
// batch into fixed 16:9, marginless PDF pages.
func (r *NodeSlideRenderer) AssemblePDF(ctx context.Context, paths []string, outputPath string) (int, error) {
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	commandCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	if err := r.ensureWorker(commandCtx); err != nil {
		return 0, err
	}
	request := PDFRequest{Type: "pdf", RequestID: "pdf_" + uuid.NewString(), PNGPaths: append([]string(nil), paths...), OutputPath: outputPath, TimeoutMS: 20000}
	raw, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}
	response := make(chan workerRenderResponse, 1)
	r.mu.Lock()
	if r.closed || r.stdin == nil {
		r.mu.Unlock()
		return 0, renderWorkerError("worker_state", ErrRenderWorkerUnavailable)
	}
	r.pending[request.RequestID] = response
	stdin := r.stdin
	r.mu.Unlock()
	r.writeMu.Lock()
	_, writeErr := stdin.Write(append(raw, '\n'))
	r.writeMu.Unlock()
	if writeErr != nil {
		r.removePending(request.RequestID)
		return 0, renderWorkerError("worker_stdin_write", writeErr)
	}
	select {
	case result := <-response:
		if !result.OK {
			return 0, errors.New(result.Error)
		}
		return result.PageCount, nil
	case <-commandCtx.Done():
		cancelRaw, _ := json.Marshal(map[string]any{"type": "cancel", "request_id": request.RequestID})
		r.writeMu.Lock()
		_, _ = stdin.Write(append(cancelRaw, '\n'))
		r.writeMu.Unlock()
		r.removePending(request.RequestID)
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, renderWorkerError("worker_response_timeout", commandCtx.Err())
	}
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
	return &NodeSlideRenderer{
		config: config, pending: map[string]chan workerRenderResponse{},
		slots: make(chan struct{}, 3),
	}
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
	return r.ensureWorker(ctx)
}

func (r *NodeSlideRenderer) Render(ctx context.Context, request RenderRequest) (RenderDiagnostics, error) {
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return RenderDiagnostics{}, ctx.Err()
	}
	commandCtx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()
	if err := r.ensureWorker(commandCtx); err != nil {
		return RenderDiagnostics{}, err
	}
	request.RequestID = "render_" + uuid.NewString()
	request.Type = "render"
	raw, err := json.Marshal(request)
	if err != nil {
		return RenderDiagnostics{}, err
	}
	response := make(chan workerRenderResponse, 1)
	r.mu.Lock()
	if r.closed || r.stdin == nil {
		r.mu.Unlock()
		return RenderDiagnostics{}, renderWorkerError("worker_state", ErrRenderWorkerUnavailable)
	}
	r.pending[request.RequestID] = response
	stdin := r.stdin
	r.mu.Unlock()
	r.writeMu.Lock()
	_, writeErr := stdin.Write(append(raw, '\n'))
	r.writeMu.Unlock()
	if writeErr != nil {
		r.removePending(request.RequestID)
		return RenderDiagnostics{}, renderWorkerError("worker_stdin_write", writeErr)
	}
	select {
	case result := <-response:
		if !result.OK {
			switch result.ErrorCode {
			case model.CodeRunCanceled:
				return RenderDiagnostics{}, context.Canceled
			case model.CodePersistenceFailed:
				return RenderDiagnostics{}, model.NewAgentError(model.CodePersistenceFailed, "render_screenshot", errors.New(result.Error))
			default:
				return RenderDiagnostics{}, renderWorkerError("worker_process", errors.New(result.Error))
			}
		}
		return result.Diagnostics, nil
	case <-commandCtx.Done():
		cancelRaw, _ := json.Marshal(map[string]any{"type": "cancel", "request_id": request.RequestID})
		r.writeMu.Lock()
		_, _ = stdin.Write(append(cancelRaw, '\n'))
		r.writeMu.Unlock()
		r.removePending(request.RequestID)
		if ctx.Err() != nil {
			return RenderDiagnostics{}, ctx.Err()
		}
		return RenderDiagnostics{}, renderWorkerError("worker_response_timeout", commandCtx.Err())
	}
}

func (r *NodeSlideRenderer) ensureWorker(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return renderWorkerError("worker_closed", ErrRenderWorkerUnavailable)
	}
	if r.cmd != nil {
		r.mu.Unlock()
		return nil
	}
	cmd := exec.Command(r.config.NodePath, r.config.WorkerPath, "--serve")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = os.Environ()
	if os.Getenv("PPT_RENDER_ELECTRON") == "1" {
		// Only the render worker runs Electron as Node. Native file-open actions
		// must not pass this mode on to editors such as VS Code or Cursor.
		cmd.Env = append(cmd.Env, "ELECTRON_RUN_AS_NODE=1")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		r.mu.Unlock()
		return renderWorkerError("worker_stdin", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.mu.Unlock()
		return renderWorkerError("worker_stdout", err)
	}
	cmd.Stderr = os.Stderr
	ready := make(chan error, 1)
	done := make(chan struct{})
	r.cmd, r.stdin, r.ready, r.done = cmd, stdin, ready, done
	if err := cmd.Start(); err != nil {
		r.cmd, r.stdin, r.ready, r.done = nil, nil, nil, nil
		r.mu.Unlock()
		return renderWorkerError("worker_start", err)
	}
	r.mu.Unlock()
	go func() {
		defer close(done)
		r.readWorker(cmd, stdout, ready)
	}()
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(minDuration(r.config.Timeout, 10*time.Second)):
		return renderWorkerError("worker_start_timeout", context.DeadlineExceeded)
	}
}

func (r *NodeSlideRenderer) readWorker(cmd *exec.Cmd, stdout io.Reader, ready chan<- error) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxRenderOutputBytes)
	readySent := false
	for scanner.Scan() {
		var response workerRenderResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			continue
		}
		if response.Type == "ready" {
			if !readySent {
				ready <- nil
				readySent = true
			}
			continue
		}
		r.mu.Lock()
		pending := r.pending[response.RequestID]
		delete(r.pending, response.RequestID)
		r.mu.Unlock()
		if pending != nil {
			pending <- response
		}
	}
	waitErr := cmd.Wait()
	if !readySent {
		ready <- renderWorkerError("worker_start", waitErr)
	}
	r.workerExited(cmd, waitErr)
}

func (r *NodeSlideRenderer) workerExited(cmd *exec.Cmd, err error) {
	r.mu.Lock()
	if r.cmd != cmd {
		r.mu.Unlock()
		return
	}
	r.cmd, r.stdin, r.ready = nil, nil, nil
	pending := r.pending
	r.pending = map[string]chan workerRenderResponse{}
	r.mu.Unlock()
	for requestID, channel := range pending {
		channel <- workerRenderResponse{
			RequestID: requestID, Error: fmt.Sprintf("render worker crashed: %v", err),
			ErrorCode: model.CodeInternal,
		}
	}
}

func (r *NodeSlideRenderer) removePending(requestID string) {
	r.mu.Lock()
	delete(r.pending, requestID)
	r.mu.Unlock()
}

func (r *NodeSlideRenderer) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	cmd := r.cmd
	stdin := r.stdin
	done := r.done
	r.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			select {
			case <-done:
			case <-time.After(time.Second):
				return errors.New("render worker did not exit after SIGKILL")
			}
		}
	}
	return nil
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
	themes   ThemeLoader
}

func (slideRenderTool) Schema() ToolSchema {
	return tooldefinition.Get("render_slide")
}

func (t slideRenderTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	if err := validateToolArguments(t.Schema(), input.Args); err != nil {
		return argumentFailure(err)
	}
	slideID := stringValue(input.Args["slide_id"])
	target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
	if !input.Scope.ContainsSlide(slideID) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested render target is outside the current run scope")
	}
	frame, frameErr := runtimeFrameForRender(input.Context, input.ProjectDir, input.Session, slideID)
	if frameErr != nil {
		return internalToolFailure(frameErr)
	}
	html, source, err := readArtifact(input.ProjectDir, input.Session, slideHTMLRef(slideID))
	if err != nil {
		return resourceReadToolFailure(err, target)
	}
	if _, htmlErr := validateHTML(html); htmlErr != nil {
		return internalToolFailure(classifyRenderError(htmlErr))
	}
	if t.themes == nil {
		return failedToolResult(model.CodeInternal, "theme runtime is unavailable")
	}
	theme, themeErr := t.themes.Get(input.Context.Project.ThemeID)
	if themeErr != nil || theme.ContentState != "ready" {
		return failedToolResult(model.CodeInternal, "theme is unavailable")
	}
	baseCSS := runtimeassets.BaseCSS()
	normalizedHTML, normalizeErr := runtimehtml.Normalize(html, theme.ID)
	if normalizeErr != nil {
		return internalToolFailure(normalizeErr)
	}
	frame.Appearance = runtimeassets.Appearance(theme.ID, []byte(theme.CSS))
	before, err := currentRenderProof(input.Context, input.ProjectDir, input.Session, slideID, hashBytes(html))
	if err != nil {
		return internalToolFailure(err)
	}
	if before.FrameContextHash != spec.RuntimeFrameHash(frame) {
		return renderConflictFailure("slide appearance changed before rendering; render again")
	}
	if image, diagnostics, ok := cachedSlideRender(ctx, input, before); ok {
		return finishSlideRender(ctx, input, image, before, diagnostics, source, true)
	}
	if t.renderer == nil {
		return internalToolFailure(classifyRenderError(renderWorkerError("renderer_missing", ErrRenderWorkerUnavailable)))
	}
	screenshotID := "shot_" + uuid.NewString()
	runID := input.RunID
	if runID == "" {
		runID = "adhoc"
	}
	screenshotDir := filepath.Join(input.ProjectDir, ".runtime", "renders", runID)
	if err := os.MkdirAll(screenshotDir, 0o700); err != nil {
		return persistenceToolFailure(err)
	}
	screenshotPath := filepath.Join(screenshotDir, screenshotID+".png")
	assetsDir, err := runtimeassets.RenderAssetDir()
	if err != nil {
		return internalToolFailure(err)
	}
	request := RenderRequest{
		RuntimeAssetsDir: assetsDir,
		RunID:            runID, ProjectDir: input.ProjectDir, SlideID: slideID, HTML: string(normalizedHTML),
		ScreenshotPath: screenshotPath, ViewportWidth: frame.Canvas.Width,
		ViewportHeight: frame.Canvas.Height, TimeoutMS: 15000,
		Frame: frame, BaseCSS: string(baseCSS), ThemeID: theme.ID, ThemeCSS: theme.CSS,
	}
	started := time.Now()
	diagnostics, err := t.renderer.Render(ctx, request)
	// Rendering has no authored effect. Bound worker recovery before exposing a
	// terminal infrastructure failure to the model.
	if errors.Is(err, ErrRenderWorkerUnavailable) && ctx.Err() == nil {
		_ = os.Remove(screenshotPath)
		diagnostics, err = t.renderer.Render(ctx, request)
	}
	if err != nil {
		_ = os.Remove(screenshotPath)
		return internalToolFailure(classifyRenderError(err))
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
		return internalToolFailure(classifyRenderError(renderWorkerError(
			"screenshot_output", errors.New("browser screenshot is missing or exceeds the output limit"),
		)))
	}
	sourceHash, err := renderArtifactHash(input, slideID)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return resourceReadToolFailure(err, target)
	}
	if sourceHash != hashBytes(html) {
		_ = os.Remove(screenshotPath)
		return renderConflictFailure("slide HTML changed while rendering; render again")
	}
	proof, err := currentRenderProof(input.Context, input.ProjectDir, input.Session, slideID, sourceHash)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return internalToolFailure(err)
	}
	if proof.FrameContextHash != before.FrameContextHash || proof.SourceHash != before.SourceHash {
		_ = os.Remove(screenshotPath)
		return renderConflictFailure("slide dependencies changed while rendering; render again")
	}
	proof.ResourceHash, err = renderimage.ResourceHash(input.ProjectDir, diagnostics.ResourceHashes)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return internalToolFailure(err)
	}
	if proof.ResourceHash != renderimage.ResourceSnapshotHash(diagnostics.ResourceHashes) {
		_ = os.Remove(screenshotPath)
		return renderConflictFailure("slide resources changed while rendering; render again")
	}
	diagnosticJSON, err := json.Marshal(diagnostics)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return internalToolFailure(err)
	}
	image := renderimage.Entry{
		ProjectID: input.Context.Project.ID, SlideID: slideID, RunID: runID, ScreenshotID: screenshotID,
		SourceHash: sourceHash, DependencyHash: proof.DependencyHash(),
		RenderedAt: time.Now().Unix(), ResourceHashes: diagnostics.ResourceHashes, Diagnostics: diagnosticJSON,
	}
	if err := renderimage.Publish(input.ProjectDir, image); err != nil {
		return persistenceToolFailure(err)
	}
	return finishSlideRender(ctx, input, image, proof, diagnostics, source, false)
}

func renderConflictFailure(reason string) ToolResult {
	return detailedToolFailure(model.CodeContentConflict, reason, map[string]any{"measure": "Render the latest saved sources again before reading or relying on the screenshot."})
}

func cachedSlideRender(ctx context.Context, input DomainToolInput, proof RenderProof) (renderimage.Entry, RenderDiagnostics, bool) {
	image, err := renderimage.Latest(input.ProjectDir, input.Context.Project.ID, proof.SlideID)
	if err != nil || image.SourceHash != proof.ArtifactHash || image.DependencyHash != proof.DependencyHash() {
		return renderimage.Entry{}, RenderDiagnostics{}, false
	}
	var diagnostics RenderDiagnostics
	if json.Unmarshal(image.Diagnostics, &diagnostics) != nil || diagnostics.FontStatus != "loaded" {
		return renderimage.Entry{}, RenderDiagnostics{}, false
	}
	if _, _, err := renderimage.Read(ctx, input.ProjectDir, input.Context.Project.ID, image.ImageRef()); err != nil {
		return renderimage.Entry{}, RenderDiagnostics{}, false
	}
	return image, diagnostics, true
}

func slideRenderResult(image renderimage.Entry, proof RenderProof, diagnostics RenderDiagnostics, source string, cached bool) ToolResult {
	slideID, sourceHash := image.SlideID, image.SourceHash
	target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
	blocking, warnings := renderIssues(target, diagnostics)
	screenshotRef := image.ImageRef()
	screenshotURL := "/api/v1/runs/" + image.RunID + "/screenshots/" + image.ScreenshotID
	width, height := spec.CanonicalCanvas().Width, spec.CanonicalCanvas().Height
	data := map[string]any{
		"model_diagnostics": modelRenderDiagnostics(diagnostics, width, height),
		"screenshot_ref":    screenshotRef, "screenshot_url": screenshotURL,
		"screenshot_id": image.ScreenshotID, "screenshot_run_id": image.RunID, "cached": cached,
		"image_path": image.ImagePath(),
		"slide_id":   slideID, "source": source,
		"hash":         sourceHash,
		"viewport":     map[string]int{"width": width, "height": height},
		"content_size": diagnostics.ContentSize, "overflow": diagnostics.Overflow,
		"out_of_bounds": diagnostics.OutOfBounds, "runtime_decorations": diagnostics.RuntimeDecorations, "console_errors": diagnostics.ConsoleErrors,
		"failed_resources": diagnostics.FailedResources, "font_status": diagnostics.FontStatus,
		"duration_ms": diagnostics.DurationMS, "blocking_issues": blocking, "warnings": warnings,
	}
	result := SuccessfulToolResult("slide rendered in isolated Chromium")
	if cached {
		result.Summary = "existing slide render reused"
	}
	result.Data = data
	result.Issues = append(blocking, warnings...)
	observationData := map[string]any{"slide_id": slideID, "diagnostics": modelRenderDiagnostics(diagnostics, width, height)}
	if len(blocking) > 0 {
		result.OK, result.Applied, result.Code, result.Retryable = false, false, CodeRenderFailed, false
		result.Summary = "渲染发现阻断问题，请根据诊断修改页面后重新渲染。"
		observationData = toolResultError(result).ModelObservation()
	}
	observationRaw, _ := json.Marshal(observationData)
	result.ObservationParts = []llm.ContentPart{
		{Type: "text", Text: string(observationRaw)},
		{Type: "image", ImageRef: image.ImageRef(), MIMEType: "image/png", Detail: "high"},
	}
	if len(blocking) > 0 {
		result.Evidence = []Evidence{newEvidence("render_diagnostic", target, sourceHash, data)}
		return result
	}
	evidence := newEvidence("render", target, sourceHash, data)
	evidence.Render = &proof
	result.Evidence = []Evidence{evidence, newEvidence("static", target, sourceHash, map[string]any{"valid": true})}
	return result
}

func runtimeFrameForRender(pack contextengine.ContextPack, projectDir string, session *RunSession, slideID string) (spec.RuntimeFrameContext, error) {
	target := Resource{Type: "slide", Part: "html", SlideID: slideID}
	outline, err := readPageOutline(pack, projectDir, session, slideID)
	if err != nil {
		return spec.RuntimeFrameContext{}, toolResultError(resourceReadToolFailure(err, target))
	}
	_, deck, _, _, err := effectiveDeckSources(pack, projectDir, session)
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	specRaw, _, err := readArtifact(projectDir, session, specSlideRef(slideID))
	if err != nil {
		return spec.RuntimeFrameContext{}, toolResultError(resourceReadToolFailure(err, Resource{Type: "slide", Part: "spec", SlideID: slideID}))
	}
	parsedSlide, err := spec.ParseStrictSourceJSON(specRaw, "spec")
	if err != nil {
		return spec.RuntimeFrameContext{}, toolResultError(savedResourceInvalid(err, Resource{Type: "slide", Part: "spec", SlideID: slideID}))
	}
	slide := *parsedSlide.(*spec.SlideSpec)
	appearance, err := runtimeassets.ProjectAppearance(projectDir, pack.Project.ThemeID)
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	logo, err := branding.RenderImage(projectDir, pack.Project.Logo)
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	frame, ok := spec.BuildRuntimeFrame(deck, outline, pack.Project.Decorations, logo, slideID, slide, appearance)
	if !ok {
		return spec.RuntimeFrameContext{}, model.NewAgentError(model.CodeInternal, "render_source", errors.New("runtime frame could not be built for a validated outline page"))
	}
	return frame, nil
}

func renderArtifactHash(input DomainToolInput, slideID string) (string, error) {
	if _, err := readPageOutline(input.Context, input.ProjectDir, input.Session, slideID); err != nil {
		return "", err
	}
	htmlRaw, _, err := readArtifact(input.ProjectDir, input.Session, slideHTMLRef(slideID))
	if err != nil {
		return "", err
	}
	return hashBytes(htmlRaw), nil
}

func renderIssues(target Resource, diagnostics RenderDiagnostics) ([]Issue, []Issue) {
	blocking, warnings := []Issue{}, []Issue{}
	if diagnostics.Overflow["horizontal"] || diagnostics.Overflow["vertical"] {
		blocking = append(blocking, Issue{
			Code: "RENDER_OVERFLOW", Severity: SeverityError, Resource: target,
			Summary: "slide content overflows the fixed PPT viewport", Action: "edit the layout and render again",
		})
	}
	if len(diagnostics.OutOfBounds) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_CLIPPING", Severity: SeverityError, Resource: target,
			Summary: fmt.Sprintf("%d elements are outside the slide stage (this does not prove clipping)", len(diagnostics.OutOfBounds)),
			Action:  "edit the out-of-bounds elements and render again",
		})
	}
	if len(diagnostics.ConsoleErrors) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_CONSOLE_ERROR", Severity: SeverityError, Resource: target,
			Summary: strings.Join(diagnostics.ConsoleErrors, "; "), Action: "fix page runtime errors and render again",
		})
	}
	if len(diagnostics.FailedResources) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_RESOURCE_FAILED", Severity: SeverityError, Resource: target,
			Summary: strings.Join(diagnostics.FailedResources, "; "), Action: "use controlled project resources and render again",
		})
	}

	return blocking, warnings
}

// Keep worker measurements internal; the model sees only actionable diagnostics.
func modelRenderDiagnostics(d RenderDiagnostics, width, height int) map[string]any {
	axis := func(content, canvas int, dimension string) string {
		if content > canvas+1 {
			return fmt.Sprintf("溢出：内容%s %dpx > 画布%s %dpx，超出 %dpx。", dimension, content, dimension, canvas, content-canvas)
		}
		return fmt.Sprintf("未溢出：内容%s %dpx，画布%s %dpx。", dimension, content, dimension, canvas)
	}
	bounds := d.OutOfBounds
	if bounds == nil {
		bounds = []map[string]any{}
	}
	errors := d.ConsoleErrors
	if errors == nil {
		errors = []string{}
	}
	failed := d.FailedResources
	if failed == nil {
		failed = []string{}
	}
	return map[string]any{"overflow": map[string]string{"horizontal": axis(d.ContentSize["width"], width, "宽度"), "vertical": axis(d.ContentSize["height"], height, "高度")}, "out_of_bounds": bounds, "console_errors": errors, "failed_resources": failed}
}
