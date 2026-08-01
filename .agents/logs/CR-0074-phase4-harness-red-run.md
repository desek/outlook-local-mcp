# CR-0074 Phase 4 — AC-9 harness red-run evidence

Substantiates AC-9: the `site.yml` "Exercise the site content-check harness"
step (`node .agents/scripts/site.content.check.mjs`) actually fails when the
puppeteer import cannot be resolved, and passes when it can. Recorded because a
gate not proven to fail is not proven to be a gate — the exact thesis this CR
exists to encode.

- Date: 2026-08-01
- Branch: `docs/cr-0073` (historical name; CR renumbered to CR-0074)
- Node: v24.15.0
- Chrome: `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`
- Built site under test: `site/dist` (present from a prior `Build site` run)
- Harness: `.agents/scripts/site.content.check.mjs`
- Raw captures (gitignored `*.log`): `CR-0074-phase4-harness-working.log`,
  `CR-0074-phase4-harness-broken.log`

The break reproduces the original CR-0072 failure mode: a harness import
pointed at a module Node cannot resolve. In `site.content.check.mjs` the line

```js
import { puppeteer } from './site.puppeteer.mjs'
```

was temporarily changed to

```js
import { puppeteer } from './site.puppeteer.nonexistent.mjs'
```

then reverted after capture.

## Working state — exit 0, assertions hold

Command: `node .agents/scripts/site.content.check.mjs`
Exit code: `0`

```
ok index.html: 11853 chars without JavaScript (floor 11853)
ok quickstart.html: 7390 chars without JavaScript (floor 7390)
ok concepts.html: 15582 chars without JavaScript (floor 15582)
ok troubleshooting.html: 17468 chars without JavaScript (floor 17468)
ok SeeDocs: 11/11 anchors resolve
ok crawler files: 6 expected
ok /index.md: 5 Mermaid fences
content-check: all assertions hold
```

## Broken state — non-zero exit, ERR_MODULE_NOT_FOUND

Command: `node .agents/scripts/site.content.check.mjs` (import pointed at
`./site.puppeteer.nonexistent.mjs`)
Exit code: `1`

```
node:internal/modules/esm/resolve:271
    throw new ERR_MODULE_NOT_FOUND(
          ^

Error [ERR_MODULE_NOT_FOUND]: Cannot find module '/Users/desek/Repo/desek/outlook-local-mcp/.agents/scripts/site.puppeteer.nonexistent.mjs' imported from /Users/desek/Repo/desek/outlook-local-mcp/.agents/scripts/site.content.check.mjs
    at finalizeResolution (node:internal/modules/esm/resolve:271:11)
    ...
  code: 'ERR_MODULE_NOT_FOUND',
  url: 'file:///Users/desek/Repo/desek/outlook-local-mcp/.agents/scripts/site.puppeteer.nonexistent.mjs'
}

Node.js v24.15.0
```

## Revert confirmed

- `git diff -- .agents/scripts/site.content.check.mjs` after revert: empty.
- Re-run after revert: exit `0`, `content-check: all assertions hold`.

The step invoked in `site.yml` is the same command exercised here, so a future
import-breaking dependency bump fails this CI step rather than shipping
unnoticed.
