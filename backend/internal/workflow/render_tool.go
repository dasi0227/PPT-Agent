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
)

const (
	renderViewportWidth  = 1600
	renderViewportHeight = 900
	maxRenderOutputBytes = 1024 * 1024
)

type RenderRequest struct {
	RequestID      string `json:"request_id,omitempty"`
	RunID          string `json:"run_id"`
	ProjectDir     string `json:"project_dir"`
	StagingDir     string `json:"staging_dir,omitempty"`
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
	RequestID   string            `json:"request_id"`
	Type        string            `json:"type"`
	OK          bool              `json:"ok"`
	Error       string            `json:"error"`
	Diagnostics RenderDiagnostics `json:"diagnostics"`
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
	raw, err := json.Marshal(request)
	if err != nil {
		return RenderDiagnostics{}, err
	}
	response := make(chan workerRenderResponse, 1)
	r.mu.Lock()
	if r.closed || r.stdin == nil {
		r.mu.Unlock()
		return RenderDiagnostics{}, errors.New("render worker is unavailable")
	}
	r.pending[request.RequestID] = response
	stdin := r.stdin
	r.mu.Unlock()
	r.writeMu.Lock()
	_, writeErr := stdin.Write(append(raw, '\n'))
	r.writeMu.Unlock()
	if writeErr != nil {
		r.removePending(request.RequestID)
		return RenderDiagnostics{}, fmt.Errorf("render worker write failed: %w", writeErr)
	}
	select {
	case result := <-response:
		if !result.OK {
			return RenderDiagnostics{}, errors.New(result.Error)
		}
		return result.Diagnostics, nil
	case <-commandCtx.Done():
		r.removePending(request.RequestID)
		return RenderDiagnostics{}, fmt.Errorf("render worker timed out: %w", commandCtx.Err())
	}
}

func (r *NodeSlideRenderer) ensureWorker(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("render worker is closed")
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
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		r.mu.Unlock()
		return err
	}
	cmd.Stderr = os.Stderr
	ready := make(chan error, 1)
	r.cmd, r.stdin, r.ready = cmd, stdin, ready
	if err := cmd.Start(); err != nil {
		r.cmd, r.stdin, r.ready = nil, nil, nil
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	go r.readWorker(cmd, stdout, ready)
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(minDuration(r.config.Timeout, 10*time.Second)):
		return errors.New("render worker startup timed out")
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
		ready <- fmt.Errorf("render worker failed to start: %w", waitErr)
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
		channel <- workerRenderResponse{RequestID: requestID, Error: fmt.Sprintf("render worker crashed: %v", err)}
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
	target := Resource{Type: "slide", SlideID: slideID, Part: "html"}
	if !stableSlideID.MatchString(slideID) || slideID == "current" {
		return failedToolResult(CodeModelInvalid, "slide_id must be a stable slide identifier", false)
	}
	if !input.Scope.AllowsRead(target) {
		return failedToolResult(ErrTargetOutOfScope.Error(), "requested render target is outside the current run scope", false)
	}
	html, source, err := readArtifact(input.ProjectDir, input.Transaction, slideHTMLRef(slideID))
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
	if input.Transaction != nil {
		request.StagingDir = input.Transaction.Root()
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
	proof, err := currentMaterializationProof(t.pack, input.ProjectDir, input.Transaction, slideID, sourceHash)
	if err != nil {
		_ = os.Remove(screenshotPath)
		return failedToolResult(CodeRenderFailed, err.Error(), true)
	}
	evidence := newEvidence("render", target, sourceHash, data)
	evidence.Materialization = &proof
	result.Evidence = []Evidence{evidence}
	return result
}

func renderHashWithoutTransaction(pack contextengine.ContextPack, input DomainToolInput, slideID string) (string, error) {
	if input.Transaction != nil {
		return renderSourceHash(pack, input.Transaction, slideID)
	}
	designRaw, _, err := readArtifact(input.ProjectDir, nil, designRef(pack))
	if err != nil {
		return "", err
	}
	specRaw, _, err := readArtifact(input.ProjectDir, nil, specSlideRef(slideID))
	if err != nil {
		return "", err
	}
	htmlRaw, _, err := readArtifact(input.ProjectDir, nil, slideHTMLRef(slideID))
	if err != nil {
		return "", err
	}
	return MaterializationSourceHash(slideID, designRaw, specRaw, htmlRaw), nil
}

func presentationRevision(pack contextengine.ContextPack, input DomainToolInput, slideID string) int {
	if input.Transaction != nil && input.Transaction.IsStaged(slideHTMLRef(slideID)) {
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
