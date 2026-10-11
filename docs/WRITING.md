# Writing the documentation

The site has two parts:

- **The learning chapters** (`learn/*.mdx`) teach Inkshelf to a **curious developer**: someone
  who can program, but knows nothing about EPUB, e-ink, the PocketBook or this code base. They
  are the gentle path, with widgets. Most of this page is about them.
- **The reference** (`reference/*.md`) is the precise, complete source for contributors, in
  plain Markdown (CommonMark, no JSX) so that it also reads well on GitHub. Its relative links
  into the code (`../../internal/...`) become repository links on the site, or plain text while
  `repoUrl` is empty (`src/remark/repoLinks.mjs`). A new reference page goes in
  `sidebars-reference.ts`.

`make docs` serves it with live reload. `make docs-build` checks types and links.

## The voice

- Short sentences. One idea per paragraph. Plain words: "the reader", "the panel", "a page".
- Say why before how. Every mechanism exists because of a constraint (the panel is slow, the
  reader has no SDK, the firmware is shared): name the constraint first.
- Define a word before using it, or use `<Term id="…">` for one from `src/data/glossary.ts`.
  Use only ids that exist there. If a term is missing, add it to the glossary first.
- Real names, real numbers. Every function, file, constant and timing cited comes from the
  code, and you checked it there (not only in the reference docs, which can drift).
- No walls of code. A snippet is at most ~12 lines, trimmed to the idea, and labelled with its
  file.

## Every chapter has the same shape

```mdx
---
title: How e-ink works          # the sidebar label
description: One line for search and link previews.
---

import Ghosting from '@site/src/components/eink/Ghosting';

# How e-ink works

<OneSentence>The whole chapter in one sentence.</OneSentence>

A short opening: what problem this chapter solves, and why you should care.

<Analogy icon="📮">A comparison with everyday life, when one helps.</Analogy>

## A section per idea, from simple to subtle

Prose, then a widget that lets the reader play with the idea:

<Figure title="Partial refreshes leave ghosts" tryThis="Turn five pages, then flash.">
  <Ghosting />
</Figure>

## How Inkshelf does it

The real names, short.

<details>
<summary>Going deeper</summary>

The details an advanced reader wants: edge cases, the numbers, the history.

</details>

## Where it lives in the code

<CodeMap entries={[
  {path: 'internal/device/pocketbook/eink_refresh.go', symbol: 'mxcfbUpdate', what: 'Asks the panel to refresh a rectangle.'},
]} />

The full story: <RefDoc file="eink-and-firmware.md">E-ink and firmware</RefDoc>.

## Check yourself

<Quiz questions={[
  {question: '…?', choices: ['…', '…', '…'], answer: 1, why: '…'},
]} />
```

The kit (`src/components/kit`) is available in every page without an import: `Figure`,
`OneSentence`, `Analogy`, `Pill`, `Controls`, `Button`, `Slider`, `Toggle`, `Segmented`,
`useStepper` and `StepperBar`, `Narration`, `useAnimationFrame`, `usePrefersReducedMotion`,
`PixelGrid`, `CodeRef`, `CodeMap`, `RefDoc`, `Term`, `Quiz`. Widgets import what they need from
`@site/src/components/kit`.

## Widgets

- One folder per chapter: `src/components/<chapter>/<Widget>.tsx`, with a CSS module next to it
  if it needs styles. The algorithms they run go in `src/lib/<name>.ts`, plain TypeScript, with
  a comment naming the Go function they follow.
- A widget teaches **one** idea. It opens in a state that already shows something (never an
  empty box), and has a "Try this" line that says what to do.
- Prefer stepping (`useStepper` + `StepperBar` + `Narration`) for algorithms: the reader controls
  the pace, and each frame has a sentence saying what happens.
- Draw with SVG (crisp, themable) or `<canvas>` for images. Colors come from the `--ink-*` CSS
  tokens of `src/css/custom.css`, so light and dark both work; gray pixels stay real grays.
- No animation or UI library: React state, CSS transitions, `requestAnimationFrame`.
- The page is rendered once on the server at build time: touch `window`, `document` or a canvas
  only inside `useEffect` or event handlers.
- Phone width: a widget fits in 360 px, or scrolls inside its frame (the frame's body does).
- Respect `usePrefersReducedMotion`: continuous animations start paused.

## Checking your work

- `npm run typecheck` and `npm run build` in `docs` (the build fails on a broken link or
  anchor and on a missing glossary term).
- `npm start`, then open the page: use every control, in light and dark mode, and at phone width.
