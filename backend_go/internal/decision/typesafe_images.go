package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"mime/multipart"
	"net/textproto"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

const (
	jevImageMinSide    = 280
	jevImageMaxSide    = 1568
	jevImageMaxBytes   = 16 << 20
	jevRequestMaxBytes = 32 << 20
	// Bound decoding memory before allocating pixels for an untrusted source.
	jevSourceMaxPixels = 32 << 20
)

type jevAsset struct {
	Type string `json:"type"`
	Part string `json:"part"`
}

type jevImagePart struct {
	Name  string
	Image *ImageInput
}

type jevPayload struct {
	Data        []byte
	ContentType string
	HasImages   bool
}

// buildJevPayload creates request-local image copies only when Evaluate is called.
// Rendered originals and assessment cache records never contain these copies.
func buildJevPayload(ctx context.Context, req Request, model string) (jevPayload, error) {
	if err := ctx.Err(); err != nil {
		return jevPayload{}, err
	}
	assets := map[string]jevAsset{}
	var images []jevImagePart
	if len(req.Input) > 0 {
		parts := make([]string, 0, len(req.Input))
		for _, part := range req.Input {
			if part.Image == nil {
				parts = append(parts, part.Text)
				continue
			}
			id := uuid.NewString()
			assets[id] = jevAsset{Type: "image", Part: id}
			images = append(images, jevImagePart{Name: id, Image: part.Image})
			parts = append(parts, id)
		}
		req.State, req.Input = parts, nil
	}
	// Preserve the existing text-only guard. Image token accounting belongs to
	// Jev; applying UTF-8 text bounds to multimodal input would reject valid HTML.
	if len(images) == 0 && !Fits(req) {
		return jevPayload{}, &Error{Kind: "capacity"}
	}
	raw, err := json.Marshal(struct {
		Model string `json:"model"`
		Request
		Assets map[string]jevAsset `json:"assets,omitempty"`
	}{model, req, assets})
	if err != nil {
		return jevPayload{}, &Error{Kind: "request"}
	}
	if len(raw) > jevRequestMaxBytes {
		return jevPayload{}, &Error{Kind: "capacity"}
	}
	if len(images) == 0 {
		return jevPayload{Data: raw, ContentType: "application/json"}, nil
	}

	var body jevRequestBuffer
	writer := multipart.NewWriter(&body)
	requestPart, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="request"`},
		"Content-Type":        {"application/json"},
	})
	if err != nil {
		return jevPayload{}, err
	}
	if _, err := requestPart.Write(raw); err != nil {
		return jevPayload{}, err
	}
	for _, part := range images {
		pixels, err := prepareJevImage(ctx, part.Image)
		if err != nil {
			return jevPayload{}, err
		}
		imagePart, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Disposition": {`form-data; name="` + part.Name + `"; filename="` + part.Name + `.png"`},
			"Content-Type":        {"image/png"},
		})
		if err != nil {
			return jevPayload{}, err
		}
		if _, err := imagePart.Write(pixels); err != nil {
			return jevPayload{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return jevPayload{}, err
	}
	return jevPayload{Data: body.Bytes(), ContentType: writer.FormDataContentType(), HasImages: true}, ctx.Err()
}

type jevRequestBuffer struct{ bytes.Buffer }

func (b *jevRequestBuffer) Write(p []byte) (int, error) {
	if len(p) > jevRequestMaxBytes-b.Len() {
		return 0, &Error{Kind: "capacity"}
	}
	return b.Buffer.Write(p)
}

// prepareJevImage preserves aspect ratio, shrinks the long edge when necessary,
// and pads short edges to 280 pixels. The PNG exists only in memory.
func prepareJevImage(ctx context.Context, input *ImageInput) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.MIMEType != "image/png" && input.MIMEType != "image/jpeg" {
		return nil, &Error{Kind: "unsupported_image"}
	}
	if len(input.Data) > jevRequestMaxBytes {
		return nil, &Error{Kind: "capacity"}
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(input.Data))
	if err != nil || (format != "png" && format != "jpeg") || input.MIMEType != "image/"+format || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, &Error{Kind: "image"}
	}
	if cfg.Width > jevSourceMaxPixels/cfg.Height {
		return nil, &Error{Kind: "capacity"}
	}
	source, _, err := image.Decode(bytes.NewReader(input.Data))
	if err != nil {
		return nil, &Error{Kind: "image"}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	width, height := cfg.Width, cfg.Height
	if longest := max(width, height); longest > jevImageMaxSide {
		width = max(1, (width*jevImageMaxSide+longest/2)/longest)
		height = max(1, (height*jevImageMaxSide+longest/2)/longest)
	}
	output := image.NewRGBA(image.Rect(0, 0, max(width, jevImageMinSide), max(height, jevImageMinSide)))
	draw.Draw(output, output.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	x, y := (output.Bounds().Dx()-width)/2, (output.Bounds().Dy()-height)/2
	area := image.Rect(x, y, x+width, y+height)
	if width == cfg.Width && height == cfg.Height {
		draw.Draw(output, area, source, source.Bounds().Min, draw.Over)
	} else {
		draw.CatmullRom.Scale(output, area, source, source.Bounds(), draw.Over, nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&encoded, output); err != nil {
		return nil, &Error{Kind: "image"}
	}
	if encoded.Len() > jevImageMaxBytes {
		return nil, &Error{Kind: "capacity"}
	}
	return encoded.Bytes(), ctx.Err()
}
