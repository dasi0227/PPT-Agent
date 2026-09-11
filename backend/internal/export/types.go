package export

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/dasi0227/PPT-Agent/backend/internal/spec"
)

type Format string

const (
	FormatPNG  Format = "png"
	FormatPDF  Format = "pdf"
	FormatHTML Format = "html"
)

func (f Format) Valid() bool { return f == FormatPNG || f == FormatPDF || f == FormatHTML }

type Status string

const (
	StatusAccepted   Status = "accepted"
	StatusRunning    Status = "running"
	StatusReady      Status = "ready"
	StatusDelivering Status = "delivering"
	StatusConsumed   Status = "consumed"
	StatusFailed     Status = "failed"
	StatusCanceled   Status = "canceled"
)

func (s Status) Terminal() bool {
	return s == StatusConsumed || s == StatusFailed || s == StatusCanceled
}

type Phase string

const (
	PhaseSnapshotting Phase = "snapshotting"
	PhaseRendering    Phase = "rendering"
	PhasePackaging    Phase = "packaging"
)

var (
	ErrNotFound           = errors.New("export operation not found")
	ErrGone               = errors.New("export operation is gone")
	ErrAlreadyActive      = errors.New("project already has an active export")
	ErrDownloadInProgress = errors.New("export download is in progress")
	ErrNotReady           = errors.New("export artifact is not ready")
)

type MissingSlide struct {
	SlideID string `json:"slide_id"`
	Ordinal int    `json:"ordinal"`
	Title   string `json:"title"`
}

type PublicError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
	Retryable bool           `json:"retryable"`
}

type SlideSnapshot struct {
	ID       string
	Title    string
	Ordinal  int
	HTML     []byte
	Frame    spec.RuntimeFrameContext
	FileName string
}

type Snapshot struct {
	ProjectID    string
	ProjectTitle string
	Root         string
	ManifestRaw  []byte
	OutlineRaw   []byte
	DesignRaw    []byte
	BaseCSS      []byte
	ThemeCSS     []byte
	ThemeID      string
	Slides       []SlideSnapshot
	Attachments  []string
	SourceDigest string
}

type Artifact struct {
	Path     string `json:"-"`
	Name     string `json:"filename"`
	MIME     string `json:"mime_type"`
	Size     int64  `json:"size_bytes"`
	Download string `json:"download_url"`
}

type View struct {
	ID             string       `json:"id"`
	ProjectID      string       `json:"project_id"`
	Format         Format       `json:"format"`
	Status         Status       `json:"status"`
	Phase          Phase        `json:"phase,omitempty"`
	CompletedPages int          `json:"completed_pages"`
	TotalPages     int          `json:"total_pages"`
	CurrentSlideID string       `json:"current_slide_id,omitempty"`
	CurrentOrdinal int          `json:"current_ordinal,omitempty"`
	Warnings       []string     `json:"warnings"`
	Artifact       *Artifact    `json:"artifact,omitempty"`
	Error          *PublicError `json:"error,omitempty"`
	EventsURL      string       `json:"events_url"`
}

type Event struct {
	Seq     int64
	Type    string
	Payload string
}

type Operation struct {
	mu sync.Mutex

	ID              string
	ProjectID       string
	ClientRequestID string
	Format          Format
	Status          Status
	Phase           Phase
	CompletedPages  int
	TotalPages      int
	CurrentSlideID  string
	CurrentOrdinal  int
	Warnings        []string
	Artifact        *Artifact
	Error           *PublicError
	Snapshot        Snapshot
	CreatedAt       time.Time
	cancel          context.CancelFunc
	seq             int64
	events          []Event
	subscribers     map[int]chan Event
	nextSubscriber  int
	done            chan struct{}
}

func (o *Operation) viewLocked() View {
	var artifact *Artifact
	if o.Artifact != nil {
		copy := *o.Artifact
		artifact = &copy
	}
	warnings := append([]string(nil), o.Warnings...)
	return View{ID: o.ID, ProjectID: o.ProjectID, Format: o.Format, Status: o.Status, Phase: o.Phase,
		CompletedPages: o.CompletedPages, TotalPages: o.TotalPages, CurrentSlideID: o.CurrentSlideID,
		CurrentOrdinal: o.CurrentOrdinal, Warnings: warnings, Artifact: artifact, Error: o.Error,
		EventsURL: "/api/v1/exports/" + o.ID + "/events"}
}

func (o *Operation) View() View {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.viewLocked()
}

func eventPayload(view View) string {
	raw, _ := json.Marshal(view)
	return string(raw)
}
