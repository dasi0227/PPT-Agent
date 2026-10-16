# Command Prompt References

Retrieved on 2026-10-16 for the prompts in `backend/prompts/command/`. The directory name follows the requested `docs/thridparty/` path.

Each directory contains the upstream skill directory and repository license unchanged, plus a local `SOURCE.json` recording the repository, pinned commit, upstream paths, and file digests. These are reference snapshots, not installed skills or runtime prompt dependencies. Included scripts and evaluations were not executed. Upstream instructions describe their original environments and do not govern this project.

| Command | Local reference | Upstream source | Ideas used | Adaptation boundaries |
| --- | --- | --- | --- | --- |
| `rename` | [codex-session-naming](codex-session-naming/SKILL.md) | [patrick-fu/awesome-skills](https://github.com/patrick-fu/awesome-skills/tree/57c0c115bc59734e411cb775be81ac0e5d6cf0e5/codex-session-naming) | Keep accurate titles stable; distinguish the continuing purpose from meaningful changes of focus. | No state emojis, tags, controller roles, session-management actions, or direct mutation instructions. |
| `polish` | [copy-editing](copy-editing/SKILL.md), [editing checklist](copy-editing/references/checklist.md) | [coreyhaines31/marketingskills](https://github.com/coreyhaines31/marketingskills/tree/365b412152b9542499ca1439426f7fc3cc92bb65/skills/copy-editing) | Preserve intent and voice; justify each edit; target the requested passage; compare the revised meaning with the original. | No marketing persuasion, invented specificity, added proof, placeholders, expert panels, or changed-lines-only output. Return the complete draft. |
| `compact` | [context-compression](context-compression/SKILL.md), [evaluation framework](context-compression/references/evaluation-framework.md) | [muratcankoylan/Agent-Skills-for-Context-Engineering](https://github.com/muratcankoylan/Agent-Skills-for-Context-Engineering/tree/9ff0c99108b36d3b886defd825d8ac1f47dba374/skills/context-compression) | Merge earlier summaries with new facts; preserve resource references, decision reasons, early constraints, and next actions; optimize for accurate continuation. | No compression thresholds, benchmark targets, runtime memory redesign, or automatic evaluation. Keep the existing five summary sections and their meaning, localized to the conversation language. |
| `handoff` | [handoff](handoff/SKILL.md) | [mattpocock/skills](https://github.com/mattpocock/skills/tree/49dd158d1076134a641b33efb035946536778336/skills/productivity/handoff) | Orient a fresh Agent; tailor the brief to the next session's focus; reference available artifacts without reproducing them; omit secrets. | No temporary-file workflow, new conversation creation, or mandatory suggested-skills section. Preserve this project's user-facing disclosure rules. |

## Source Selection

Discovery used the [skills.sh directory](https://skills.sh/), targeted web searches, and `skills find 'session naming'`. Source content, repository metadata, and MIT licenses were read from GitHub before adaptation. The download retained complete skill directories and checked downloaded files against their upstream Git blob digests.

Popularity is a discovery signal, not evidence that a rule fits this product. The naming reference is narrowly relevant but has limited adoption; only its title-stability principles were retained after comparison with the existing prompt. The compression reference is established at the repository level but less frequently installed as an individual skill.

| Reference | Observed skill installs | GitHub repository stars |
| --- | --- | --- |
| `codex-session-naming` | 25, Skills CLI | 68 |
| `copy-editing` | 142.4K, skills.sh | 54,964 |
| `context-compression` | 416, skills.sh | 18,149 |
| `handoff` | 991.7K, skills.sh | 293,226 |

These counts are dated observations, not current guarantees. Exact commit IDs and content digests are recorded in each `SOURCE.json`; each upstream license remains in its corresponding directory.

## Local Commit Convention

`command/commit.md` follows the user's locally installed `standard-commit` skill: evidence-based wording, one primary intent, conventional change types, a specific title, and concise body entries. Its repository inspection, commit splitting, staging, commit execution, and push procedures do not apply to a model call that only generates a message. No repository operation is authorized by quoting that skill.

## Prompt Organization

All five command prompts use English instructions, matching the adjacent prompt modules' `id` and `description` front matter, level-two headings, and direct rules. Their common structure is:

1. Task Definition
2. Input Description
3. Processing Rules
4. Examples

Input descriptions explain the source material sent to the model. Tool schemas continue to define the submitted result's fields and structural limits. Task-specific output semantics and submission behavior remain in Processing Rules. Runtime retry counts and budgets remain in code.

The instruction language does not force English results: rename, polish, compact, and handoff follow their specified conversation or user language; commit messages have no fixed output language. The compact summary retains five semantic sections, with localized headings. Result tool names, argument shapes, and runtime parsing are unchanged.
