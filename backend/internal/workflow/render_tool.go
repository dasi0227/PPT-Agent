package workflow

import (
	"bufio"
	"bytes"
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

	"github.com/google/uuid"

	"github.com/dasi0227/PPT-Agent/backend/internal/contextengine"
	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimeassets"
	"github.com/dasi0227/PPT-Agent/backend/internal/runtimehtml"
	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
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
	Type           string                   `json:"type,omitempty"`
	RequestID      string                   `json:"request_id,omitempty"`
	RunID          string                   `json:"run_id"`
	ProjectDir     string                   `json:"project_dir"`
	SlideID        string                   `json:"slide_id"`
	HTML           string                   `json:"html"`
	ScreenshotPath string                   `json:"screenshot_path"`
	ViewportWidth  int                      `json:"viewport_width"`
	ViewportHeight int                      `json:"viewport_height"`
	TimeoutMS      int                      `json:"timeout_ms"`
	Frame          spec.RuntimeFrameContext `json:"frame"`
	BaseCSS        string                   `json:"base_css"`
	ThemeID        string                   `json:"theme_id"`
	ThemeCSS       string                   `json:"theme_css"`
}

type RenderDiagnostics struct {
	ScreenshotBytes int              `json:"screenshot_bytes"`
	ContentSize     map[string]int   `json:"content_size"`
	Overflow        map[string]bool  `json:"overflow"`
	Clipping        []map[string]any `json:"clipping"`
	RuntimeChrome   []string         `json:"runtime_chrome"`
	ConsoleErrors   []string         `json:"console_errors"`
	FailedResources []string         `json:"failed_resources"`
	FontStatus      string           `json:"font_status"`
	DurationMS      int64            `json:"duration_ms"`
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
	closed  bool
	slots   chan struct{}
}

type workerRenderResponse struct {
	RequestID             string            `json:"request_id"`
	Type                  string            `json:"type"`
	OK                    bool              `json:"ok"`
	Error                 string            `json:"error"`
	Diagnostics           RenderDiagnostics `json:"diagnostics"`
	PageCount             int               `json:"page_count"`
	InfrastructureFailure bool              `json:"-"`
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
			if result.InfrastructureFailure {
				return RenderDiagnostics{}, renderWorkerError("worker_process", errors.New(result.Error))
			}
			return RenderDiagnostics{}, errors.New(result.Error)
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
	r.cmd, r.stdin, r.ready = cmd, stdin, ready
	if err := cmd.Start(); err != nil {
		r.cmd, r.stdin, r.ready = nil, nil, nil
		r.mu.Unlock()
		return renderWorkerError("worker_start", err)
	}
	r.mu.Unlock()
	go r.readWorker(cmd, stdout, ready)
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
			InfrastructureFailure: true,
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
	r.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	return nil
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
	themes   ThemeLoader
}

func (slideRenderTool) Schema() ToolSchema {
	return ToolSchema{
		Name: "render_slide", Description: "Render one authorized slide in isolated Chromium and return screenshot-bound visual diagnostics.",
		Parameters: objectSchema([]string{"slide_id"}, map[string]any{
			"slide_id": map[string]any{
				"type": "string", "pattern": `^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`,
			},
			"visual_review": map[string]any{
				"type":        "boolean",
				"description": "Request the high-detail screenshot back for visual judgement. Omit for a lightweight diagnostics-only render.",
			},
		}),
	}
}

