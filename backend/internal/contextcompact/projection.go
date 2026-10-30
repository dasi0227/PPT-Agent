package contextcompact

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/llm"
)

var compactDelimiters = func() *strings.Replacer {
	pairs := []string{}
	for _, tag := range []string{"previous_summary", "compressed_context", "retained_context", "compact_summary", "user", "assistant", "runtime_instruction", "tool_call", "tool_result"} {
		pairs = append(pairs, "<"+tag+">", "&lt;"+tag+"&gt;", "</"+tag+">", "&lt;/"+tag+"&gt;")
	}
	return strings.NewReplacer(pairs...)
}()

func escapeCompactText(text string) string { return compactDelimiters.Replace(text) }

func projectPartition(partition Partition) string {
	blocks := []string{}
	if partition.PreviousSummary != "" {
		blocks = append(blocks, "<previous_summary>\n"+escapeCompactText(partition.PreviousSummary)+"\n</previous_summary>")
	}
	blocks = append(blocks, "<compressed_context>\n"+projectMessages(partition.Compressed)+"</compressed_context>")
	blocks = append(blocks, "<retained_context>\n"+projectMessages(partition.Retained)+"</retained_context>")
	return strings.Join(blocks, "\n\n")
}

func projectMessages(messages []llm.Message) string {
	var text strings.Builder
	for _, message := range messages {
		tag := string(message.Role)
		if message.Role == llm.RoleTool {
			tag = "tool_result"
		}
		if m := message.Metadata; message.Role == llm.RoleUser && m != nil && m.Origin == "runtime" {
			tag = "runtime_instruction"
		}
		text.WriteString("<" + tag + ">\n")
		if message.ToolCallID != "" {
			text.WriteString("调用 ID：" + escapeCompactText(message.ToolCallID) + "\n")
		}
		if m := message.Metadata; m != nil {
			if m.Origin == "user" && message.Role == llm.RoleTool {
				text.WriteString("来源：用户回答或审批反馈（" + escapeCompactText(m.Kind) + "）\n")
			}
			for _, resource := range m.Resources {
				text.WriteString("资源版本：" + escapeCompactText(resource.Key) + " @ " + escapeCompactText(resource.Hash) + "\n")
			}
			if m.ResourceBasis != nil {
				text.WriteString("依赖资源版本：" + escapeCompactText(m.ResourceBasis.Key) + " @ " + escapeCompactText(m.ResourceBasis.Hash) + "\n")
			}
		}
		for _, part := range message.Content {
			if part.Type == "text" {
				text.WriteString(escapeCompactText(part.Text) + "\n")
			}
			if part.Type == "image" {
				label := part.ImageRef
				if label == "" || strings.HasPrefix(label, "data:") {
					label = "图片内容块（未复制图片数据）"
				}
				text.WriteString("图片引用：" + escapeCompactText(label) + "\n")
			}
		}
		for _, call := range message.ToolCalls {
			text.WriteString("<tool_call>\n调用 ID：" + escapeCompactText(call.ID) + "\n工具：" + escapeCompactText(call.Name) + "\n参数：\n")
			projectValue(&text, call.Args, "  ")
			text.WriteString("</tool_call>\n")
		}
		text.WriteString("</" + tag + ">\n")
	}
	return text.String()
}

func projectValue(out *strings.Builder, value any, indent string) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out.WriteString(indent + escapeCompactText(key) + "：\n")
			projectValue(out, v[key], indent+"  ")
		}
	case []any:
		for i, item := range v {
			fmt.Fprintf(out, "%s%d.\n", indent, i+1)
			projectValue(out, item, indent+"  ")
		}
	case []string:
		for i, item := range v {
			fmt.Fprintf(out, "%s%d. %s\n", indent, i+1, escapeCompactText(item))
		}
	case string:
		out.WriteString(indent + escapeCompactText(v) + "\n")
	default:
		raw, _ := json.Marshal(v)
		out.WriteString(indent + escapeCompactText(string(raw)) + "\n")
	}
}
