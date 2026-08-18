# Publishing the plugin

Everything needed to list `contra.esports` publicly. Nothing here has been
submitted — this is the prepared path.

## Where plugins actually get listed

There is **no official Basecamp-run plugin directory**. Omarchy's own manual
delegates discovery to a community site:

> "To help people actually find it, list it at omarchyplugins.com… it's the
> community directory of Omarchy shell plugins."
> — <https://omarchy.org/manual/shell-plugins/>

That site is run by a separate org and states plainly that it is "not
affiliated with, sponsored by, or endorsed by Omarchy or 37signals". It is the
practical target because the official docs point at it, not because it is
official.

- Catalog: <https://omarchyplugins.com>
- Backing repo: <https://github.com/HANCORE-linux/omarchy-plugin-marketplace>
- Entries live in `registry.json`, written by a bot after a maintainer approves

## The repo-layout constraint

`omarchy plugin add <git-url>` clones a repo and validates
`"$clone/manifest.json"`. It only ever reads the manifest at the **repository
root** — there is no subdirectory option anywhere in the script. It then moves
the entire clone to `~/.config/omarchy/plugins/<id>/`.

This repo keeps the plugin under `plugin/contra.esports/` next to the daemon
and the app, so it cannot be installed that way as-is. `./package-plugin.sh`
builds the publishable tree with the manifest promoted to the root:

```bash
./package-plugin.sh
# -> dist/plugin-repo/  (manifest.json, *.qml, Model.js, README.md, LICENSE)
```

Push that directory as its own public repo and submit *that* URL.

## The daemon dependency

The plugin is QML only. Installed from the catalog it will render, read no
state file, and show its "waiting for the daemon" panel, which names the
project so a user can find the daemon. This is worth being explicit about in
the listing description rather than letting people discover it: the widget is
a front end, and the Go daemon does the fetching.

Two honest options when listing:

1. **List it as-is** and say in the description that it requires the
   `omarchy-esports` daemon, with the install command in the README. Simplest,
   and the empty state already explains itself.
2. **Ship a prebuilt daemon binary** as a GitHub release asset and have the
   README's install step fetch it. Note the marketplace's automated security
   baseline flags `curl | sh` patterns, so give people a download-then-verify
   sequence rather than a pipe into a shell.

Option 1 is the default assumption of everything below.

## Pre-submission checklist

- [ ] `manifest.json` at the packaged repo root, `id` not starting with `omarchy.`
- [ ] `README.md` at root with both **install and removal** instructions (required)
- [ ] `LICENSE` at root
- [ ] `preview.png` at root — put the source at `docs/preview.png` and the
      packaging script copies it. Do not hand-crop; the site generates its own
      card and detail crops
- [ ] `omarchy plugin validate ./dist/plugin-repo` exits 0
- [ ] No symlinks anywhere in the tree (hard validator failure)
- [ ] Decide the final plugin id — **ids are permanent and globally unique
      across the whole catalog, and retired ids cannot be reused**. See below
- [ ] Search omarchyplugins.com to confirm the id is free
- [ ] No `curl | sh`, no unpinned remote code execution, no `NOPASSWD` sudoers
      rules, no PID files in shared `/tmp` — these trip the security scanner

## The plugin id decision

Currently `contra.esports`, which matches the other plugins on this machine
(`contra.gpu`, `contra.media`) and passes the official validator, whose only
real rules are `^[A-Za-z0-9][A-Za-z0-9._-]*$` and no `omarchy.` prefix.

The marketplace *recommends* a namespaced form like
`io.github.matt-shearing.esports`. Since ids are permanent, the trade-off is
worth settling before submission:

| | `contra.esports` | `io.github.matt-shearing.esports` |
|---|---|---|
| Matches local plugins | yes | no |
| Collision risk in a global namespace | higher | effectively nil |
| Marketplace convention | tolerated | recommended |

Changing it later means a new listing, not a rename.

## Submission

Category (exactly one): **Widgets**
Tags (1–3, from the fixed list): **Bar**, **Quickshell**, **Media**

```bash
gh issue create --repo HANCORE-linux/omarchy-plugin-marketplace \
  --title "[Plugin]: Esports"
```

The form asks for the repository URL, the category, the tags, and five
confirmations (public repo with install and removal docs; license and
dependencies documented; you own or are authorised to submit it; the plugin
does not overwrite user config without consent; you understand listing approval
is not a security audit).

After submission a workflow runs a validator and an automated security baseline
and comments the results; a human maintainer then approves, at which point a
bot writes the entry into `registry.json`. The record pins an exact commit SHA,
so **later updates require re-validation** rather than being picked up from the
branch automatically.
