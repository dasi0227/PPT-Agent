# logo2 形态 B、C、E

设计依据：logo2.png 的身材、宽度与透亮蓝色。统一人物在左、面朝右。

Provider: OpenAI built-in image_gen. Underlying model identifier not exposed in runtime tool metadata.

Constraint delivery: main-prompt constraints

/Users/wyw/Downloads/logo2.png for build, width, identity and palette. Older B and D used solely as pose references.

User-specified: every character on the left, emerging from lower-left, facing right.

Four independent one-pass edits, all preserved as returned without filtering, retries, repairs or post-processing.

| 编号 | 形态 | 用途 | 原生尺寸 | 文件 |
| --- | --- | --- | --- | --- |
| B | 思考交流 | chat / grill / plan 等非 execute 模式 | 1254 × 1254 | [B.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states/B.png) |
| C | 编辑执行 | execute 模式 | 1254 × 1254 | [C.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states/C.png) |
| E | 专注执行 | execute 模式，另一种表情 | 1254 × 1254 | [E.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states/E.png) |

## 提示词、参考图与配色

### B · 思考交流

保留托腮姿态；身材与颜色采用 logo2

参考图（按传入次序）：/Users/wyw/Downloads/logo2.png, /Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo-variants/B.png

颜色：Blue family matched to logo2.png, including its luminous lighter and deeper blue regions / Warm apricot matched to logo2.png。背景：透明。

指定角落：lower-left；面向：right。

```text
Produce ONE complete 1:1 square image, approximately 1536 × 1536 pixels, on a genuinely transparent background.
Reference authority: image 1, logo2.png, is the definitive character reference for identity, slim build, head/body width, proportions, palette and visual treatment. Match the slimness and width of image 1 closely; do not return to the wider, heavier older body. If image 2 is supplied, use it ONLY for the pose or expression named below, never for body width, placement or color.
Subject: retain the same lovable slide sprite from image 1: one continuous soft rounded rectangular body/head, one broad warm apricot face panel, two simple blue eyes and a small mouth, with two short rounded blue arms. Keep the established baby's softness and identity.
Colors: preserve image 1's exact blue and warm apricot semantic color families. Match the original blue's luminous, airy color transitions within that same blue family, with the lighter fresh blue areas and deeper blue areas occurring naturally as they do in image 1. Preserve that reference's color treatment faithfully rather than coloring the whole body a uniform dark blue. Facial marks reuse the blue, and the face panel stays the reference's warm apricot. Introduce no third semantic color.
Composition and direction: keep the person on the LEFT of the square, upright and emerging from the LOWER-LEFT, with bottom cropping. Face toward the viewer's RIGHT in a gentle three-quarter right-facing orientation: turn the head/face panel clearly rightward while retaining both eyes, both arms, the familiar rounded silhouette and recognizability. Leave the greater open transparent area on the RIGHT. The character must not face left, occupy the right side, or sit in the center. Keep the whole artwork upright without rotating the canvas or tilting the main body. Match image 1's vertical scale and dominant presence.
Simplicity: use roughly 4–7 large basic shapes, minimal facial marks, thick rounded contours, blunt ends and clean surfaces. No eyebrow or detailed anatomy. Keep the character readable at 32 × 32. Maintain an ultra-clean graphic treatment. Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Pose and expression: Preserve the thoughtful cheek-resting action in image 2: one short rounded blue arm gently touches the lower cheek, and the other stays low and visible. Adapt this exact action onto image 1's slimmer body. The face turns toward the right, with two attentive simple oval eyes and a tiny calm friendly smile. Convey thoughtful discussion and planning without thought bubbles, questions, or additional objects.
Constraints: No text, watermark, labels, code, lettering, typography, badges, status marks, scenery, extra characters, borders, frames, presentation masks, repeated buttons, fingers, decorative marks, thin fragile lines, sharp tips, glossy hotspots, photorealistic material, deep occlusion, extrusion, strong three-dimensional rendering, external cast shadow, checkerboard, or a painted substitute background.
```

### C · 编辑执行

用拿笔编辑页面的动作明确表达做任务

参考图（按传入次序）：/Users/wyw/Downloads/logo2.png

颜色：Blue family matched to logo2.png, including its luminous lighter and deeper blue regions / Warm apricot matched to logo2.png。背景：透明。

指定角落：lower-left；面向：right。

