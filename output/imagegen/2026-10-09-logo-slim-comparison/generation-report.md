# Logo 收窄程度比较

按目标收窄程度从大到小排列。以当前 logo.png 为共同参考，保留配色、表情、姿态与透明背景。

Provider: OpenAI built-in image_gen. Underlying model identifier not exposed by tool metadata.

Constraint delivery: main-prompt constraints

Three independent one-pass edits from the same current reference. Preserve all outputs as returned, without filtering, retries, or post-processing. Reduction percentages are prompt targets, not measured results.

All preserve original lower-left emergence to keep this slimming comparison consistent.

| 编号 | 方向 | 提示词中的收窄目标 | 原生尺寸 | 文件 |
| --- | --- | --- | --- | --- |
| 1 | 明显收窄 | 28% | 1254 × 1254 | [slim-1.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo-slim-comparison/slim-1.png) |
| 2 | 中等收窄 | 18% | 1254 × 1254 | [slim-2.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo-slim-comparison/slim-2.png) |
| 3 | 轻微收窄 | 10% | 1254 × 1254 | [slim-3.png](/Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo-slim-comparison/slim-3.png) |

## 配色与完整提示词

### 1 · 明显收窄

角色：cobalt blue #2F67F6 / warm apricot #FFD19A。背景：透明。角落：lower-left。

```text
Edit the supplied image into ONE complete full-resolution 1:1 square PNG with a genuinely transparent background, approximately 1536 × 1536 pixels.
Requested change: make this same cute slide sprite slimmer by approximately 28% relative to the supplied reference. The main continuous body/head should be about 72% of its current horizontal width, with its original vertical height maintained. This is the 明显收窄 variant. This reduction must be visible, rather than shrinking the entire character uniformly on the canvas.
Keep the original friendly identity, the soft rounded rectangle silhouette, its cobalt blue body and both arms, one continuous warm apricot face panel, two blue simple oval eyes and blue curved smile. Keep the face panel proportionate to the slimmer body. Bring the arm attachments and eye spacing inward appropriately, while preserving the softness and thickness of both rounded arms and the natural shape of the eyes and mouth. Maintain the same waving pose with both arms visible. Keep the same generous head, heavy rounded contours, calm baby-like personality, restrained ultra-clean graphic treatment, and 4–7 large basic shapes. Use exactly two semantic character color families, reusing the blue for all facial marks.
Composition: retain the original upright character orientation, lower-left emergence and bottom crop. Maintain the same vertical placement and scale as the reference so the width change is the meaningful difference. Retain a square canvas; do not stretch or rotate the canvas. Any freed space must stay truly transparent. Do not make the character angular, skeletal, stretched vertically, or a different character.
Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Constraints: No text, watermark, badges, borders, frames, props, additional subjects, scenery, extra decorations, stray colored specks, unnecessary outlines, thin fragile lines, sharp tips, glossy hotspots, photorealistic materials, strong three-dimensional rendering, external cast shadows, checkerboard, solid backdrop, or simulated transparency.
```

### 2 · 中等收窄

角色：cobalt blue #2F67F6 / warm apricot #FFD19A。背景：透明。角落：lower-left。

```text
Edit the supplied image into ONE complete full-resolution 1:1 square PNG with a genuinely transparent background, approximately 1536 × 1536 pixels.
Requested change: make this same cute slide sprite slimmer by approximately 18% relative to the supplied reference. The main continuous body/head should be about 82% of its current horizontal width, with its original vertical height maintained. This is the 中等收窄 variant. This reduction must be visible, rather than shrinking the entire character uniformly on the canvas.
Keep the original friendly identity, the soft rounded rectangle silhouette, its cobalt blue body and both arms, one continuous warm apricot face panel, two blue simple oval eyes and blue curved smile. Keep the face panel proportionate to the slimmer body. Bring the arm attachments and eye spacing inward appropriately, while preserving the softness and thickness of both rounded arms and the natural shape of the eyes and mouth. Maintain the same waving pose with both arms visible. Keep the same generous head, heavy rounded contours, calm baby-like personality, restrained ultra-clean graphic treatment, and 4–7 large basic shapes. Use exactly two semantic character color families, reusing the blue for all facial marks.
Composition: retain the original upright character orientation, lower-left emergence and bottom crop. Maintain the same vertical placement and scale as the reference so the width change is the meaningful difference. Retain a square canvas; do not stretch or rotate the canvas. Any freed space must stay truly transparent. Do not make the character angular, skeletal, stretched vertically, or a different character.
Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Constraints: No text, watermark, badges, borders, frames, props, additional subjects, scenery, extra decorations, stray colored specks, unnecessary outlines, thin fragile lines, sharp tips, glossy hotspots, photorealistic materials, strong three-dimensional rendering, external cast shadows, checkerboard, solid backdrop, or simulated transparency.
```

### 3 · 轻微收窄

角色：cobalt blue #2F67F6 / warm apricot #FFD19A。背景：透明。角落：lower-left。

```text
Edit the supplied image into ONE complete full-resolution 1:1 square PNG with a genuinely transparent background, approximately 1536 × 1536 pixels.
Requested change: make this same cute slide sprite slimmer by approximately 10% relative to the supplied reference. The main continuous body/head should be about 90% of its current horizontal width, with its original vertical height maintained. This is the 轻微收窄 variant. This reduction must be visible, rather than shrinking the entire character uniformly on the canvas.
Keep the original friendly identity, the soft rounded rectangle silhouette, its cobalt blue body and both arms, one continuous warm apricot face panel, two blue simple oval eyes and blue curved smile. Keep the face panel proportionate to the slimmer body. Bring the arm attachments and eye spacing inward appropriately, while preserving the softness and thickness of both rounded arms and the natural shape of the eyes and mouth. Maintain the same waving pose with both arms visible. Keep the same generous head, heavy rounded contours, calm baby-like personality, restrained ultra-clean graphic treatment, and 4–7 large basic shapes. Use exactly two semantic character color families, reusing the blue for all facial marks.
Composition: retain the original upright character orientation, lower-left emergence and bottom crop. Maintain the same vertical placement and scale as the reference so the width change is the meaningful difference. Retain a square canvas; do not stretch or rotate the canvas. Any freed space must stay truly transparent. Do not make the character angular, skeletal, stretched vertically, or a different character.
Add an extremely, extremely subtle, almost imperceptible sense of depth through a barely-there neo-skeuomorphic treatment.
Constraints: No text, watermark, badges, borders, frames, props, additional subjects, scenery, extra decorations, stray colored specks, unnecessary outlines, thin fragile lines, sharp tips, glossy hotspots, photorealistic materials, strong three-dimensional rendering, external cast shadows, checkerboard, solid backdrop, or simulated transparency.
```
