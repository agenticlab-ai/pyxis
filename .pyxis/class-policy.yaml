# class-policy/v1 — the authored ownership-class input for the release build
# (installer-v1 §3.2; AP-24). The release workflow walks the packed payload
# tree against these rules (internal/relmanifest) and derives the release
# manifest; a payload file no glob matches FAILS the build naming the file
# (DINV-6, A-11). First match wins — specific rows precede catch-alls.
#
# The walk sees the payload tree AS PACKED, and the packer stages tracked
# content only: gitignored caches (.pyxis/bin/, .pyxis/feedback-queue/,
# .pyxis/contract-gate-reports/, vendor node_modules/, .pyxis/scratch/ scratch)
# never enter the payload, so they carry no rows here. The governance
# skeletons and the operand AGENTS.md are instantiated by init, not
# shipped-in-place: repo-owned from birth and never listed here
# (installer-v1 §3.5 step 4).
schema_version: class-policy/v1
classes:
  # The operand's own knobs + paths + pins + epoch — converge validates it
  # against the closed schema, never rewrites (DINV-3). init writes it before
  # converge runs, so the repo-owned file always exists at converge time.
  - { glob: ".pyxis/config.yaml", class: repo-owned }
  # Vendored browser assets and pricing data ship byte-exact.
  - { glob: ".pyxis/vendor/**", class: engine-verbatim }
  # Everything else under the system tree — templates, skills, scripts,
  # authoring, hooks, ui, writehook, the argus-stamp wiring surface
  # (.pyxis/stamp/**, engine documentation read by nothing at runtime), the
  # maps and policy files — ships byte-exact; converge repairs drift byte-exact
  # and a fresh install materializes it. A repo-owned class here would
  # dead-end converge: the human authors none of it and no init stage seeds it.
  - { glob: ".pyxis/**", class: engine-verbatim }
  # The INSTALLED-repo gate workflow variant (installer-v1 §3.10):
  # exec-grammar gate steps, zero source-toolchain steps. GENERATED, not
  # engine-verbatim (config-driven-portability-v1 §3.1, FR-4): a CI-host push
  # trigger is a static literal the host evaluates with no config read, so the
  # branch/remote must be baked correct-for-the-repo. converge renders it from
  # the shipped template (.pyxis/dist/github-workflows/docs-gate.yml),
  # substituting branch+remote from the repo config, and re-renders on config
  # change; doctor names a hand-edit as drift and converge repairs it (DINV-4).
  # The manifest row carries sha null (generated); the overlay stages the
  # DEFAULT render at pack time (config.Defaults() → origin/main, byte-identical
  # to the former static file — DINV-2). No phase-1 operand carries an old-class
  # workflow to migrate (the pyxis repo is source-mode; adoption is phase-2).
  #
  # ci_mode-conditional at CONSUMPTION, not here: this row (and the one
  # release manifest it derives) stays UNCONDITIONAL — one release serves
  # every install mode. converge/doctor gate it on the resolved knobs.ci_mode
  # knob when they turn the manifest into an operand's desired set: included
  # (actions/both) writes and repairs it; excluded (local, or uncaptured)
  # drops it from the set entirely — UNLISTED, never deleted, so a knob
  # flipped to excluding after install leaves a prior file in place until a
  # human acts (doctor's advisory names both remedies)
  # (prd:init-hardening/FR-11, design:init-hardening-v1#3.8).
  - { glob: ".github/workflows/docs-gate.yml", class: generated }
  # The design index is a docgate output — converge regenerates, never hashes.
  - { glob: "docs/tech-design/index.*", class: generated }
