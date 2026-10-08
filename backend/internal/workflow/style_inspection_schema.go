package workflow

func styleInspectionOutputSchema() map[string]any {
	styles := map[string]any{}
	for key, description := range map[string]string{
		"color": "Computed foreground color.", "background_color": "Computed background color on this element; may be transparent.",
		"background_image": "Computed background image, including gradients; color alone may not describe the visible surface.",
		"font_size":        "Computed font size.", "line_height": "Computed line height.",
		"display": "Computed display value.", "visibility": "Computed visibility.", "opacity": "Computed opacity.",
		"fill": "Computed SVG fill.", "stroke": "Computed SVG stroke.",
	} {
		styles[key] = outputString(description)
	}
	return outputArray("Present only when inspect_selectors was requested. Advisory computed styles identify CSS effects; they are not a contrast score, the originating CSS rule, or visual approval.", outputObject("One requested selector's result.", []string{"selector", "match_count", "elements"}, map[string]any{
		"selector":    outputString("CSS selector requested for this page."),
		"match_count": outputNumber("Total matches; only the first three are inspected. Zero means no matching element or an invalid selector."),
		"error":       outputString("Selector parsing error, when invalid; the screenshot remains available."),
		"elements": outputArray("Up to three matched elements in document order.", outputObject("Actual style values of one matched element.", []string{"tag", "id", "class", "computed", "background_layers"}, map[string]any{
			"tag": outputString("Lowercase element tag."), "id": outputString("Element ID, empty when absent."),
			"class":    outputString("Element class text, capped at 160 characters."),
			"computed": outputObject("Computed style after theme and local CSS are applied.", []string{"color", "background_color", "background_image", "font_size", "line_height", "display", "visibility", "opacity", "fill", "stroke"}, styles),
			"background_layers": outputArray("Up to eight nontransparent or image-bearing layers from this element through its ancestors, nearest first. These are CSS layers, not a composited pixel color; transparency, images and pseudo-elements may require screenshot inspection.", outputObject("One element or ancestor background layer.", []string{"tag", "id", "class", "color", "image", "opacity"}, map[string]any{
				"tag": outputString("Layer owner tag."), "id": outputString("Layer owner ID."), "class": outputString("Layer owner class text, capped at 160 characters."),
				"color": outputString("Computed background color."), "image": outputString("Computed background image."), "opacity": outputString("Computed layer owner's opacity."),
			})),
		})),
	}))
}
