# Project reference rules

Study community ASS tools before you add lint or format behavior. Useful local references include:

- `ref/community/line0-Aegisub-Scripts/l0.ASSWipe.moon` for ASSWipe
- `ref/community/unanimated-Aegisub-Scripts/ua.HYDRA.lua` for HYDRA
- `ref/community/ASSFoundation`
- `ref/community/SubInspector`
- `ref/community/Aegisub/automation/include/cleantags.lua` for Aegisub Clean Tags

Clone these projects under `ref/community/` when a task needs them. `ref/` is gitignored and local only.

Use these projects to find real lint, cleanup, and format needs. Do not copy their rules without review. Design the assx rule from ASS semantics and assx goals.
