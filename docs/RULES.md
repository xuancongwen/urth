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

---

## 1. Constraints from the engine

What a rule script will and will not be able to touch once milestone 6
lands. This section is owned by the engine, not the designer.

### Hook points (planned)

Each hook is a script function. Inputs are read-only views; outputs are
plain values the engine applies.

| Hook | Called when | Inputs | Returns |
|---|---|---|---|
| `resolveAttack` | each attack in a combat round | attacker, defender, weapon (or unarmed), round number | `{hit, damage, crit, messageKey}` |
| `resolveCast` | a spell finishes casting | caster, target(s), spell definition | `{success, effects[], messageKey}` |
| `onTick` | once per round for each character | character | `{healthDelta, manaDelta, effectsToExpire[]}` |
| `onLevel` | a character gains a level | character, new level | `{statDeltas, healthMax, manaMax}` |
| `xpForKill` | a character kills a mob | killer, victim | integer |
| `derivedStats` | any base stat or equipment changes | character, equipment | `{attackMultiplier, defense, speed, ...}` |

### What scripts see

- Character: name, level, base stats, derived stats, current and max
  vitals, equipment by slot, active effects, room.
- Item: definition fields (see section 5) plus instance state.
- Spell: definition fields (see section 6).
- A seeded random source, so simulations are reproducible.

### What scripts cannot do

- Move characters, create or destroy items, send arbitrary text. Scripts
  return values; the engine applies them and renders messages.
- Block. A hook that takes more than a tick's budget is killed and logged.
- Keep state between calls except through effects on characters.

### Time

- A tick is `timing.tick_ms` (100 ms). Commands are processed once per tick.
- A round is `timing.round_seconds` (3 s). Combat, regeneration, and effect
  durations are counted in rounds.

### Balance tooling (milestone 6)

- A `simulate` admin command runs N fights between two definitions and
  reports win rate, mean rounds, and damage distribution.
- Scripts hot-reload. A formula change is visible on the next round.

---

## 2. Design goals

**Status: open.** Fill in before choosing numbers. Everything in sections
3 to 8 should trace back to a line here.

- What should a player be thinking about during a fight? (Which target,
  which ability, when to flee, or nothing because it is automatic.)
- How much should gear matter relative to level and to stat allocation?
- How lethal is the world? Is death routine, rare, or catastrophic?
- Is the game solo-first, group-first, or both?
- What is the intended session length for one "outing" (leave town, fight,
  return)?

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

**Status: open.** Numbers exist only to hit these. Set them first, then let
the simulator tell you whether a formula does.

| Target | Value |
|---|---|
| Rounds for an even fight (equal level, standard gear) | |
| Win rate at +1 level, +3 levels, +5 levels | |
| Rounds to recover from a fight to full | |
| Fights per outing before returning to town | |

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
