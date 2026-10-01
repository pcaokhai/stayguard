# How to apply this patch

Copy the contents of this folder over the repository root (same paths), then commit:

    git add -A
    git commit -m "docs: switch to FAST MODE (docs/14), slim CI, add design assets"
    git push

Files: CLAUDE.md, api/CLAUDE.md, web/CLAUDE.md, README.md, docs/README.md, docs/progress.md,
docs/14-demo-and-production-plan.md, docs/assets/design/**, deploy/compose.demo.override.yaml,
.github/workflows/ci.yml (checks, test-api-int, contracts and licenses now run on pushes to main only).

Nothing under api/ or web/ source code is changed.
