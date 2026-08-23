# Submission draft

Everything needed to list the plugin, ready to fire. Nothing here has been
submitted — two decisions are yours first.

## Decide first

**1. The plugin id.** Settled: `contra.esports`. That matches `contra.keep-on`
and `contra.layouts`, already on the catalog. Ids are permanent.

**2. The contact address.** `contactEmail` in the config is sent to Liquipedia
in the User-Agent on every request. See `TODO.md`. Not in the plugin repo.

## Publish the plugin repo

`omarchy plugin add` clones a repo and only ever reads `manifest.json` at the
**repository root**, so the plugin cannot be installed from its subdirectory
here. `package-plugin.sh` builds the standalone tree:

```bash
./package-plugin.sh                      # -> dist/plugin-repo/
cd dist/plugin-repo
git init -b main && git add -A
git commit -m "contra.esports 0.1.0"
gh repo create omarchy-esports-plugin --public --source=. --push
```

Then verify it installs the way a user would:

```bash
omarchy plugin add https://github.com/matt-shearing/omarchy-esports-plugin.git --enable
```

## Submit to the community catalog

There is no official Basecamp-run directory; Omarchy's own manual points at
omarchyplugins.com, which is community-run. Submission is a GitHub issue form,
then an automated validator and security-baseline scan, then human approval.

```bash
gh issue create --repo HANCORE-linux/omarchy-plugin-marketplace \
  --title "[Plugin]: Esports"
```

Field values to use:

- **Repository URL**: `https://github.com/matt-shearing/omarchy-esports-plugin`
  (root, no trailing slash, no `/tree/...`)
- **Category** (exactly one): `Widgets`
- **Tags** (1–3 from their fixed list): `Bar`, `Quickshell`, `Media`

Suggested description:

> A spoiler-free esports schedule for the Omarchy bar. Shows the next match for
> the teams you follow with a live countdown; click any fixture for stream
> links, the VOD, and Liquipedia pages. Results are withheld by a background
> daemon rather than merely undrawn, so a score you have not asked for is
> absent from the file the widget reads. Covers 37 Liquipedia games. Requires
> the `omarchy-esports` daemon, linked from the README.

The five confirmations the form asks for, and where each is satisfied:

| Confirmation | Where |
|---|---|
| Public repo with install **and removal** docs | `docs/plugin-README.md` has both |
| Licence and dependencies documented | `LICENSE` (MIT + third-party notice), README names the daemon dependency |
| You own or are authorised to submit it | yours |
| Does not overwrite user config without consent | the plugin only reads state; the daemon writes only its own config and state |
| Listing approval is not a security audit | acknowledged |

## Security-baseline trip-wires

Their scanner flags these. None are present, but worth knowing before editing:

- no `curl \| sh` anywhere
- no unpinned remote code execution
- no `NOPASSWD` sudoers rules
- no PID files in shared `/tmp`

The packaged plugin is QML and a JSON manifest only — no scripts at all.
Daemon uninstall lives in the companion repo README, not here, so the listing
does not mention `systemctl`.

## After approval

The registry entry pins an exact commit SHA rather than a branch, so later
updates need re-validation rather than being picked up automatically.
