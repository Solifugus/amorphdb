# Archived documentation

Nothing here is authoritative. These files are kept because they record how
decisions were reached, not what is true now. **Do not read them for current
behaviour** — start from `CLAUDE.md` at the repository root.

Archived 2026-07-30, during a documentation review that found several of these
still being cited as if current.

## Superseded design snapshots

Dated copies of the master spec, kept for provenance. The live spec is
`docs/amorphdb_design.md`.

| File | Snapshot of |
|---|---|
| `amorphdb_design_until20260402.md` | the spec as of 2 April 2026 |
| `amorphdb_design_until20260426.md` | the spec as of 26 April 2026 |
| `amorphdb_design_preCurrencyFix.md` | before the Money/currency revision |
| `amorphdb_design_prePWABiolerplate.md` | before the PWA boilerplate was specified |

## Completed development plans

Each of these drove a body of work that has since landed. They are historical
records of intent, and their status markers were never updated on completion.

- `spec_compliance_plan.md`
- `syntax_changes_development_plan.md`
- `web_layer_development_plan.md`
- `pwa_auth_development_plan.md` — the PWA bridge and boilerplate it planned are
  built; it was still listed in CLAUDE.md's authoritative-documents table.

## Superseded status reports

- `STATUS.md` — a snapshot from 5 April 2026 headed "P0 CORRECTNESS ISSUES
  FIXED". CLAUDE.md's standing rules used to direct sessions to record open
  questions here, which meant writing into a file nobody read. That rule now
  points at `DEVPLAN.md`.
- `amorphdb_spec_vs_implementation_report.md` — 10 April 2026. Superseded by
  `docs/AmorphDB_Remaining_Work.md`, which ships in release archives and is
  re-verified against the code.
- `AmorphDB_Review_Items.md` — 5 April 2026 review notes.

## Superseded guides and drafts

- `DISTRIBUTION.md` — predates the release pipeline. Distribution is now
  `.goreleaser.yaml` plus `.github/workflows/release.yml`: push a `vX.Y.Z` tag
  and the archives, checksums and signature are produced automatically.
- `pwa_biolerplate_design.md` — early draft (the filename typo is original).
  Superseded by `docs/AmorphDB_PWA_Boilerplate_Spec.md`.
- `development_history.md` — narrative history to April 2026.
- `book-design.md` — notes toward a book; never developed.
