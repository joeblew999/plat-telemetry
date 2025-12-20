# TODO

~~We can round trip it by just adding a single github workflow that itslef runs task and then uploads releases, and so then a user with just the root task file can run it ALL. THis is because the Release is tagged with the binaries in the release and the taskfiles loaded off the source.   This is exactly what the xplat project is trying to formalise, which includes task and process compose.   https://github.com/joeblew999/xplat. But first i need to finish XPlat.~~ **DONE** - v0.1.0 release with binaries working

---

~~arc needs GO TAG for duckdb variant.~~ **DONE** - DuckDB Go v2 uses pre-built platform-specific bindings (linux-amd64, darwin-arm64). CGO_ENABLED=1 is automatic on GitHub runners. Optional: add `-tags=duckdb_arrow` for Arrow interface support if needed.

---

~~If you are smart with the taskfiles, you can have the github actions run much faster ?~~ **DONE** - Parallel builds + Go module caching added. v0.1.1 built in ~9 min (cache will speed up future runs). Added `bin:build:one SUBSYSTEM=x` for single subsystem builds.

---

~~The github pages can be automated via the gh cli ? assuming basic defaults that we want.~~ **DONE** - Added Hugo docs with GitHub Pages workflow

---

~~https://github.com/joeblew999/plat-telemetry/releases/tag/v0.1.0 only has source so far ...~~ **DONE** - 8 binaries uploaded (linux/darwin × 4 subsystems)

---

~~I suggest you introduce a manifest for each sub system, so you can track versions, so that when it flows through the whole system, you know what version you have.~~ **DONE** - Added `manifest.yaml` with upstream info + `task manifest` to show current versions.

---

~~build: can have bin suffix to match bin:download~~ **DONE** - renamed to `bin:build`

---

~~hugo docs will need DEV and USERS section, because users are doing the taskfile stuff differently of course~~ **DONE** - Added `/users/` and `/dev/` sections

We have same task files to reduce friction between DEV and USERS and OPS.

---

~~Use PICO CSS in our hugo based docs ?~~ **DONE** - Replaced custom CSS with Pico CSS v2