func (t slideRenderTool) Execute(ctx context.Context, input DomainToolInput) ToolResult {
	slideID := stringValue(input.Args["slide_id"])
	target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
	if !stableSlideID.MatchString(slideID) || slideID == "current" {
		return failedToolResult(CodeModelInvalid, "slide_id must be a stable slide identifier", false)
	}
	if !AllowsRead(input.Scope, target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested render target is outside the current run scope", false)
	}
	html, source, err := readArtifact(input.ProjectDir, input.Session, slideHTMLRef(slideID))
	if err != nil {
		if errorsIsNotExist(err) {
			return failedToolResult(CodeTargetNotFound, "slide HTML was not found", false)
		}
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	if _, htmlErr := validateHTML(html); htmlErr != nil {
		return failedToolResult(CodeRenderFailed, htmlErr.Error(), true)
	}
	frame, frameErr := runtimeFrameForRender(t.pack, input.ProjectDir, input.Session, slideID)
	if frameErr != nil {
		return failedToolResult(CodeRenderFailed, frameErr.Error(), true)
	}
	if t.renderer == nil {
		agentErr := classifyRenderError(renderWorkerError("renderer_missing", ErrRenderWorkerUnavailable))
		return failedToolResult(agentErr.Code, agentErr.Error(), agentErr.Retryable)
	}
	design, designErr := currentDesignForRender(t.pack, input.ProjectDir, input.Session)
	if designErr != nil || t.themes == nil {
		return failedToolResult(CodeRenderFailed, "theme runtime is unavailable", true)
	}
	theme, themeErr := t.themes.Get(design.Theme)
	if themeErr != nil {
		return failedToolResult(CodeRenderFailed, "theme is unavailable", false)
	}
	baseCSS := runtimeassets.BaseCSS()
	normalizedHTML, normalizeErr := runtimehtml.Normalize(html, theme.ID)
	if normalizeErr != nil {
		return failedToolResult(CodeRenderFailed, normalizeErr.Error(), false)
	}
	screenshotID := "shot_" + uuid.NewString()
	runID := input.RunID
	if runID == "" {
		runID = "adhoc"
	}
	screenshotDir := filepath.Join(input.ProjectDir, ".runtime", "renders", runID)
	if err := os.MkdirAll(screenshotDir, 0o700); err != nil {
		agentErr := classifyRenderError(renderWorkerError("screenshot_directory", err))
		return failedToolResult(agentErr.Code, agentErr.Error(), agentErr.Retryable)
	}
	screenshotPath := filepath.Join(screenshotDir, screenshotID+".png")
	request := RenderRequest{
		RunID: runID, ProjectDir: input.ProjectDir, SlideID: slideID, HTML: string(normalizedHTML),
		ScreenshotPath: screenshotPath, ViewportWidth: frame.Canvas.Width,
		ViewportHeight: frame.Canvas.Height, TimeoutMS: 15000,
		Frame: frame, BaseCSS: string(baseCSS), ThemeID: theme.ID, ThemeCSS: theme.CSS,
	}
	started := time.Now()
	diagnostics, err := t.renderer.Render(ctx, request)
	if err != nil {
		_ = os.Remove(screenshotPath)
		agentErr := classifyRenderError(err)
		return failedToolResult(agentErr.Code, agentErr.Error(), agentErr.Retryable)
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
		agentErr := classifyRenderError(renderWorkerError(
			"screenshot_output", errors.New("browser screenshot is missing or exceeds the output limit"),
		))
		return failedToolResult(agentErr.Code, agentErr.Error(), agentErr.Retryable)
	}
	sourceHash, err := renderHashWithoutSession(t.pack, input, slideID, theme.CSS)
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
		"viewport":     map[string]int{"width": frame.Canvas.Width, "height": frame.Canvas.Height},
		"content_size": diagnostics.ContentSize, "overflow": diagnostics.Overflow,
		"clipping": diagnostics.Clipping, "runtime_chrome": diagnostics.RuntimeChrome, "console_errors": diagnostics.ConsoleErrors,
		"failed_resources": diagnostics.FailedResources, "font_status": diagnostics.FontStatus,
		"duration_ms": diagnostics.DurationMS, "blocking_issues": blocking, "warnings": warnings,
	}
	result := SuccessfulToolResult("slide rendered in isolated Chromium")
	result.Data = data
	result.Issues = append(blocking, warnings...)
	observationData := map[string]any{
		"ok": len(blocking) == 0, "code": CodeRenderFailed,
		"tool_call_id": input.CallID,
		"resource":     target, "slide_id": slideID, "diagnostics": map[string]any{
			"content_size": diagnostics.ContentSize, "overflow": diagnostics.Overflow,
			"clipping": diagnostics.Clipping, "runtime_chrome": diagnostics.RuntimeChrome, "console_errors": diagnostics.ConsoleErrors,
			"failed_resources": diagnostics.FailedResources, "font_status": diagnostics.FontStatus,
		},
		"source_hash": sourceHash,
	}
	if len(blocking) == 0 {
		delete(observationData, "code")
	}
	observationRaw, _ := json.Marshal(observationData)
	parts := []llm.ContentPart{{Type: "text", Text: string(observationRaw)}}
	visualReview, _ := input.Args["visual_review"].(bool)
	if len(blocking) > 0 || visualReview {
		parts = append(parts, llm.ContentPart{Type: "image", ImageRef: screenshotRef, MIMEType: "image/png", Detail: "high"})
	}
	result.ObservationParts = parts
	if len(blocking) > 0 {
		result.Evidence = []Evidence{newEvidence("render_diagnostic", target, sourceHash, data)}
		result.OK, result.Code, result.Retryable = false, CodeRenderFailed, false
		return result
	}
	proof, err := currentMaterializationProof(t.pack, input.ProjectDir, input.Session, slideID, sourceHash)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	evidence := newEvidence("render", target, sourceHash, data)
	evidence.Materialization = &proof
	result.Evidence = []Evidence{evidence}
	return result
}

