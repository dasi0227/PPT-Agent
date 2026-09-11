package workflow

import (
	"strings"
	"testing"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

func TestReferenceMessagePartsPreserveMixedOrderAndDOMComment(t *testing.T) {
	attachment := model.AttachmentReference{ID: "att_one", OriginalName: "brand.png", MediaType: "image/png"}
	selection := model.DOMSelection{SelectionID: "sel_one", MarkerNo: 3, Comment: "字号缩小", SlideID: "sli_one"}
	parts := referenceMessageParts("调整这两处", "pro_one", []model.AttachmentReference{attachment}, []model.DOMSelection{selection}, []model.ReferenceOrderItem{
		{Kind: "dom", RefID: "sel_one"}, {Kind: "image", RefID: "att_one"},
	})
	if len(parts) != 4 || !strings.Contains(parts[1].Text, "<selected_dom>") || !strings.Contains(parts[1].Text, "字号缩小") || parts[2].Type != "text" || parts[3].Type != "image" {
		t.Fatalf("parts = %#v", parts)
	}
}
