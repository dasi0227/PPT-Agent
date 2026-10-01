package schemas

import "fmt"

// Called after structural validation for every Design read/write path, including
// strict source parsing and Agent edits. Missing decoration text does not free a slot.
func validateDecorationPositions(value any) error {
	decorations := value.(map[string]any)["decorations"].(map[string]any)
	labels := map[string]string{
		"page_number": "页码", "section_title": "章节标题", "deck_title": "演示标题", "key_message": "核心信息",
	}
	placements := map[string]string{
		"top-left": "左上", "top-center": "顶部居中", "top-right": "右上",
		"bottom-left": "左下", "bottom-center": "底部居中", "bottom-right": "右下",
		"left-edge": "左侧边", "right-edge": "右侧边",
	}
	occupied := map[string]string{}
	for _, key := range []string{"page_number", "section_title", "deck_title", "key_message"} {
		placement := decorations[key].(string)
		if placement == "none" {
			continue
		}
		if previous, exists := occupied[placement]; exists {
			return fmt.Errorf("页面装饰位置冲突：%s（%s）与%s（%s）均设为%s（%s），请调整其中一项的位置或显隐设置。", labels[previous], previous, labels[key], key, placements[placement], placement)
		}
		occupied[placement] = key
	}
	return nil
}
