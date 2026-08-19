# TODO

## Swap the outbound contact email

`contactEmail` in `~/.config/omarchy-esports/config.json` is currently
`matt@oneqode.com`. That address is sent to Liquipedia on **every request**, in
the User-Agent header:

```
omarchy-esports/0.1.0 (https://github.com/…; matt@oneqode.com) go-http
```

Liquipedia's API terms require a contact address, so the field should not be
emptied — swap it for whatever you would rather publish. A role address or a
GitHub profile URL both work:

```bash
omarchy-esports config edit      # set "contactEmail"
systemctl --user restart omarchy-esports
```

If it is left blank the client falls back to the project's issues URL, which is
compliant but gives Liquipedia no way to reach you specifically.

Note this address will also be visible in the published plugin repo if it ends
up in a committed config example — the repo currently only references it in
this file and in `docs/`.

## Publish to the plugin catalog

Everything is prepared and nothing is submitted. `docs/SUBMISSION.md` has the
exact commands, the field values, and the five form confirmations mapped to
where each is already satisfied. Two decisions gate it: the plugin id (below)
and the contact address (above).

## Decide the published plugin id

The plugin ships as `contra.esports`, matching your other local plugins. The
community marketplace recommends a namespaced id such as
`io.github.matt-shearing.esports`, and **plugin ids are permanent and globally
unique once listed** — they cannot be reused after retirement. Worth settling
before submitting, not after. See `docs/PUBLISHING.md`.

## Resolved

- The stray `gamerlegion` follow-list entry is gone. The org is now followed
  as `{"name": "GamerLegion", "wiki": "dota2"}` — their Dota roster only,
  which is what you were trying to express.
