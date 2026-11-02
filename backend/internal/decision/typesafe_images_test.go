package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/config"
)

func decisionPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	pixels := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(pixels, pixels.Bounds(), image.NewUniform(color.RGBA{R: 210, G: 50, B: 30, A: 255}), image.Point{}, draw.Src)
	var data bytes.Buffer
	if err := png.Encode(&data, pixels); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestJevImagesPreserveMaterialAndReplayMultipartOnRetry(t *testing.T) {
	original := decisionPNG(t, 1920, 1080)
	before := bytes.Clone(original)
	html := "<html>" + strings.Repeat("complete HTML ", 3000) + "</html>"
	calls := 0
	var firstBody []byte
	var firstContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/systemone" || r.Header.Get("Authorization") != "Bearer image-key" || r.Header.Get("TypeSafe-Danger-Zone-Images") != "true" {
			t.Error("wrong image endpoint, credentials or feature header")
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if calls == 1 {
			firstBody, firstContentType = raw, r.Header.Get("Content-Type")
		} else if !bytes.Equal(raw, firstBody) || r.Header.Get("Content-Type") != firstContentType {
			t.Error("retry changed or dropped multipart material")
		}
		kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || kind != "multipart/form-data" || params["boundary"] == "" {
			t.Fatal("image request is not multipart")
		}
		reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
		requestPart, err := reader.NextPart()
		if err != nil || requestPart.FormName() != "request" || requestPart.Header.Get("Content-Type") != "application/json" {
			t.Fatal("first multipart part must be request JSON")
		}
		var body struct {
			Model     string                     `json:"model"`
			State     []string                   `json:"state"`
			Assets    map[string]jevAsset        `json:"assets"`
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(requestPart).Decode(&body); err != nil || body.Model != "jev-test" || len(body.State) != 3 || len(body.Assets) != 1 || len(body.Questions) != 1 {
			t.Fatal("invalid image request JSON")
		}
		if body.State[0] != "requirements" || body.State[2] != html {
			t.Error("text order or complete HTML lost")
		}
		asset, ok := body.Assets[body.State[1]]
		if !ok || asset.Type != "image" || asset.Part == "" {
			t.Fatal("state does not reference its image asset")
		}
		imagePart, err := reader.NextPart()
		if err != nil || imagePart.FormName() != asset.Part || imagePart.Header.Get("Content-Type") != "image/png" {
			t.Fatal("image asset does not match its file part")
		}
		pixels, err := png.Decode(imagePart)
		if err != nil || pixels.Bounds().Dx() != 1568 || pixels.Bounds().Dy() != 882 {
			t.Fatal("slide was not scaled to 1568x882")
		}
		if _, err := reader.NextPart(); err != io.EOF {
			t.Error("unexpected multipart material")
		}
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"jev-test","answers":{"visible":{"type":"noul","noul":1}},"usage":{"input_tokens":42,"output_tokens":1}}`))
	}))
	defer server.Close()
	snapshot, err := NewSnapshot(&config.DecisionConfig{Provider: "jev", Jev: &config.JevConfig{BaseURL: server.URL + "/v1", Model: "jev-test", Key: "image-key"}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := snapshot.Provider.Evaluate(context.Background(), Request{
		Input:     []InputPart{{Text: "requirements"}, {Image: &ImageInput{MIMEType: "image/png", Data: original}}, {Text: html}},
		Questions: map[string]Question{"visible": NoulQuestion{Instructions: "Is the content visible?"}},
	})
	if err != nil || calls != 2 || len(response.Invalid) != 0 || response.Usage.InputTokens != 42 {
		t.Fatalf("image evaluation failed: calls=%d response=%+v error=%v", calls, response, err)
	}
	if !bytes.Equal(original, before) {
		t.Fatal("evaluation modified the original screenshot")
	}
}

func TestJevImagePaddingAndInvalidInputs(t *testing.T) {
	for _, size := range []struct{ width, height, wantWidth, wantHeight int }{
		{1600, 100, 1568, 280},
		{100, 1600, 280, 1568},
		{120, 80, 280, 280},
		{800, 600, 800, 600},
	} {
		raw, err := prepareJevImage(context.Background(), &ImageInput{MIMEType: "image/png", Data: decisionPNG(t, size.width, size.height)})
		if err != nil {
			t.Fatal(err)
		}
		pixels, err := png.Decode(bytes.NewReader(raw))
		if err != nil || pixels.Bounds().Dx() != size.wantWidth || pixels.Bounds().Dy() != size.wantHeight {
			t.Fatalf("wrong output bounds for %+v: %v", size, err)
		}
		center := color.RGBAModel.Convert(pixels.At(size.wantWidth/2, size.wantHeight/2)).(color.RGBA)
		if center != (color.RGBA{R: 210, G: 50, B: 30, A: 255}) {
			t.Fatal("padding lost image content")
		}
	}
	client := &Client{}
	request := Request{Input: []InputPart{{Image: &ImageInput{MIMEType: "image/png", Data: []byte("invalid PNG")}}}, Questions: map[string]Question{"q": NoulQuestion{Instructions: "Visible?"}}}
	if _, err := client.Evaluate(context.Background(), request); Failure(err) != "image" {
		t.Fatalf("invalid image was not rejected before sending: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Evaluate(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled image request was processed: %v", err)
	}
}
