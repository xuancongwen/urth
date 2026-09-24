# Rules

A living design document for the game rules: combat, stats, items, magic,
progression. Nothing here is implemented yet; rules land in scripts from
milestone 8 onward. Each section records a decision, the reasoning, and the
questions still open. Change a decision by editing it and noting the date.

Rules are formulas plugged into fixed seams. Section 1 is the contract the
engine offers; everything after it must be expressible through that contract.
If a rule cannot be, widen the contract in milestone 6, do not put the rule
in Go.

Status of each decision: **decided**, **leaning**, or **open**.

### Method

Open questions are worked dialectically. Each gets a **thesis** (the
obvious or traditional answer), an **antithesis** (the strongest case
against it), and a **synthesis** that keeps what survives from both. The
synthesis becomes the section's *leaning* position until it is argued
down or confirmed as *decided*. Keep the thesis and antithesis in the
document after deciding; they are the reasoning a future change has to
beat.

---

## 1. Constraints from the engine

What a rule script will and will not be able to touch once milestone 6
lands. This section is owned by the engine, not the designer.

### Hook points (implemented in milestone 6)

Each hook is a global function in `data/scripts/*.js`. Inputs are
read-only snapshots; outputs are plain objects the engine applies. The
placeholder implementations live in `data/scripts/rules.js`.

| Hook | Called when | Inputs | Returns |
|---|---|---|---|
| `resolveAttack` | each swing in a combat round, and once on `kill` | attacker, defender, weapon (null if unarmed), round | `{hit, damage, crit, verb}` |
| `derivedStats` | login, spawn, equipment change, level | character | `{healthMax, manaMax, attacksPerRound}` |
| `onTick` | once per round for every character | character | `{healthDelta, manaDelta}` |
| `xpForKill` | a player kills a mob | killer, victim | integer |
| `xpToLevel` | after any experience gain | level | integer (total xp needed) |
| `onLevel` | a character gains a level | character, new level | `{statDeltas:{}, message}` |
| `resolveCast` | not yet; arrives with magic in milestone 8 | | |

### What scripts see

- Character: `{name, level, xp, stats:{}, health, healthMax, mana, manaMax,
  equipment:{slot: item}, isPlayer, fighting, room:{vnum,name,area}, vnum
  and flags for mobs}`. `stats` is whatever the rules put there; the engine
  does not define the stat set.
- Item: `{vnum, name, type, slot, weight, value, flags, weapon:{damage,
  hands, kind}, armor:{defense}, mods:{}}`.
- `random.int(n)`, `random.float()`, `random.roll(count, sides)`: seeded
  per simulation so runs are reproducible. `log(...)` writes to the server
  log.

### What scripts cannot do

- Move characters, create or destroy items, send arbitrary text. Scripts
  return values; the engine applies them and renders messages.
- Block. A hook that runs longer than 50 ms is interrupted; the engine
  uses a safe default (a miss, no regen, 1 max health) and warns admins
  once per load.
- Keep state between calls. Effects on characters arrive with magic.

### Time

- A tick is `timing.tick_ms` (100 ms). Commands are processed once per tick.
- A round is `timing.round_seconds` (3 s). Combat, regeneration, and effect
  durations are counted in rounds.

### Balance tooling (implemented)

- `simulate <mob|me> <mob> [fights] [seed]` runs detached fights through
  the same hooks and reports win rates, rounds, and damage per fight. Mobs
  are equipped as their reset entry spawns them; `me` uses your sheet and
  gear. A seed makes the run reproducible.
- Scripts reload when a file changes, checked once per round. A syntax
  error keeps the previous rules and warns admins once. `reload` forces it.
- `bin/urthbot` drives a running server from the command line for
  scripted checks.

---

## 2. Design goals

Everything in sections 3 to 8 should trace back to a line here. Each goal
below was worked dialectically on 2026-09-23; the syntheses are
**leaning** until confirmed.

### 2.1 What the player is thinking about during a fight

**Thesis.** Combat is automatic, as in ROM: type `kill`, watch the rounds
scroll, intervene only to `flee`, `quaff`, or `cast`. This is what the
command vocabulary (D7) implies, it works over a bare telnet line, and it
lets a fight run while the player talks.

**Antithesis.** If nothing happens between `kill` and the corpse, the
fight was decided before it started by stats and gear. Every fight is a
spreadsheet lookup the player already knows the answer to. There is
nothing to learn and nothing to get better at, so the only progression is
numbers going up.

**Synthesis (leaning).** Auto-attack is the floor, not the ceiling. An
even fight at level is winnable with no input at all, so a player who is
chatting or lagging is not punished. Above that floor, one action per
round is available (an ability, `flee`, an item), and abilities are
paced by cooldowns counted in rounds so that a fight of typical length
offers two or three real decisions, not a decision every round. Fights
above the player's level are where those decisions matter; fights at
level are where they are optional.

Consequences: 4.4 must allow one player action per round on top of the
auto-attack. Abilities need a cooldown field. The simulator must be able
to run a fight with no ability use, since that is the floor being
balanced.

