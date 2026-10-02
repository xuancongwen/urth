# Working in this repository

Urth is a MUD server in Go; `README.md` says how to run it and where
things live. `make check` (gofmt, vet, tests, content check) must pass
before work is called finished.

## Trackstar is the project tracker

Everything we do and everything we plan is a story in Trackstar, project
**Urth** (slug `urth`). The repository holds code, content, and the
design record (`docs/RULES.md`, `docs/DECISIONS.md`); it holds no
roadmaps, milestone lists, or to-do lists. If you are about to write
"later", "still owed", or "not built yet" in a file, write a story
instead.

### Before any work

1. Find the story. Search the board (`list_stories` with a `query`)
   before creating one, so the same work is not tracked twice.
2. If there is none, create it: a short title in the board's style ("As
   a player, I can ..." for features; the symptom for bugs; the task
   for chores), and a description that says what and why.
3. Set it to `started`, with Sam as owner, **before** reading code or
   editing anything. Investigation counts as work.

This holds for small jobs too: a one-line fix, a docs change, a
question that turns into a change. Something that changes nothing (a
question answered, a file explained) needs no story.

### While working

Move the story as the work moves, at the time, not in a batch at the
end.

| State | Set it when |
|---|---|
| `started` | work begins |
| `finished` | the work is complete in the working tree and `make check` passes |
| `delivered` | it is committed on `master`; comment with the commit hash |
| `accepted` | Sam says so. Never accept on his behalf unless asked. |
| `rejected` | Sam sends it back; it returns to `started` |

- Comment on the story when something worth knowing later happens: the
  cause of a bug, a decision and its reason, what was left out. The
  story is the record; the chat is not.
- If the work turns out to be two things, split it into two stories.
- If the work stops unfinished, say where it stopped in a comment and
  leave the story `started`.
- At the start of a session, check the stories left `started` or
  `finished` against `git log`: work Sam has committed since moves to
  `delivered`.

### What you notice along the way

Anything found while doing something else, and not fixed as part of it,
becomes its own story rather than a note in a file or a line in the
chat:

- a bug: type `bug`, in the backlog
- an idea or a deferred piece: type `feature` or `chore`, in the icebox
- a design question that is still open: a `chore` in the icebox; its
  reasoning goes in `docs/RULES.md`, the fact that it is undecided goes
  in Trackstar

### Planning

- The icebox is ideas; the backlog is what is planned, in order, top
  first; current is this week. Reorder with `move_story` when
  priorities change. Do not start something from the icebox without
  Sam choosing it.
- "What's next" is the top of the backlog.
- A plan with several steps is several stories, sharing a label. Give a
  story `blocked_by` when it waits on another.

### How Trackstar behaves

- States cannot be skipped: unstarted, started, finished, delivered,
  accepted, one `update_story` call each.
- A feature needs an `estimate` before it can be started. Use 1, 2, 3,
  5, or 8, and say in the reply that the number is yours so Sam can
  change it. Bugs and chores take no estimate.
- An accepted story cannot be moved.
- `labels` on `update_story` replaces the whole list.
- Labels in use: `world`, `combat`, `gear`, `rules`, `engine`,
  `builder`, `admin`, `web`, `deploy`, `crafting`, `deferred`, `fork`,
  `milestone`. Prefer these to new ones.
- Sam is user 1.
