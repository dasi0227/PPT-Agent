package spec

// The presentation canvas is a product-wide contract. Slide authors always
// design at these CSS pixel dimensions; every Runtime surface fits this canvas
// into its available viewport without changing the authored coordinates.
const (
	CanvasWidth       = 1920
	CanvasHeight      = 1080
	CanvasAspectRatio = "16:9"
)

type RuntimeCanvas struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AspectRatio string `json:"aspect_ratio"`
}

func CanonicalCanvas() RuntimeCanvas {
	return RuntimeCanvas{Width: CanvasWidth, Height: CanvasHeight, AspectRatio: CanvasAspectRatio}
}