### 2.2 Gear versus level versus stats

**Thesis.** Gear dominates. Items are visible, comparable at a glance
(4.1 fixed damage exists for this), tradeable, and give a reason to
explore. A better sword is the most satisfying thing a MUD can hand out.

**Antithesis.** If gear dominates, a level 5 character in level 20 gear is
a level 20 character. Level stops meaning anything, twinking becomes the
meta, and every mob has to be balanced against the best gear a player
could possibly have been handed. Stats (3.1) become noise on top of the
sword.

**Synthesis (leaning).** Three axes with distinct jobs. Level *gates*:
items carry a level requirement, and experience from mobs far below the
player's level falls to nothing. Gear *sets the base numbers*: the weapon
is the damage, the armor is the defense, and the item level table (5.2)
is the progression curve made visible. Stats *multiply* and are the
build: two characters of the same level in the same gear differ by their
stat allocation, and that difference is felt but never larger than the
gear difference.

Working ratios to test in the simulator: at a fixed level, best
available gear versus starting gear is about 2x; best stat allocation
versus worst is about 1.5x; five levels of item table is about 2x. The
ordering matters more than the numbers: level > gear > stats over a
career, gear > stats > level within a session.

Consequences: 5.1 adds a `level` field to items and the engine enforces
it on `wear` and `wield` (a small contract widening for milestone 6).
3.3's multiplier ceiling is bounded by the 1.5x figure. 7.1's `xpForKill`
must decay with level gap.

### 2.3 Lethality

**Thesis.** Death should be rare and expensive. If dying costs little,
fights carry no tension and `flee` is never typed.

**Antithesis.** This is a small server. Every character lost to a harsh
death is a player who may not come back, and every balance mistake that
slips past the simulator becomes a player-losing bug instead of an
annoyance. ROM's routine death (lose some experience, corpse in the
room, walk back) has kept players for thirty years precisely because it
is survivable.

**Synthesis (leaning).** Death is routine in frequency and bounded in
cost. It never destroys the character, never de-levels, and never
permanently destroys gear. What it costs is time and a slice of
experience: respawn in town at low health, lose a capped fraction of the
experience toward the next level, and leave a corpse holding the
inventory that persists long enough to walk back for it.

The tension comes from *attrition*, not from variance. An even fight at
full health should almost never kill; a chain of fights without resting
should. A careful player at level dies about once every several outings,
and a reckless one dies every outing. `flee` is typed because the player
sees health dropping across a chain, not because one swing went badly.

Consequences: 4.2 must choose low variance (one fight cannot swing from
comfortable to fatal). 4.6 is mostly written by this section. 4.5's
recovery time is what makes resting a real choice.

### 2.4 Solo or group

**Thesis.** Group-first. MUDs are social; grouping is the reason to run a
multiplayer text game rather than a roguelike.

**Antithesis.** A new server has a handful of players in a handful of
time zones. Content that requires a group is content that is unplayable
most hours of the day, and a player who logs in alone and finds nothing
to do logs out for good.

**Synthesis (leaning).** Solo-complete, group-accelerated. Every fight in
the world is soloable by a character of the intended level with the
intended gear. Groups get speed, safety, and the ability to take fights
above level; they never get access. Social play is encouraged by making
grouping efficient, not by making solo play impossible.

Consequences: 8 cannot define a mob role that requires a party (no
mandatory healer, no tank-and-spank). 7.3 cannot include a class that
cannot solo; a healer must also do damage. Experience sharing in a group
must not penalise the group below solo rate.

### 2.5 Session length

**Thesis.** Long outings reward planning: pack supplies, delve, come back
laden. An hour in the dungeon is the classic shape.

**Antithesis.** Players of a small hobby server play in fifteen to
thirty minute windows. An hour-long outing means most sessions end in
the middle of one, with the player logging out somewhere unsafe or
abandoning the run.

**Synthesis (leaning).** An outing is ten to twenty minutes. That is
roughly ten fights with rests between them, each fight lasting six to
eight rounds at three seconds a round. A longer delve is several outings
chained by a player who chooses not to return to town, which is exactly
the attrition risk 2.3 wants. Session-scale goals (a level, a piece of
gear) span a few outings; career-scale goals span many sessions.

Consequences: 4.5's table is filled from these numbers. Regeneration
(`onTick`) is tuned so resting to full between fights takes about thirty
seconds, long enough to be a choice and short enough not to be a chore.

---

## 3. Stats

### 3.1 Stats act as multipliers

**Status: decided (2026-09-23).** Weapons and spells carry base numbers;
stats scale them. A stat never adds a flat amount.