```text
Produce ONE complete 1:1 square image, approximately 1536 × 1536 pixels, on a genuinely transparent background.
Reference authority: image 1, logo2.png, is the definitive character reference for identity, slim build, head/body width, proportions, palette and visual treatment. Match the slimness and width of image 1 closely; do not return to the wider, heavier older body. If image 2 is supplied, use it ONLY for the pose or expression named below, never for body width, placement or color.
Subject: retain the same lovable slide sprite from image 1: one continuous soft rounded rectangular body/head, one broad warm apricot face panel, two simple blue eyes and a small mouth, with two short rounded blue arms. Keep the established baby's softness and identity.
Colors: preserve image 1's exact blue and warm apricot semantic color families. Match the original blue's luminous, airy color transitions within that same blue family, with the lighter fresh blue areas and deeper blue areas occurring naturally as they do in image 1. Preserve that reference's color treatment faithfully rather than coloring the whole body a uniform dark blue. Facial marks reuse the blue, and the face panel stays the reference's warm apricot. Introduce no third semantic color.
Composition and direction: keep the person on the LEFT of the square, upright and emerging from the LOWER-LEFT, with bottom cropping. Face toward the viewer's RIGHT in a gentle three-quarter right-facing orientation: turn the head/face panel clearly rightward while retaining both eyes, both arms, the familiar rounded silhouette and recognizability. Leave the greater open transparent area on the RIGHT. The character must not face left, occupy the right side, or sit in the center. Keep the whole artwork upright without rotating the canvas or tilting the main body. Match image 1's vertical scale and dominant presence.
Simplicity: use roughly 4–7 large basic shapes, minimal facial marks, thick rounded contours, blunt ends and clean surfaces. No eyebrow or detailed anatomy. Keep the character readable at 32 × 32. Maintain an ultra-clean graphic treatment. Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Pose and expression: The two short thick rounded arms are visibly doing a task: the near arm holds one very short thick stylus with a clearly blunt rounded end and brings its tip into contact with a small broad rounded presentation page held steadily by the other arm, toward the character's right at lower chest height. Keep the page and stylus deliberately minimal, using only the same blue and warm apricot character color families: an apricot page with just one short broad blue rounded stroke, no other marks or page details. This little writing/editing gesture must remain clear as a single compact shape group at small size. Keep both arms visible and the face unobstructed. No hands with fingers. Do not repeat the old pose of two empty arms resting forward. Expression: two open friendly oval eyes directed toward the page on the right, with a small cheerful confident smile. The body/head remains upright; the rightward face turn, arm positions and pen-page contact carry the working gesture.
Constraints: No text, watermark, labels, code, lettering, typography, badges, status marks, scenery, extra characters, borders, frames, presentation masks, repeated buttons, fingers, decorative marks, thin fragile lines, sharp tips, glossy hotspots, photorealistic material, deep occlusion, extrusion, strong three-dimensional rendering, external cast shadow, checkerboard, or a painted substitute background.
```

### E · 专注执行

采用编辑动作，换成更专注的神情

参考图（按传入次序）：/Users/wyw/Downloads/logo2.png

颜色：Blue family matched to logo2.png, including its luminous lighter and deeper blue regions / Warm apricot matched to logo2.png。背景：透明。

指定角落：lower-left；面向：right。

```text
Produce ONE complete 1:1 square image, approximately 1536 × 1536 pixels, on a genuinely transparent background.
Reference authority: image 1, logo2.png, is the definitive character reference for identity, slim build, head/body width, proportions, palette and visual treatment. Match the slimness and width of image 1 closely; do not return to the wider, heavier older body. If image 2 is supplied, use it ONLY for the pose or expression named below, never for body width, placement or color.
Subject: retain the same lovable slide sprite from image 1: one continuous soft rounded rectangular body/head, one broad warm apricot face panel, two simple blue eyes and a small mouth, with two short rounded blue arms. Keep the established baby's softness and identity.
Colors: preserve image 1's exact blue and warm apricot semantic color families. Match the original blue's luminous, airy color transitions within that same blue family, with the lighter fresh blue areas and deeper blue areas occurring naturally as they do in image 1. Preserve that reference's color treatment faithfully rather than coloring the whole body a uniform dark blue. Facial marks reuse the blue, and the face panel stays the reference's warm apricot. Introduce no third semantic color.
Composition and direction: keep the person on the LEFT of the square, upright and emerging from the LOWER-LEFT, with bottom cropping. Face toward the viewer's RIGHT in a gentle three-quarter right-facing orientation: turn the head/face panel clearly rightward while retaining both eyes, both arms, the familiar rounded silhouette and recognizability. Leave the greater open transparent area on the RIGHT. The character must not face left, occupy the right side, or sit in the center. Keep the whole artwork upright without rotating the canvas or tilting the main body. Match image 1's vertical scale and dominant presence.
Simplicity: use roughly 4–7 large basic shapes, minimal facial marks, thick rounded contours, blunt ends and clean surfaces. No eyebrow or detailed anatomy. Keep the character readable at 32 × 32. Maintain an ultra-clean graphic treatment. Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Pose and expression: The two short thick rounded arms are visibly doing a task: the near arm holds one very short thick stylus with a clearly blunt rounded end and brings its tip into contact with a small broad rounded presentation page held steadily by the other arm, toward the character's right at lower chest height. Keep the page and stylus deliberately minimal, using only the same blue and warm apricot character color families: an apricot page with just one short broad blue rounded stroke, no other marks or page details. This little writing/editing gesture must remain clear as a single compact shape group at small size. Keep both arms visible and the face unobstructed. No hands with fingers. Do not repeat the old pose of two empty arms resting forward. Expression: focused and calm, with two slightly narrower rounded oval eyes looking at the page to the right, and a tiny short rounded neutral mouth instead of a cheerful smile. Keep this expression endearing and quietly concentrated rather than angry, sad, strained, sleepy or mechanical. No eyebrows, forehead marks or sweat.
Constraints: No text, watermark, labels, code, lettering, typography, badges, status marks, scenery, extra characters, borders, frames, presentation masks, repeated buttons, fingers, decorative marks, thin fragile lines, sharp tips, glossy hotspots, photorealistic material, deep occlusion, extrusion, strong three-dimensional rendering, external cast shadow, checkerboard, or a painted substitute background.
```

D 已按用户要求撤回并删除项目内 PNG。