func runtimeFrameForRender(pack contextengine.ContextPack, projectDir string, session *RunSession, slideID string) (spec.RuntimeFrameContext, error) {
	deckRaw, _, err := readArtifact(projectDir, session, manifestRef(pack))
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	outlineRaw, _, err := readArtifact(projectDir, session, outlineRef(pack))
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	designRaw, _, err := readArtifact(projectDir, session, designRef(pack))
	if err != nil {
		return spec.RuntimeFrameContext{}, err
	}
	var deck spec.Manifest
	var outline spec.Outline
	var design spec.Design
	if json.Unmarshal(deckRaw, &deck) != nil || json.Unmarshal(outlineRaw, &outline) != nil || json.Unmarshal(designRaw, &design) != nil {
		return spec.RuntimeFrameContext{}, errors.New("runtime frame source is invalid")
	}
	frame, ok := spec.BuildRuntimeFrame(deck, outline, design, slideID)
	if !ok {
		return spec.RuntimeFrameContext{}, errors.New("slide is not present in the current outline")
	}
	return frame, nil
}

func currentDesignForRender(pack contextengine.ContextPack, projectDir string, session *RunSession) (spec.Design, error) {
	raw, _, err := readArtifact(projectDir, session, designRef(pack))
	if err != nil {
		return spec.Design{}, err
	}
	var design spec.Design
	if err := json.Unmarshal(raw, &design); err != nil {
		return spec.Design{}, err
	}
	return design, nil
}

func renderHashWithoutSession(pack contextengine.ContextPack, input DomainToolInput, slideID, themeCSS string) (string, error) {
	htmlRaw, _, err := readArtifact(input.ProjectDir, input.Session, slideHTMLRef(slideID))
	if err != nil {
		return "", err
	}
	return hashBytes(append(append([]byte{}, htmlRaw...), []byte(themeCSS)...)), nil
}

func presentationRevision(pack contextengine.ContextPack, input DomainToolInput, slideID string) int {
	if input.Session != nil && input.Session.HasChange(slideHTMLRef(slideID)) {
		return pack.Revisions.SlideHTML[slideID] + 1
	}
	return pack.Revisions.SlideHTML[slideID]
}

func renderIssues(target Resource, diagnostics RenderDiagnostics) ([]Issue, []Issue) {
	blocking, warnings := []Issue{}, []Issue{}
	if diagnostics.Overflow["horizontal"] || diagnostics.Overflow["vertical"] {
		blocking = append(blocking, Issue{
			Code: "RENDER_OVERFLOW", Severity: SeverityError, Resource: target,
			Summary: "slide content overflows the fixed PPT viewport", Action: "edit the layout and render again",
		})
	}
	if len(diagnostics.Clipping) > 0 {
		blocking = append(blocking, Issue{
			Code: "RENDER_CLIPPING", Severity: SeverityError, Resource: target,
			Summary: fmt.Sprintf("%d elements are clipped or outside the slide stage", len(diagnostics.Clipping)),
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
	if diagnostics.FontStatus != "loaded" {
		warnings = append(warnings, Issue{
			Code: "RENDER_FONT_STATUS", Severity: SeverityWarning, Resource: target,
			Summary: "font readiness: " + diagnostics.FontStatus,
		})
	}
	return blocking, warnings
}
