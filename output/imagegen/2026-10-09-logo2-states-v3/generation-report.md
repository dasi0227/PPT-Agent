# B、C、D 第三版生成记录

- Provider: OpenAI built-in image_gen
- Model: Runtime model identifier not exposed
- Constraints: main-prompt constraints
- 以 logo2.png 为主体宽高、位置、裁切和配色参考；尺寸对齐为生成目标，不是像素锁定编辑。
- D 对应上一版 E 画刷形态，改为 B 的笑脸。
- 原始输出直接保存，未裁切、缩放或重绘。

| 版本 | 形态 | 尺寸 | 文件 |
| --- | --- | --- | --- |
| B | 思考形态 | 1254 × 1254 | B.png |
| C | 书写形态 | 1254 × 1254 | C.png |
| D | 画刷形态 | 1254 × 1254 | D.png |

## B

References:
- /Users/wyw/Downloads/logo2.png
- /Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states-v2/B.png

Prompt:

```text
Edit the provided character references into one independent transparent square PNG. Image 1 is the strict authority for the main character's scale, head width and height, body width, placement, luminous blue shading, apricot face color and near-frontal orientation. Image 2 supplies the alternate arm pose and props only. The previous alternate characters are too small compared to image 1: fix this by using image 1's main head and torso as an unchanged fixed base, attaching the alternate arms and props to it. Do not scale down the base to fit the props. Match image 1's exact visible head width and height, head top at about 21.5 percent of the canvas, face-panel dimensions and position, torso silhouette and lower-edge crop. Maintain image 1's rounded tilted head and near-frontal face, just its subtle existing rightward bias. Do not rotate into a right-facing profile. Character stays on the left; any paper and tool interaction goes to the right. Preserve the slim body proportions rather than widening the torso. Keep the airy translucent-looking luminous blue gradients with bright blue highlights and deeper blue shadows; apricot face, oval blue eyes, minimal rounded forms. The character itself is opaque with clean edges. No new feet, tabletop, background, ground shadow, backdrop, border, lettering, labels, decorative objects or additional character. Background must have real alpha transparency. Output a single 1254 by 1254 square composition retaining the reference's camera scale and body crop.

Keep image 2's exact cheek-resting thoughtful arm pose and friendly curved smile. Preserve both oval eyes and the curved smiling mouth. No props.
```

## C

References:
- /Users/wyw/Downloads/logo2.png
- /Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states-v2/C.png

Prompt:

```text
Edit the provided character references into one independent transparent square PNG. Image 1 is the strict authority for the main character's scale, head width and height, body width, placement, luminous blue shading, apricot face color and near-frontal orientation. Image 2 supplies the alternate arm pose and props only. The previous alternate characters are too small compared to image 1: fix this by using image 1's main head and torso as an unchanged fixed base, attaching the alternate arms and props to it. Do not scale down the base to fit the props. Match image 1's exact visible head width and height, head top at about 21.5 percent of the canvas, face-panel dimensions and position, torso silhouette and lower-edge crop. Maintain image 1's rounded tilted head and near-frontal face, just its subtle existing rightward bias. Do not rotate into a right-facing profile. Character stays on the left; any paper and tool interaction goes to the right. Preserve the slim body proportions rather than widening the torso. Keep the airy translucent-looking luminous blue gradients with bright blue highlights and deeper blue shadows; apricot face, oval blue eyes, minimal rounded forms. The character itself is opaque with clean edges. No new feet, tabletop, background, ground shadow, backdrop, border, lettering, labels, decorative objects or additional character. Background must have real alpha transparency. Output a single 1254 by 1254 square composition retaining the reference's camera scale and body crop.

Keep image 2's exact writing arm pose, short thick blue stylus and apricot paper on the right. Paper must be completely blank, without blue marks, lines or symbols. Preserve the existing oval eyes and curved smiling mouth.
```

## D

References:
- /Users/wyw/Downloads/logo2.png
- /Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states-v2/E.png
- /Users/wyw/Desktop/Projects/PPT-Agent/output/imagegen/2026-10-09-logo2-states-v2/B.png

Prompt:

```text
Edit the provided character references into one independent transparent square PNG. Image 1 is the strict authority for the main character's scale, head width and height, body width, placement, luminous blue shading, apricot face color and near-frontal orientation. Image 2 supplies the alternate arm pose and props only. The previous alternate characters are too small compared to image 1: fix this by using image 1's main head and torso as an unchanged fixed base, attaching the alternate arms and props to it. Do not scale down the base to fit the props. Match image 1's exact visible head width and height, head top at about 21.5 percent of the canvas, face-panel dimensions and position, torso silhouette and lower-edge crop. Maintain image 1's rounded tilted head and near-frontal face, just its subtle existing rightward bias. Do not rotate into a right-facing profile. Character stays on the left; any paper and tool interaction goes to the right. Preserve the slim body proportions rather than widening the torso. Keep the airy translucent-looking luminous blue gradients with bright blue highlights and deeper blue shadows; apricot face, oval blue eyes, minimal rounded forms. The character itself is opaque with clean edges. No new feet, tabletop, background, ground shadow, backdrop, border, lettering, labels, decorative objects or additional character. Background must have real alpha transparency. Output a single 1254 by 1254 square composition retaining the reference's camera scale and body crop.

Keep image 2's exact working arm pose, blue-handled paintbrush with a broad rounded apricot brush head, and blank apricot paper on the right. Paper must be completely blank with no marks, lines or symbols. CHANGE ONLY the expression to the same friendly curved open smile as image 3 B, with the same two oval eyes; no neutral flat mouth and no closed eyes.
```