Why: keeps item numbers legible (a sword's damage is its damage) and makes
stat allocation a build choice rather than a pile of small bonuses.

### 3.2 Which stats exist

**Status: open.**

- List the stats. Fewer is better; each must affect at least two things so
  no stat is a dump stat.
- Which stat multiplies weapon damage? Spell damage? Hit chance? Defense?
  Speed or attacks per round? Health and mana pools? Regeneration?
- Are mob stats the same set as player stats, or a simplified one?

### 3.3 Multiplier shape and ceiling

**Status: open.** This decision shapes itemization more than any other.

- Formula: linear (`1 + k * stat`), diminishing (`stat / (stat + c)`), or
  stepped?
- Ceiling: across the whole progression, what is the largest multiplier a
  stat can reach? (Suggested starting bound: 2x to 3x base.)
- Where do stat points come from: level, training, gear, all three?

---

## 4. Combat

### 4.1 Weapons have fixed damage

**Status: decided (2026-09-23).** A weapon's damage is a number, not a dice
expression. The number is multiplied by the wielder's stat multiplier.

Why: item tiers become the visible progression axis; damage is predictable
and comparable at a glance.

### 4.2 Where randomness lives

**Status: open.** With fixed damage and multiplicative stats, combat is
deterministic unless variance is added somewhere on purpose. Choose at
most one primary source:

- Hit chance (miss = 0 damage): swingy, easy to explain.
- Critical hits (rare bonus): keeps base damage fixed, adds highs.
- Damage spread (e.g. 90 to 110 percent): smooths without changing the mean.
- None: fights are fully predictable; tactics come from choices, not rolls.

Also decide whether randomness is symmetric for mobs and players.

### 4.3 Defense

**Status: open.** How does armor reduce damage?

- Flat reduction: simple but breaks fixed damage (a weapon below the armor
  value does nothing; a weapon above it is unaffected by more armor).
- Percentage reduction: scales cleanly with fixed damage; needs a cap.
- Hit-chance penalty: only meaningful if 4.2 chose hit chance.
- Which stat, if any, multiplies armor?

### 4.4 Action economy

**Status: open.**

- One attack per round, or attacks per round as a derived stat?
- Do weapons have a speed? (Fast weapon: lower damage, more attacks.)
- Does dual wield exist? Off-hand penalty?
- Can a player act during a round beyond auto-attacking (abilities,
  flee, quaff)?

### 4.5 Pacing targets

**Status: leaning (2026-09-23).** Numbers exist only to hit these. Set
them first, then let the simulator tell you whether a formula does. The
values below follow from 2.3 and 2.5 and are the first thing `simulate`
should be checked against.

| Target | Value |
|---|---|
| Rounds for an even fight (equal level, standard gear) | 6 to 8 |
| Health remaining after an even fight at full health | 40 to 60 percent |
| Win rate at +1 level, +3 levels, +5 levels | 80, 40, under 10 percent |
| Rounds to recover from a fight to full | about 10 (30 seconds) |
| Fights per outing before returning to town | about 10 |
| Even fights chained with no rest before death is likely | 2 to 3 |

### 4.6 Death

**Status: open.** What is lost on death: XP, gear, time, nothing? Where do
you return? Is there a corpse?

---

## 5. Items

### 5.1 Definition fields

**Status: leaning.** What an item definition carries, independent of any
formula. Weapons: damage, speed or attacks, hands, damage type. Armor:
slot, defense, and stat modifiers. All: level, weight, value, flags.

### 5.2 Power budget

**Status: open.**

- What does an item of level N look like? A table of damage and defense by
  level is the single most useful balance artifact to produce early.
- How many equipment slots matter? (Suggested: few. Every slot is a
  multiplier on the number of items to balance.)
- Do stat modifiers on gear stack with base stats before the multiplier is
  computed? (Interacts with 3.3.)

### 5.3 Rarity and drops

**Status: open.** Is there a rarity tier? Random affixes or fixed items?
Where do items come from: drops, shops, crafting?

---

## 6. Magic

### 6.1 Resource

**Status: open.** Mana pool, cooldowns, components, or a mix? What
regenerates it and how fast?

### 6.2 Casting

**Status: open.**

- Instant or cast time in rounds?
- Can casting be interrupted by damage or movement?
- Do spells scale with a stat multiplier the same way weapons do (3.1)?

### 6.3 Resistance and effects

**Status: open.**

- How does a target resist a spell: flat chance, stat contest, or none?
- Effect model: buffs, debuffs, damage over time, all as timed effects on
  a character (the engine's effect list is the only cross-round state
  scripts get).

### 6.4 Special rules

**Status: open.** The rules you said would differ from typical systems.
List each as its own sub-decision with the reason it exists.

---

## 7. Progression

### 7.1 Experience and levels

**Status: open.** XP per kill formula (level gap, mob difficulty), XP curve
per level, level cap.

### 7.2 What a level grants

**Status: open.** Stat points, health and mana, access to abilities or gear?
(`onLevel` returns these.)

### 7.3 Classes or free-form

**Status: open.** Classes, professions, skill trees, or pure stat
allocation? This decides whether "build" is chosen once or continuously.

---

## 8. Mobs

**Status: open.**

- Are mobs built from the same stat model as players, or from a simplified
  "level implies everything" table?
- How is mob difficulty expressed to the builder: level alone, or level plus
  role tags (brute, caster, swarm)?
- Do mobs use abilities or only auto-attack?

---

## 9. Open questions log

Anything not yet placed in a section above.

- 
