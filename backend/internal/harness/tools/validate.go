package tools

// Validator 校验文件内容是否合规（M1 为最小实现；M3 接 html-output-spec 的 lint-slide）。
// 返回 ok=false 时 patch/write MUST 拒绝落盘（ARCH-TOOLS-004）。
type Validator interface {
	Validate(rel string, content []byte) (ok bool, reason string)
}

// NopValidator 恒通过；用于无需内容规范的文件（如纯文本 fixture）。
type NopValidator struct{}

func (NopValidator) Validate(string, []byte) (bool, string) { return true, "" }

// NonEmptyValidator 是 M1 演示用的最小校验：内容非空即通过，空则拒绝。
// 用于验证「落盘前 validate」链路（AC-TOOLS-004），不代表真实 html-output-spec。
type NonEmptyValidator struct{}

func (NonEmptyValidator) Validate(_ string, content []byte) (bool, string) {
	if len(content) == 0 {
		return false, "content is empty"
	}
	return true, ""
}
