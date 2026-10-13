# Rules

A living design document for the game rules: combat, stats, items, magic,
progression. Each section records a decision, the reasoning, and the
questions still open. Change a decision by editing it and noting the date.
Work still to be built or decided is tracked in Trackstar (project Urth),
not here.

Rules are formulas plugged into fixed seams. Section 1 is the contract the
engine offers; everything after it must be expressible through that contract.
If a rule cannot be, widen the contract, do not put the rule in Go.

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

What a rule script can and cannot touch. This section is owned by the
engine, not the designer.

### Hook points (implemented; combat core, 2026-09-24)

Each hook is a global function in `data/scripts/*.js`. Inputs are
read-only snapshots; outputs are plain objects the engine applies. The
live rules are `data/scripts/rules.js`. Hooks marked optional may be
left out; the engine then uses a quiet default.

| Hook | Called when | Inputs | Returns |
|---|---|---|---|
| `resolveAttack` | each swing (the engine's swing meter decides when) | attacker, defender, weapon (null if unarmed), round, hand (`wield`, or `second` for the off hand, 4.4) | `{hit, damage, crit, verb, stage, resisted, procs:[{verb, damage, back, kind, resisted}], heal, effects:[{on, kind, params, rounds}]}`; `stage` is `dodge`, `block`, or `miss` when `hit` is false. `resisted` marks a swing (or a proc) that landed on a target immune to its kind (4.8), narrated as shrugged off. The rest is the 5.4 `onhit` phase, applied only on a hit: each proc is an extra strike narrated by its verb, on the defender or, with `back`, on the attacker (thorns); `heal` goes to the attacker; effects attach to the defender (`on: "target"`) or attacker (`"self"`) |
| `derivedStats` | login, spawn, equipment change, level, effect change, script reload | character | `{healthMax, manaMax, staminaMax, speed, offSpeed}`; `speed` is swings per round, fractional allowed; `offSpeed` the second weapon's (4.4), zero without one |
| `onTick` | once per round for every character, except a mob it last left unchanged while nothing in its view has changed since (D28); so it must change nothing again for the same view | character | `{healthDelta, manaDelta, staminaDelta, skills:{id: rating}, aura:[{verb, damage, kind}]}`; each aura entry strikes every enemy fighting the character in its room |
| `resistDamage` (optional) | each aura strike, per enemy it lands on | target, kind, damage | the damage left after the target's resistance to the kind (4.8); without it an aura lands whole |
| `damageKinds` (optional) | `urth check` | | every damage kind content may name; the check warns on any other (4.8) |
| `xpForKill` | a player kills a mob | killer, victim | integer |
| `xpToLevel` | after any experience gain, and on death | level | integer (total xp needed to reach it) |
| `onLevel` | a character gains a level | character, new level | `{statPoints, featPicks, statDeltas:{}, message}` |
| `onCreate` (optional) | a character's first login, or one with no stats | character | `{stats:{}, statPoints, featPicks, message}` |
| `deathRules` (optional) | every death | | `{xpFraction, xpLevelCap, corpseRounds, respawnHealth}` |
| `pickChance` (optional) | each `pick` at a locked door | character, `{name, key}` | a number from 0 to 1, the chance one try opens the lock; 0.5 without the hook. A pickproof door never opens |
| `itemBaseline` (optional) | content load, script reload | item prototype as stated | `{weapon:{damage, spread, speed, hands, kind, verb}, armor:{defense}, color}`; `color` is the color a named item's or a material's name is shown in |
| `mobBaseline` (optional) | content load, script reload | mob prototype as stated | `{stats:{}, health, xp, attack:{...}, armor:{...}}` |
| `featList` (optional) | the `feat` command, level-up, `score` | | `[{id, name, level, description, requires:[ids], effect:{kind, params}}]` |
| `standardKit` (optional) | `simulate fighter:N` | level | `[{name, type, slot, baseline, weapon, armor}]`, built into resolved prototypes at that level |
| `skillList` (optional) | typing a skill's name, `skills`, `practice`, level-up | | `[{id, name, level, passive, target, cooldown, stamina, start, innate, price, requires:[{material, count}], description}]`; `stamina` is the cost of a use (7.7) |
| `rollItem` (optional) | a reset creates a plain weapon or armor, and for each extra drop (5.3) | item, `{source: "reset" or "drop", level}` | `{rarity, color, prefix, suffix, effects:[{kind, params}]}`; an empty `rarity` leaves the item plain, the effects are stored on the instance |
| `itemTier` (optional) | a rolled item is loaded from a player file (5.3) | item | `{rarity, color}` for the count of effects the item carries; an empty `rarity` leaves it as it was |
| `lootRules` (optional) | a mob dies | victim | `{drops, levelBand}`: extra items for the corpse, drawn from the rollable prototypes of the victim's area within the band |
| `expeditionRules` (optional) | `expedition` | player | `{levelBand, fights, maxFights, restBelow, retreatBelow}`: which of an area's mobs a run may meet, its default and longest length, and the health fractions at which the character sleeps and turns for home |
| `moneyFor` (optional) | a mob dies | victim | silver the corpse holds; the prototype's `silver` overrides |
| `questRules` (optional) | `quest request`, and when a quest ends | player | `{levelBand, rounds, cooldown, quitCooldown}`, the band in levels and the timers in rounds (7.6) |
| `questReward` (optional) | a quest is drawn | player, quest `{kind, target, name, level, area, room, rounds}` | `{points, silver, xp, message}`, shown on offer and paid on completion |
| `useSkill` | an active skill is used | user, target or null, skill, the skill's effect (with its rating in `state`) | `{ok, message, hit, stage, damage, verb, resisted, effects:[{on, kind, params, rounds}], skills:{id: rating}, cooldown, stamina}`; `cooldown` and `stamina`, when present, replace the skill's own for this use |
| `spellList` (optional) | `cast`, `spells` | | `[{id, name, branch, school, deity, castRounds, interruptOnDamage, cooldown, mana, materials:[{material, count}], target, save, saveEffect, description}]`; `materials` is for the few spells that want items |
| `resolveCast` | a cast completes | caster, targets (array of views), spell | `{ok, message, consume:[item ids], targets:[{index, damage, heal, saved, negated, effects:[{kind, params, rounds}], message}], casterEffects:[...]}` |

### What scripts see

- Character: `{name, level, xp, stats:{}, statPoints, featPoints, silver, health,
  healthMax, mana, manaMax, stamina, staminaMax, speed, equipment:{slot: item}, effects:[],
  schools:[], deity, isPlayer, fighting, position, group:[brief], enemies:[brief],
  casting:{spell, rounds} or absent, cooldowns:{id: rounds},
  room:{vnum,name,area}}`, where `position` is `standing`, `resting`,
  or `sleeping`, plus for mobs `vnum`, `flags`, and
  `mob:{vnum, flags, health, xp, attack, armor, effects}` carrying the
  prototype's resolved natural numbers. A brief is `{name, level,
  health, healthMax, isPlayer}`, so views do not recurse. `stats` is whatever the rules put
  there; the engine does not define the stat set.
- Item: `{id, vnum, name, type, slot, level, baseline, weight, value,
  flags, weapon:{damage, spread, speed, hands, kind, verb},
  armor:{defense}, effects:[], mods:{}}`, plus `material` and
  `rarity` on materials, `school` on totems, and `sacrifice` on an item
  a god accepts. `id` is the instance id that `consume` names. The weapon and armor numbers are the
  *resolved* ones: what the builder stated, with the baseline filling
  the gaps.
- Effect: `{kind, params:{}, state:{}, rounds}`. `rounds` 0 is permanent.
  An item's `effects` merges the prototype's intrinsic effects with any
  applied to the instance.
- `random.int(n)`, `random.float()`, `random.roll(count, sides)`: seeded
  per simulation so runs are reproducible. `log(...)` writes to the server
  log.

### What scripts cannot do

- Move characters, create items, or send arbitrary text. Scripts return
  values; the engine applies them and renders messages.
- Destroy items directly. A hook that needs to consume items (casting,
  6.2; unlocking a school, 6.1) returns their ids in a `consume` list and
  the engine removes them from the actor's inventory before applying the
  rest of the result. Decided 2026-09-24; see D17. If any listed item is
  not in the inventory the whole result is rejected, so a spell never
  half-fires. A spell's listed materials are committed by the engine when
  the cast begins (6.4), so `consume` is for anything beyond them.
- Block. A hook that runs longer than 50 ms is interrupted; the engine
  uses a safe default (a miss, no regen, 1 max health) and warns admins
  once per load.
- Keep state between calls. Effects carry a `state` map for that; the
  engine persists it. The first write-back is skill ratings: a `skills`
  map in a hook result updates `state.effectiveness` on the named skill
  effects. Other state stays read-only until something needs it.
- Attach effects except through a cast. `resolveCast` returns effects
  per target and for the caster and the engine attaches them; the attack
  pipeline cannot yet.

### What the engine owns

- The swing meter (4.4): each round a combatant's meter gains its
  `speed`; every whole point is one `resolveAttack` call. A second
  weapon has its own meter, filled by `offSpeed`, and the hands take
  turns.
- Hands (4.4): a two-handed weapon, a second weapon, and a shield are
  refused when they would not fit together.
- Effect lifetimes (5.4): timed effects count down once per round and
  are dropped at zero; `derivedStats` is re-run when a list changes.
- Death (4.6): the corpse (a container holding everything carried and
  worn, decaying after `corpseRounds`), the experience loss with its cap
  and floor, respawn at the start room at `respawnHealth`.
- Stat points and `train`: banked from `onLevel` and `onCreate`, spent
  one at a time on any stat the character has.
- Feat picks and `feat`: banked the same way; `feat` lists the feats
  learned and available, `feat all` every feat with its status (needs
  level N, needs another feat, and so on), and `feat <name>` spends a
  pick. A learned feat is a permanent
  effect whose params carry `feat: <id>`, so the rules see it like any
  other effect and stacking follows the kind.
- Baseline resolution: `itemBaseline` and `mobBaseline` run for every
  prototype at load and after every successful script reload; a live
  mob keeps the base stats it spawned with but its derived values move.
- Casting (6.4): one cast in progress per character with rounds left;
  `castRounds` 0 resolves on the command; movement always cancels;
  damage cancels when the spell says so; the spell's mana is spent and
  any materials leave the inventory when the cast begins, and a cast
  the caster cannot pay for is refused; cooldowns count down per round. Targets are
  built by the engine from the spell's `target`: single (named or the
  current target), ally (named or self), self, group (members here),
  area (everyone here not in the caster's group). A hostile cast starts
  the fight. Access (since 2026-10-12): a spell's `tier` and its
  `opposite` school come from `spellList`; `swear <school>` in a room
  whose `order` holds the school (the `magicSchools` hook) sets the
  character's one specialty, and `brandish <totem>` does the same in the
  balance lab; `sacrifice <item>` in a room whose `temple` matches the
  item's `sacrifice` makes the character that god's apostle (one god at
  a time).
- Groups (4.7): `follow`, `group`, `gtell`, `assist`; followers move with
  the leader; grouped players here auto-assist when a member's fight
  starts; a mob flagged `assist` joins a mob of its kind; `kill` on a new
  target switches; a fighter whose target is gone turns on whoever is
  still fighting it; every grouped player here earns a kill's experience
  as if alone; a room flagged `safe` forbids fighting and hostile casts.

### Time

- A tick is `timing.tick_ms` (100 ms). Commands are processed once per tick.
- A round is `timing.round_ms`: 3000 ms, ROM 2.4's combat round, as of
  2026-10-11 (it was 3 s, then 2 s from 2026-09-24, and the designer
  brought it back to ROM's pace). Combat, cooldowns, cast times, and
  effect durations are counted in rounds and run at its pace.
  Regeneration out of a fight, corpse decay, and quest clocks are meant
  as wall-clock time: the rules keep them in seconds or minutes and
  turn them into rounds with their own `P.roundSeconds`, which must
  match `round_ms` (the server warns at start if it does not, and a
  test holds the shipped pair together). The field is in milliseconds
  so fractional seconds can be tried in playtesting; it must be at
  least one tick.

### Balance tooling (implemented)

- `simulate <mob | me | fighter[:N]> <mob> [fights] [seed]` runs detached fights through
  the same hooks and reports win rates, rounds (mean and standard
  deviation), damage and swings per fight, and the winner's health left
  as mean and standard deviation, which is the 4.5 variance row. Mobs
  are equipped as their reset entry spawns them; `me` uses your sheet and
  gear; `fighter:N` is a level-N character with a fresh sheet, no points
  spent, in the rules' `standardKit` at level N, which is the "player at
  level in standard gear" the 4.5 targets are written for. A seed makes
  the run reproducible. The run blocks the world loop, so keep batches
  to a few thousand on a live server.
- `data/world/balance/` holds one plain mob per level, vnum 900 + level,
  so any gap can be measured: `simulate fighter:5 908 1000 1` is a
  level-5 fighter against a level-8 dummy.
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

**Synthesis (leaning, revised 2026-09-24).** Three pillars, each
dominant in its own right. Level, gear, and stats are all meant to be
felt strongly; a character who is far ahead on any one of them is far
ahead, full stop. What distinguishes them is not their weight but their
job. Level *gates*: it decides which mobs give experience (and, if 5.2's
leaning changes, what can be worn). Gear *sets the base numbers*: the
weapon is the damage, the armor is the defense, and the item table (5.2)
is the progression curve made visible. (Revised 2026-10-12: past the
first levels the weapon is the smaller part of the damage, and what is
on it, its affixes and procs, is the larger; 5.2.) Stats *multiply* and are the
build: two characters of the same level in the same gear differ by
their stat allocation.

No caps, ceilings, or fixed ratios between the pillars for now. The
antithesis's twinking concern is real but is a balance problem, and
balance work happens in the simulator once the formulas exist. If a
pillar turns out to swamp the others, the fix is a cap added then, with
the simulator run that justified it.

Earned magic (6.1) is a fourth pillar, gated by content rather than by
numbers, and uncapped like the other three.

Consequences: 3.3 has no multiplier ceiling yet. 7.1's `xpForKill` still
decays with level gap, since that is a gate, not a cap. Item level
requirements are settled in 5.2 (leaning no).

### 2.3 Lethality

**Thesis.** Death should be rare and expensive. If dying costs little,
fights carry no tension and `flee` is never typed.

**Antithesis.** This is a small server. Every character lost to a harsh
death is a player who may not come back, and every balance mistake that
slips past the simulator becomes a player-losing bug instead of an
annoyance. ROM's routine death (lose some experience, corpse in the
room, walk back) has kept players for thirty years precisely because it
is survivable.

**Synthesis (leaning, revised 2026-09-24).** Death is uncommon and
consequential. It should feel like an event, not a tax, but the
consequence is bounded: the character is never destroyed. What it costs
is respawning in town at low health, losing experience, and leaving a
corpse that holds the inventory until the player walks back for it.
That package is already heavy enough to make `flee` worth typing; it
does not need permadeath or gear destruction on top. Whether the
experience loss can cross a level boundary is left open.

Frequency is controlled by making death the result of a decision the
player can identify afterwards, not of a bad roll. An even fight at full
health should almost never kill; fighting above level, or chaining
fights without resting, is what does. A careful player at level rarely
dies. A reckless one does, and knows why.

Consequences: 4.2 must choose low variance (one fight cannot swing from
comfortable to fatal). 4.6 is mostly written by this section. 4.5's
recovery time is what makes resting a real choice, and its attrition row
is tuned so that death takes a run of poor choices, not one.

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
roughly ten fights with rests between them, each fight lasting eight
to ten rounds: fifteen to twenty seconds at the two-second round this
was written for, twenty-four to thirty seconds at ROM's three-second
round restored on 2026-10-11, a deliberate slowing of the fight. A longer delve is several outings
chained by a player who chooses not to return to town, which is exactly
the attrition risk 2.3 wants. Session-scale goals (a level, a piece of
gear) span a few outings; career-scale goals span many sessions.

Consequences: 4.5's table is filled from these numbers. Regeneration
(`onTick`) is tuned so sleeping to full between fights takes about thirty
seconds, long enough to be a choice and short enough not to be a chore.
Standing out of a fight takes three times as long and resting twice as
long (decided 2026-09-28), so stopping to rest is worth a command:
`regenSeconds` is the standing figure (ninety seconds; the rules turn
it into rounds), `restRegen` and `sleepRegen` the multipliers. Resting costs movement, fighting, and casting until the
character stands; a sleeper can do almost nothing until it wakes, and a
fight wakes it.
The targets are in seconds; the round counts follow from the round
length, so a shorter round means more rounds per fight, not shorter
fights, and more swings for 4.2's variance to average over.

---

## 3. Stats

### 3.1 Stats act as multipliers

**Status: decided (2026-09-23).** Weapons and spells carry base numbers;
stats scale them. A stat never adds a flat amount.

Why: keeps item numbers legible (a sword's damage is its damage) and makes
stat allocation a build choice rather than a pile of small bonuses.

### 3.2 Which stats exist

**Thesis.** The six from Dungeons and Dragons: Strength, Dexterity,
Constitution, Intelligence, Wisdom, Charisma. Every player knows them,
builders know what a "strong" mob means, and six names leave room for
jobs that do not exist yet.

**Antithesis.** The rule at the top of this section is that each stat
must affect at least two things or it is a dump stat. In D&D itself,
Charisma is the dump stat for most of the table, and Intelligence and
Wisdom are told apart by class. There are no classes here (7.3), so
Intelligence and Wisdom need distinct jobs on their own merits, and
Charisma needs two things worth having. Six multiplier axes with no
caps (2.2) is also six dimensions for the simulator to sweep. Four
would be easier: Might, Agility, Mind, Presence.

**Synthesis (leaning, 2026-09-24).** Keep the six names. Names are
cheap, familiarity is worth something, and the engine does not define
the stat set (section 1), so this costs no Go. But a name is not a
stat: a stat exists when it has at least two jobs in the table below,
drawn from numbers the system actually has. Any stat with fewer than
two jobs when the rules are built is merged into its neighbour. The
table is the decision; the names are the flexibility.

**Revision (decided, 2026-10-11): five stats; Charisma is gone.** No
rule ever read Charisma. Both of its jobs in the table below would have
been the sort a player feels obliged to buy rather than wants, which
makes the build choice more min-maxy, not less. The one thing that
would justify it is making it the power stat of a third branch of
magic, as D&D does for sorcerers and warlocks, and a branch is too much
to build just to keep a stat. The themes it carried (presence,
command, charm) go to the stats that already work: a commanding
presence is Wisdom, a quick tongue is Dexterity. If a third branch is
ever wanted, a stat costs no Go to add back. Folding Constitution into
Strength was considered at the same time and rejected: health is the
one thing every build needs, so Strength would become the stat no one
can skip, and the trade-off between hitting hard and lasting would
vanish.

The numbers available to assign, gathered from the sections above:
weapon damage multiplier (4.1), spell power multiplier (6.4), defense
`D` multiplier (4.3), dodge and block chances (4.3), health maximum and
health regeneration (4.5), attacks per round or speed (4.4, open),
resistance to spells and to timed effects (5.4), carrying weight, and
the two things discovery (6.1) and materials (6.2) make matter: what
NPCs will tell you and what things cost.

| Stat | Job 1 | Job 2 | Job 3 |
|---|---|---|---|
| Strength | weapon damage multiplier | block chance (shields, parrying weapons) | carrying weight |
| Dexterity | dodge chance | weapon speed multiplier (4.4) | |
| Constitution | health maximum | health regeneration | defense `D` multiplier |
| Intelligence | arcane spell power multiplier (6.1) | potency of effects the character applies (buff strength, DoT damage) | |
| Wisdom | divine spell power multiplier (6.1) | resistance to spells and timed effects (shorter, weaker) | optional: the width of the character's own stat band (4.2), a wise character being a steady one |

Intelligence and Wisdom split the way D&D splits them: arcane power
against divine power (6.1), so the two are told apart by which branch
of magic a character pursues rather than by class. Wisdom's optional
third job, narrowing the stat band, is a proposal: it fits the name but
means one stat modifies the randomness of the others, and it should be
adopted only if the simulator shows it is felt. What NPCs tell you and
what things cost belong to no stat; discovery (6.1) and prices are
authored, not rolled.

Mobs use the same five with prototype values; the builder sets what is
needed and the rest default. A mob's stats are how "brute" or "caster"
is expressed (8), not a separate role system.

Consequences: 4.3's open questions close (dodge from Dexterity, block
from Strength, `D` from Constitution). 4.4 honours Dexterity as the
speed multiplier. `derivedStats` reads all five.

### 3.3 Multiplier shape, and a starting point

**Status: leaning (2026-09-24).** The shape is decided; the numbers are
an arbitrary starting point chosen so that there is something to
playtest, not something the simulator picked. Every value in the table
below is expected to change.

Shape: linear, no ceiling (2.2). A stat's multiplier is

    multiplier = 1 + k * (stat - 10)

with 10 as the baseline, as in D&D, so a stat of 10 is exactly the
item's number and every point above or below it moves the number by
`k`. Linear is the only shape that keeps "one point is worth the same
everywhere", which is what makes a stat point at level 30 feel like
one at level 3. The diminishing and stepped shapes in the old text of
this section are kept out for the same reason the document rejects
caps: they are balance tools, to be reached for with a simulator run
in hand, not a starting assumption.

Where stat points come from: character creation, every level (7.2),
and `stat` effects on gear (5.4). Whether a trainer can also sell them
is open.

Starting values, to be adjusted in playtesting:

| Quantity | Starting value | Why this and not another |
|---|---|---|
| Baseline stat | 10 in each of the five | D&D-familiar; multiplier of exactly 1 |
| `k`, per point | 0.05 (5 percent) | 7.2 asked for a point to be visible, on the order of a few percent |
| Points at creation | 6, placed freely | enough to make a build identity on day one, not enough to hollow a stat |
| Points per level | 2 | with few levels (7.2), two is one meaningful choice per level beside the feat |
| Stat band (4.2) | ±10 percent of the multiplier | the value the variance table in 4.2 was computed at |
| Level multiplier (7.2) | 1 + 0.02 * (level - 1) | "slight": ten levels is 20 percent, which is felt but is less than a gear tier |
| Level growth | 1.10 per level, `levelGrowth` | every baseline (damage, defense, health) is its level-1 value times 1.10^(level-1); decided 2026-09-24 after the linear curve failed the 4.5 gap row |
| Health base | 50 at level 1, times level growth and Constitution's multiplier | with a standard weapon this gives about 10.5 rounds for an even fight at every level |
| Dodge base | 3 percent | a few percent, per 4.3's sizing |
| Block base | 0 | per 4.3 |
| `K` for reduction (4.3) | 20 at level 1, times level growth to the power `kGrowth` (1.0) of the attacker's level | starter armor totals about 3 defense, 13 percent reduction; with `kGrowth` 1.0 a tier of armor reduces the same share against an attacker of any level, so fight length does not drift with level |
| Default weapon spread | 0.2 | the value the variance tables were computed at |

The first playtest question these values pose is whether five percent
per point is too little to feel at creation with six points (a 30
percent swing in one stat) or too much at level 20 with forty-six
(a 230 percent multiplier). Both ends should be tried before `k` moves.

---

## 4. Combat

### 4.1 Weapons carry a damage number and a spread

**Status: decided 2026-09-23, revised 2026-09-24.** A weapon's damage is
a single number that is the *mean* of what it does, plus a spread that
is part of the item's identity. The number is what a player compares
between two swords; the spread is how reliable the sword is. Both are
multiplied by the wielder's stat multiplier.

Why the revision: the original rule chose one number over dice for
legibility. Keeping the mean as the headline number preserves that; a
spread field beside it costs nothing in legibility and buys a design
axis. A dagger that always does close to its number and a maul that
swings wide around the same number are different weapons in a way that
two flat numbers cannot express.

Armor has no spread (revised 2026-10-12): its defense is a fixed
number. A defense that rolled each hit read as noise, since the player
cannot see it and nothing they choose moves it. The chance in defending
comes from parries, dodges, and shield blocks (4.3), which are events
the fight reports.

Item form: `damage: 10` with `spread: 0.2` means each swing is drawn
uniformly from 8 to 12 before multipliers. A missing spread is zero.
There is no global cap on spread (2.2); it is an item field the builder
sets.

### 4.2 Where randomness lives

**Thesis.** Randomness lives in the hit roll, as in ROM and most of its
descendants. A swing either lands or misses; damage on a hit is
whatever the weapon says. Swingy, familiar, and easy to explain: "you
missed".

**Antithesis.** A binary roll is the most violent kind of variance a
fight can have. Over seven swings at an 80 percent hit chance, the
total damage of an even fight varies by about 19 percent (standard
deviation), which is enough for an even fight at full health to kill
now and then. That is exactly what 2.3 forbids: death from a roll
rather than from a decision. Crits have the same problem from the other
side; a spike is a miss with the sign flipped.

**Synthesis (leaning, 2026-09-24).** Randomness is *continuous and
symmetric*, and it lives in two places with different widths:

1. **Equipment.** Each swing draws damage from the weapon's spread
   (4.1), and each hit draws the defender's reduction from the armor's
   spread. This is the primary source and the wider one. Items differ
   in how reliable they are, which is a thing to itemise.
2. **Stats.** Each stat's multiplier is drawn from a narrower band
   around its value. Every stat has one, so the jitter applies to
   whatever the stat touches: damage, defense, regeneration, spell
   power. This is the secondary source and always narrower than the
   equipment's.

The attacker has no to-hit roll and no critical roll. Every swing is
delivered at its drawn value. The one binary outcome in the game sits
on the defender's side: the avoidance tries in 4.3's pipeline (dodge,
block), which are intrinsic to every character but start small and are
sized by the variance row in 4.5. A "miss" is a dodge or a block, so
the message ROM players expect exists, and it is the defender's doing.

**Critical hits are not natural (decided 2026-09-24).** No character has
a base critical chance or multiplier. A crit exists only as an *effect*
(5.4): something a feat, an item, or a spell grants, with its own
chance and multiplier as parameters. A character with no such effect
never crits. This keeps the base game at the low variance 2.3 asks for
and makes a crit build something a player assembles on purpose.

Why this satisfies 2.3: continuous per-swing spread averages out over a
fight. With a ±20 percent weapon spread and a ±10 percent stat band,
the total damage of a seven-swing fight has a standard deviation of
about 5 percent, against 19 percent for the hit-roll model. Individual
swings feel varied (about 13 percent per swing) while the fight's
outcome stays close to its mean. The 4.5 row "health remaining after an
even fight" is what the simulator checks this against.

An honest note on the stat band: at ±5 percent it is nearly invisible
in outcomes (fight-level deviation moves from 4.4 to 4.5 percent). It
earns its place as flavour on individual swings and by being the only
randomness a naked character has, not by changing who wins. If it needs
a distinct job, the leaning is to draw it once per round rather than per
swing, so a round can read as a good or bad one. Open.

Randomness is symmetric between mobs and players: a mob's natural
attack is a weapon with a damage number and a spread, and its stats
have the same band.

Consequences: `resolveAttack` gains nothing; `weapon.spread` is added
to the item view. Damage stays
positive after a low draw; the floor is zero, not negative. The simulator should report the
standard deviation of health remaining, not only the mean.

### 4.3 Defense is a three-stage pipeline

**Thesis.** Defense is one number that reduces damage. The prior text
of this section argued flat versus percentage and settled on an
asymptotic reduction; that reasoning is kept below as stage 3.

**Antithesis.** A single reduction has no texture. It cannot express a
nimble character who is hard to hit but fragile when hit, or a shield
that stops some blows outright and does nothing for the rest. Every
defensive item and stat would be feeding one number, so defensive
builds would all feel the same and defensive effects (5.4) would have
only one phase to act in.

**Synthesis (leaning, 2026-09-24).** Defense is three stages, run in
order for every incoming swing. Each stage is an intrinsic property of
every character, with a base value, and each is modified by stats,
equipment, and effects. The first two *mitigate*: the swing is stopped
entirely or it is not. The third *reduces*: whatever gets through is
scaled down.

| Stage | Kind | Intrinsic base | Modified by | Effects phase |
|---|---|---|---|---|
| 1. Dodge | try: the swing misses | small, from the body | a stat; light versus heavy armor; effects | `dodge` |
| 2. Block | try: equipment stops it | zero without a shield or parrying weapon | the item; a stat; effects | `block` |
| 3. Reduce | continuous | zero when naked | armor `D` and its stat; effects | `reduce` |

The two tries are independent rolls, so combined avoidance is
`1 - (1 - dodge)(1 - block)`. That rises toward but never reaches one,
which is the same no-cap-but-asymptote shape as stage 3 and needs no
ceiling. Both chances take the stat band from 4.2 like any other
stat-derived number.

**Stage 3, reduction.** Flat reduction has a threshold: below the armor
value a weapon does nothing, which is immunity by accident, and with
4.1's spread the low end of a draw hits the threshold while the high
end does not, so armor reshapes the weapon's randomness instead of
scaling it. A plain percentage scales cleanly but must be capped below
100, and 2.2 forbids caps. So:

    reduction     = D / (D + K)
    damage taken  = roll * K / (D + K)
    effective health = health * (1 + D / K)

`D` is the sum over worn armor of each piece's defense (fixed; 4.1),
multiplied by the defending stat's multiplier. `K` is a unit,
not a cap: it is how much defense doubles effective health. Since the
baselines grow geometrically with level (5.2), `K` grows with the
*attacker's* level at the same rate (revised 2026-09-24), so a piece of
armor at level N reduces the same share of a level-N attacker's blow as
a level-1 piece does of a level-1 blow, and high-level armor is
correspondingly strong against low-level attackers. The exponent
`kGrowth` lets that be softened; at 0.5, reduction rises slowly with
level and fights lengthen slowly. Effective health is *linear* in `D`, so every
point of defense is worth the same from the first to the thousandth
even though the displayed percentage flattens. "Defense 40, K 40" means
doubled effective health, which is as legible as a weapon's damage
number. Damage floors at zero, never below. Mobs use the same three
stages with prototype values.

**The cost of the tries, stated plainly.** Stages 1 and 2 are the only
binary randomness in the game, and they are the largest variance lever
it has. With a ±20 percent weapon spread alone, the total damage of a
seven-swing fight has a standard deviation of about 4 percent. Adding
avoidance:

| Combined avoidance | Fight-level deviation |
|---|---|
| 0 percent | 4 percent |
| 5 percent | 10 percent |
| 10 percent | 13 percent |
| 20 percent | 19 percent |

Five percent avoidance doubles the variance of a fight; twenty percent
reproduces the hit-roll model that 4.2 rejected. The pipeline is worth
that cost for the texture it buys, but only if the tries start small.
So: the intrinsic base for dodge is a few percent and the base for
block is zero, and high avoidance is something a build *reaches*
through a stat, light armor, a shield, and effects, in the same spirit
as crits. The variance row in 4.5 is the check: the simulator must
report fight-level deviation for a character at level in standard
gear, and if avoidance at level pushes it past the target, the bases
or the modifiers come down. No cap; a target the simulator enforces.

Consequences: 5.4's `defend` phase becomes three phases, `dodge`,
`block`, and `reduce`. `resolveAttack` returns `hit: false` on a dodge
or block, and the `verb` says which, so the transport can render "dodges"
and "blocks" distinctly. `derivedStats` gains `dodge` and `block` so the
values are visible on `score`. Armor items may carry an effect that
lowers dodge (heavy) or raises it (light); this is an effect, not a
field, per 5.1. Shields are armor with a `block` effect intrinsic to
them. `K` is the first tuning constant the simulator sweeps.

Stats: dodge from Dexterity, block from Strength, `D` from
Constitution (3.2).

**Open:** whether the
stat band is drawn per hit or per round (shared with 4.2); whether a
block can be partial (absorb a fraction) rather than binary, which
would move variance from the tries to the reduction and may be the
right call for shields.

### 4.4 Action economy

**Thesis.** One attack per round for everyone, as the placeholder does.
The simplest model, and it makes damage per round equal the weapon's
number.

**Antithesis.** Then a dagger and a maul are the same weapon with
different numbers, and there is no reason for a fast weapon to exist.
ROM's answer was extra attacks as skills (second attack, third attack)
rolled as chances, which is binary randomness of the kind 4.2 avoids,
and it still left weapons themselves speedless.

**Synthesis (leaning, 2026-09-24).** A weapon has a *speed*: swings per
round, which may be fractional. Speed 2 swings twice a round; speed
0.5 swings once every other round. The rules keep a *swing meter* per
combatant: each round it adds the combatant's total speed, and while
the meter is at least one, a swing resolves and one is subtracted. That
handles any rate without a roll, so a slow weapon is exactly as
reliable as a fast one, just lumpier. Speed is per round.

Total speed is the weapon's speed, multiplied by Dexterity (3.2, which
gives Dexterity its second job as promised), plus any extra attacks
from effects. The extra attacks are where ROM's second and third attack
live: each is a feat (7.2) that adds one attack per round to the
meter, level-locked, deterministic, no roll. A half-attack feat (one
extra attack every other round) is equally expressible and is a
plausible lower-level rung.

**An extra attack is a weapon's worth of swings (decided 2026-10-11).**
At first an extra attack added one swing whatever the weapon. A maul's
swing carries four times a dagger's damage, so the same feat was worth
four times as much on the maul: with Second, Third, and Fourth Attack
at level 20 a maul dealt 289 a round against a sword's 164. Now an
extra attack adds the weapon's own speed to the meter, so speed is
`weapon speed * (Dexterity multiplier + extra attacks)`: a sword swings
once more, a dagger twice, a maul every other round, and each adds the
same damage a round. Every source of extra attacks works this way
(feats, Haste, Berserk, Twin and Triple Strike, the haste affix), and
the off-hand feats add the second weapon's speed. Weapons keep their
rhythm and their traits as their identity, not a different worth for
the same feat.

What speed buys, and what to watch:

- Damage per round is `damage * speed` before multipliers, and that
  product is the number to show a player comparing weapons; the
  headline damage number (4.1) is per swing.
- Stage 3 reduction (4.3) is proportional, so a fast weapon and a slow
  one of equal damage per round are equal against armor. Flat
  reduction would have punished fast weapons; this is one more reason
  4.3 chose the asymptote.
- Each swing draws its own spread and faces its own dodge and block
  tries, so fast weapons have *lower* fight-level variance than slow
  ones at the same damage per round. That is a real axis: the maul is
  the swingy choice, the dagger the steady one, and 4.2's variance
  table should be checked at both ends.
- Per-swing effects (`onhit` procs, 5.4) fire more often on fast
  weapons. That is intended, and it is the fast weapon's identity;
  effect kinds that should not scale with speed carry a per-round
  chance instead of a per-swing one, decided per kind.

Beyond the auto-attack, one player action per round (2.1): an ability,
`flee`, an item.

**Hands, two weapons, and two-handers (decided 2026-10-11).** A weapon
takes one hand or two (`hands`, stated or from its baseline). The hands
hold three things between them: the main weapon (`wield`), a shield,
and a second weapon (`second`, its own command, so `wear` and `wield`
always mean the main hand). A two-handed weapon leaves no hand for a
shield or a second weapon; a second weapon is one-handed and takes the
hand a shield would. So the three fighting styles exclude each other by
the hands alone, with no feat field for it: sword and shield, two
weapons, or one great weapon. Held items (an orb, a tome) are not in the
hand count, so a caster keeps a staff and an orb. Versatile weapons (one
or two hands, better with two) are left for later.

*Two weapons.* The second weapon fills its own swing meter at its own
speed times Dexterity, so it adds one attack a round with a standard
weapon, and the hands take turns. It carries its own numbers, verb, and
family traits; its effects, like any worn item's, apply to the wearer.
Untrained, it is a strong penalty: every main-hand swing hits for 0.6
of itself and every off-hand swing for 0.4 (`P.dualMain`, `P.dualOff`),
so the pair deals about what one weapon does and the shield is gone;
with Second Attack the pair deals less than one sword. The Dual Wield
feat removes the penalty entirely, and Two-Weapon Fighting and Greater
Two-Weapon Fighting each add a whole swing a round to the off hand, as
Second and Third Attack do to the main hand. Removing the main weapon
moves the second one into the main hand.

*Two-handers.* Great Weapon Fighting, Great Weapon Mastery, and
Devastating Blows each add 20 percent damage to a two-handed weapon's
swings (additive with other damage bonuses), so the line pays for the
shield the way the dual line does, in damage per swing rather than
swings.

Measured on 2026-10-11 against an even fighter in the standard kit,
with the extra-attack feats the level allows, damage per round (after
extra attacks became a weapon's worth of swings, above):

| Level | Sword, dagger, or maul | Two swords, untrained | Two weapons, all dual feats | Maul, all Great Weapon feats |
|---|---|---|---|---|
| 5 | 16 | 13 | 24 | 19 |
| 10 | 28 | 22 | 55 | 39 |
| 20 | 164 | 114 | 286 | 262 |

A sword, a dagger, and a maul deal the same with the same feats, and two
daggers the same as two swords. The styles differ by what they give up:
sword and shield keeps the shield's block and defense, two weapons trade
it for the most damage, and a two-hander for a little less than that
with a single weapon to find.

Consequences: `weapon.speed` on the item and in the view,
default 1. `derivedStats` returns `attacksPerRound` as the total speed
and the engine's combat loop keeps the meter, or the rules keep it in
effect state; the leaning is the engine, since the meter is a lifetime
concern like an effect's rounds. Dexterity multiplies speed. Feats
`second attack` and `third attack` are `prepare`-phase effects adding
one swing each. As built, `derivedStats` returns `speed` and
`offSpeed`, the engine keeps a meter for each hand, and
`resolveAttack` receives the hand (`wield` or `second`) as a fifth
argument.

### 4.5 Pacing targets

**Status: leaning (2026-09-23).** Numbers exist only to hit these. Set
them first, then let the simulator tell you whether a formula does. The
values below follow from 2.3 and 2.5 and are the first thing `simulate`
should be checked against.

| Target | Value |
|---|---|
| Rounds for an even fight (equal level, standard gear) | 8 to 10 (24 to 30 seconds at 3 s rounds) |
| Health remaining after an even fight at full health | 50 to 70 percent |
| Win rate at +1 level, +3 levels, +5 levels | 80, 40, under 10 percent |
| Time to recover from a fight to full, asleep | about 30 seconds (10 rounds at 3 s) |
| Fights per outing before returning to town | about 10 |
| Even fights chained with no rest before death is likely | 3 to 4 |
| Mana spent per fight when magic is used freely, against the pool (6.2) | to be set |
| Standard deviation of health remaining after an even fight (this row sizes avoidance, 4.3) | under 8 points |

Measured 2026-09-24 with the 3.3 starting values, geometric baselines
at 1.10, and `kGrowth` 1.0, using `simulate fighter:N` against the
balance dummies, 500 seeded fights per cell. Even fights:

| Fighter level | Rounds | Health left | sd |
|---|---|---|---|
| 1 | 10.4 | 57.8 percent | 5.3 |
| 10 | 10.5 | 57.3 | 4.6 |
| 20 | 10.9 | 57.2 | 4.6 |

Flat across levels, in band on health and variance, a touch long on
rounds. Win rates by level gap (from the run at `kGrowth` 0.5, which
differs only in a slow drift):

| Fighter level | +1 | +3 | +5 |
|---|---|---|---|
| target | 80 | 40 | under 10 |
| 1 | 100 | 77 | 0 |
| 5 | 100 | 79 | 0 |
| 10 | 100 | 72 | 0 |
| 15 | 100 | 60 | 0 |
| 20 | 100 | 49 | 0 |

The shape is now the same at every level, which is what the geometric
curve was for. The row sits easier than the target: +1 is a sure win
and +3 is favourable. Two knobs move it, and they are independent:
`levelGrowth` steepens the gap (1.15 would bring +3 toward 40 and make
+1 less than sure), and `mobDamage` (8) raises difficulty across the
whole row without changing its shape. Neither has been moved; the
designer asked for 1.10 as the starting point.

**Re-measured 2026-10-12, weapons on the flatter curve (5.2).**
`simulate fighter:N` now wears the kit with the effects a character
can expect at its level (`standardKit`: one full-quality roll of added
fire on the sword from item level 5, 5.3), and `simulate plain:N` the
same kit without them. Dummies above level 25 were made at the same
baseline for the run; 500 seeded fights per cell. +1 is 100 and +5 is
0 in every kit row.

| Level | Kit rounds | Health left | sd | +3 win | Plain rounds | Plain health left | Plain even win |
|---|---|---|---|---|---|---|---|
| 1 | 10.3 | 58 percent | 5.0 | 71 | 10.3 | 58 percent | 100 |
| 5 | 10.4 | 58 | 4.7 | 75 | 11.6 | 52 | 100 |
| 10 | 10.5 | 58 | 4.5 | 77 | 13.5 | 44 | 100 |
| 20 | 10.9 | 57 | 4.6 | 78 | 18.0 | 26 | 100 |
| 30 | 11.2 | 57 | 4.4 | 80 | 23.5 | 6 | 58 |
| 40 | 11.4 | 57 | 3.9 | 82 | 24.8 | | 0 |
| 50 | 11.7 | 57 | 4.3 | 74 | 25.3 | | 0 |
| 60 | 12.0 | 57 | 4.3 | 75 | 25.9 | | 0 |

The kit matches the old plain kit on the old curve within noise at
every level (measured the same day before the change: 10.3 to 11.9
rounds, 57 to 58 percent, +3 at 70 to 84), which is what sizing a
full-quality affix to the curve's gap was for. Plain gear loses ground
from level 5 and stops winning even fights past about level 35: the
"stopgap" of 5.2 at its harshest.

**History.** The first measurement used linear baselines (`4 + 2L`,
`40 + 10L`, `L + 2`) and found +3 at 0 percent for a level-1 fighter
and 100 percent for a level-15 one: three levels was a 2x jump at
level 1 and a 1.2x jump at level 15. Geometric growth was chosen over
a per-band target row on 2026-09-24. Two consequences followed: damage
and defense became decimals in the engine, because whole-number
rounding at level 1 (a weapon doing 6 against one doing 8) was
distorting the low-level cells; and `K` had to grow with the attacker's
level, because a fixed `K` against geometric defense made even fights
lengthen from 10 rounds at level 1 to 20 at level 20.

### 4.6 Death

**Status: decided 2026-09-24, with 2.3.** The cost of death:

- **Experience: 20 percent of the character's total, capped at half
  the current level's cost, floored at the current level's threshold.**
  Two parameters in the rules scripts, `deathXpFraction` (0.20) and
  `deathXpLevelCap` (0.50), both starting values for playtesting. A
  death never crosses a level boundary downward. Under the 7.1 curve
  the 20 percent alone is a rising share of the current level, a tenth
  at level 2, half at about level 9, and the whole level by 30; the cap
  stops that climb at half, so from level 9 on every death costs the
  same half a level of progress and never more. Death gets heavier
  with level up to a point, then holds, and never takes a level.
- **Respawn in town** at low health, at the recall room of the area's
  town or the world's start room.
- **A corpse in the room** holding the inventory and equipment. It
  persists long enough to walk back for it; the duration is a
  starting value for playtesting, on the order of the 2.5 outing, so
  twenty minutes. Whether mobs can loot a corpse is open and leans no.

Consequences: the engine's death path applies the experience rule
through `xpToLevel` (the floor is the total for the current level) and
creates the corpse. The two experience parameters and the corpse
duration live in the rules scripts as named constants, read by the
engine through a small `deathRules()` hook, so they can be tuned
without a rebuild.

### 4.7 Groups, and combat between anyone

**Status: decided 2026-09-24.** Combat is not a mob-to-player
relationship. Any character, player or mob, can attack any other.
Several attackers can fight one target; one attacker can fight several
targets over a fight; players can form groups, and groups can attack
others, players included.

- **Groups.** A player can join a group (ROM's `follow` and `group`
  vocabulary, D7). A group shares experience from kills, split so that
  the group is never worse off per member than a soloist would be on
  the same kill (2.4: group-accelerated, never group-penalised).
  Members can `assist` and can be set to assist automatically. Group
  talk exists.
- **Many on one.** Any number of attackers can fight one target; the
  target auto-attacks one of them and may switch. Whoever is attacked
  fights back without a command: when its target dies, flees, or is
  otherwise gone, it turns on the next attacker still on it, one at a
  time, until none is left. Mobs can gang up, and a mob may assist
  another of its kind or its group (a builder flag).
- **One on many.** An attacker's auto-attack has one target at a time,
  but `kill <other>` switches it mid-fight, and skills and spells may
  affect several or all enemies in the room. Everything hostile to a
  character in a fight is that character's *enemies*; area effects are
  defined over that set, not over the room.
- **Players versus players.** Allowed by the rules; the world decides
  where (a safe-room flag) and the consequences are the same as any
  death (4.6). Whether the starting area is safe is content.

Why: the fight model has to be general before magic, because area
spells, wards on allies, and a healer in a group all need "my group",
"my enemies", and "everyone fighting me" to be real sets, not
inferences from a single pointer.

Consequences for the engine: each character keeps its current target
and the set of characters fighting it; `kill` while fighting switches
target; a `Group` with a leader and members, persisted only for the
session; experience split on kill; `follow`, `group`, `assist`, `gtell`.
Views gain `group: [...]` and `enemies: [...]` so that hooks can see
both sets. `resolveCast` takes a target *set*, not a single target.

### 4.8 Damage types and resistance

**Status: decided 2026-10-12.** Asked for by the designer alongside the
weapon curve (5.2): once high damage is effects stacking, something has
to make a fight ask which effects, and immunities are the oldest answer.

**Thesis.** Every hit has a kind, and creatures can be immune to kinds.
A skeleton cannot be stabbed, a fire elemental cannot be burned. The
player learns the world's rules and brings the right weapon.

**Antithesis.** Binary immunity is a wall. A player who arrives with
the wrong weapon does nothing at all, which is a lookup failure rather
than a decision, and a player who stacks immunities stops taking a
kind of damage forever. Applied to physical kinds it can make whole
areas unplayable for a build.

**Synthesis.** Resistance is graded, and immunity is its rare top.

- **Kinds.** Every part of every hit has a kind, from one list:
  physical (slash, pierce, blunt, bite, claw), elemental (fire, cold,
  lightning, acid, poison), and magic, psionic, drain, holy, and
  unholy. A weapon's swing has its weapon's `kind`, which content
  mostly states; a weapon that states none takes its family's (a
  dagger pierces, a mace is blunt), and a natural attack is blunt. A
  spell has a `kind` (magic when it names none), and so do procs,
  auras, and thorns (magic, and pierce for thorns, when they name
  none), added damage (5.3), and the damage over time they leave
  behind (ignite is fire, venom is poison, a spell's burn its spell's).
  A skill's damage is its own kind, its weapon's for a weapon strike,
  or blunt. Physical kinds and elements are both resistable.
- **Families.** A resistance may name a kind or a family: `physical`
  (slash, pierce, blunt, bite, claw), `elemental` (fire, cold,
  lightning, poison, acid), or `all`, so a ghost can shrug off weapons
  without a line per kind.
- **One effect.** `resist {kind, amount}`. Amounts add across sources,
  as `stat` does, and a resistance to a kind adds to one to its family
  and to `all`. A part of a hit of that kind is multiplied by `1 -
  total`: 0.5 halves it, 1 is immunity, and a negative total is a
  vulnerability (-0.5 makes it hit 1.5 times as hard).
- **Immunity is rare.** A player's total resistance to a kind stops at
  75 percent (`LIMITS.resist`, beside leech's), so gear and feats give
  partial resistance and a player always takes some of every kind.
  Full immunity is for mobs, and a builder writes it on named creatures
  and bosses, not on ordinary ones.
- **Where it applies.** Last: after the avoidance tries and armor for a
  swing (4.3), after the save for a spell (6.4), as a burn ticks, and
  for an aura on each enemy it reaches. A hit made of several kinds (a
  fire sword's slash and its flame) is resisted part by part, so one
  immunity takes one part: a swing wholly resisted still lands, and
  what rides on it is weighed on its own kind. A damage spell wholly
  resisted is shrugged off when it does nothing else; one that also
  curses or burns still lands its effects.
- **Seen before the fight.** `consider` names a creature's immunities
  and vulnerabilities, an identified item lists its resistances, and
  the score sheet totals a character's own.
  When a part of a hit is wholly resisted, the message says so ("The
  skeleton shrugs off your stab").

Why graded: the designer wants high damage to come from stacked
effects; graded resistance makes the mix of kinds in that stack matter
in every fight, not only against the few immune creatures, and a
player with the wrong kit is slowed, not stopped.

Consequences (built 2026-10-12): `resolveAttack`, `resolveCast`,
`useSkill`, and the proc, aura, and damage-over-time paths apply
`resist` by kind; a swing, proc, or skill wholly resisted comes back
`resisted` and the engine narrates it as shrugged off; auras, which the
engine lands on each enemy itself, go through an optional
`resistDamage` hook. The kinds in content are checked against the
rules' `damageKinds` by `urth check` (the one `blast` became
lightning), and every proc, aura, and thorns effect in content names
its kind. Open: whether anything lowers a target's resistance (an
armor-pierce for kinds, or a curse).

---

## 5. Items

### 5.1 Definition fields

**Status: leaning, revised 2026-09-24.** An item has exactly one
*primary* property, set by its type, and everything else about it is an
effect (5.4).

- Weapon: `damage` and `spread` (4.1) and `speed` (4.4), plus `hands`,
  `kind` (damage type), and `verb` (decided 2026-09-24): the word the
  combat message uses for a swing with this weapon, so a sword's
  "slash" reads "Your slash hits the guard for 12 damage" and an
  unarmed swing uses "punch". A mob's natural attack carries its own
  verb ("bite", "claw"). `resolveAttack` returns the verb; the
  transport renders the perspective forms (D10).
- Armor: `defense`, plus `slot`.
- Material (6.2): the schools it feeds, and a quantity if it stacks.
- Totem (6.1): the school it unlocks. Consumed on use.
- All: `weight`, `value`, `flags`, and an `effects` list.

Anything that would once have been a stat modifier, a proc, a
resistance, or a special property is an entry in `effects`. There is no
other place for it. Items carry a `level` and a `baseline` (5.2); the
level is not a requirement to use the item (5.2, leaning no).

### 5.2 Power budget: baselines by level, overridable

**Thesis.** A hand-written table: for each level, the damage a weapon
does and the defense a piece of armor gives. Builders read the row and
copy the numbers into the item.

**Antithesis.** A table has one column per number and no room for a
dagger and a maul to both be "level 10". Every item copies numbers
that go stale the moment the curve is tuned, and the simulator cannot
change the curve without a builder re-editing every file.

**Synthesis (decided 2026-09-24).** An item's level selects a
*baseline*, and there are many baselines, one per family of item. A
baseline is a formula in the rules scripts from level to the item's
primary numbers: for weapons `damage`, `spread`, and `speed`; for armor
`defense`. An item prototype names its level and its
baseline and gets those numbers computed; any number it states
explicitly overrides the baseline. Builders write `level: 10, baseline:
dagger` and nothing else for an ordinary item, and `damage: 40` beside
it for the one that is special. Tuning the curve is editing one
function; overrides are the exceptions a builder chose on purpose, and
`stat` shows both the computed and the stated value.

Starting baselines, arbitrary in the sense of 3.3 and chosen to hit
the 4.5 targets against the 3.3 health curve (an even fight at level
lasting eight to ten rounds with a naked Constitution of 10):

Every baseline is geometric in level (decided 2026-09-24, replacing
the linear curves measured in 4.5): its level-1 value times
`growth(level) = 1.10^(level - 1)`. Numbers are kept as decimals in
the engine so that rounding does not distort low levels.

| Baseline | Damage per swing | Speed | Spread | Notes |
|---|---|---|---|---|
| `standard` (sword, spear, mace) | `6 * growth` | 1.0 | 0.2 | the reference curve |
| `dagger` | `3 * growth` | 2.0 | 0.1 | same damage per round, steadier |
| `heavy` (maul, greataxe) | `12 * growth` | 0.5 | 0.4 | same damage per round, swingier; two hands |
| `unarmed` | `1.5 * growth` | 1.0 | 0.2 | what a player with nothing wielded does; verb `punch` |
| `axe` | `7.2 * growth` | 0.8 | 0.35 | +5 percent crit chance |
| `mace` | `6.6 * growth` | 0.9 | 0.15 | ignores a quarter of the defender's armor |
| `spear` | `5.4 * growth` | 1.1 | 0.15 | reach: +3 percent dodge while wielded |
| `flail` | `8.4 * growth` | 0.7 | 0.5 | goes over shields: shield block does not count against it |
| `whip` | `2.8 * growth` | 2.1 | 0.2 | one hit in ten slows the target to 75 percent for two rounds |
| `staff` | `5.4 * growth` | 1.0 | 0.1 | two hands by default; +10 percent spell power |

The families added 2026-09-28 (axe to staff) each come to about the
standard's damage per round; what they add is a trait, kept in the
rules' `BASELINE_TRAITS` and read from the baseline an item names.

**Weapons ride a flatter curve than everything else (decided and
built 2026-10-12).** Health, armor, mob attacks, spells, procs, auras, and
soak all grow by 1.10 a level, and so did weapons, so a plain level-60
sword hit for 1,660 a swing before Strength. The numbers were in
proportion, which is what holds an even fight at 8 to 10 rounds at
every level (4.5), but they put the whole of a character's power in the
weapon's one number, and left the effects on it (5.3, 5.4) as small
change on top. The designer asked for the opposite: high damage should
be effects, affixes, procs, and spells stacking, not a weapon with an
innate range in the thousands. Health is left as it is.

So the curve is split in two:

- The *power unit* is the standard weapon's old damage, `6 * 1.10^(L
  - 1)`. It sizes everything that is not a weapon's own swing: health
  (3.3), mob natural attacks (8), procs, auras, soak, skills, spells
  (6.5), and added damage (5.3). In the rules it is `unitAt(level)`,
  and `unit(c)` at a character's level.
- A weapon's own damage grows by 1.07 a level instead, so its share of
  the power unit falls: all of it at level 1, about 60 percent at 20, a
  third at 40, a fifth at 60. The table above keeps its level-1 values
  and family ratios; only the growth differs.

The rest is meant to come from what the weapon carries and what the
character has built: added damage and procs on the weapon (5.3),
feats (7.2), stats, and spells. Plain gear is therefore weak late, by
design: past level 40 a common weapon is a stopgap, and a rare or named
one is what carries a fight. Level 1 is untouched, because no affix
rolls that early.

Consequences: 4.5's targets are now measured with a fighter whose
weapon carries the effects a character can expect at its level (the
tier a drop at that level most often gives, with typical affixes), not
a plain one; the kit is a rules table and the simulator reports both.
An armed mob (8) keeps power-unit damage: its weapon sets the swing's
kind, verb, speed, spread, and effects, and the weapon's damage counts
against a plain weapon of the mob's level, so a weapon above its level
still makes it stronger. Named weapons (5.3) get their strength from
more effects and stronger procs, not stated damage.

| Baseline | Total `D` at level, across all slots | Spread | Notes |
|---|---|---|---|
| `medium` | `3 * growth` | 0.2 | the reference: 13 percent reduction against an attacker of the same level, at every level |
| `light` | 0.7 of medium | 0.1 | carries an intrinsic effect raising dodge |
| `heavy` | 1.3 of medium | 0.3 | carries an intrinsic effect lowering dodge |
| `cloth` | 0.4 of medium | 0.05 | a full set gives +5 percent dodge (each piece its slot's share) |
| `plate` | 1.6 of medium | 0.35 | a full set gives -5 percent dodge |
| `shield` | 0.08 of medium, per piece | 0.2 | a 10 percent block when the shield states none of its own |
| `tower` | 0.15 of medium, per piece | 0.2 | an 18 percent block and -3 percent dodge |

A set's total `D` is split across slots by fixed weights that sum to
one (revised 2026-09-25): body 30 percent, legs 15, head 10, arms 10,
feet 8, shoulders 8, hands 7, face 4, belt 4, each wrist 2. A single
piece's number follows from its slot, and a full set at level N is the
baseline `D` at level N. The positions on a character: light, head,
face, neck, shoulders, body, arms, hands, two wrists, belt, legs, feet,
two fingers, shield, wielded, held. Neck, fingers, shield, light, and
held carry no baseline defense; they are for effects, or for stated
numbers. An item names a family (`wrist`, `finger`) and goes on the
first free one.

Stat effects on gear stack with base stats *before* the multiplier is
computed (follows from 3.1 and 5.4): an effect that gives +2 Strength
raises the stat, and the multiplier is derived from the raised stat. A
flat bonus to damage is not a thing an effect can give.

Level requirement: **leaning no.** An item's level is the row of the
curve it was drawn from, not a gate. The twinking concern from 2.2 is
bounded by where gear comes from (8: mobs wear what they drop, so a
level-N mob drops level-N gear), and the designer has not asked for a
restriction.

Consequences: a new hook, `itemBaseline(prototype)`, called by the
engine when content loads and again when scripts reload, returning the
primary numbers; explicit fields on the prototype win. Prototypes gain
`level` and `baseline` fields. The engine stores the results and never
computes them (D15, D16).

### 5.3 Rarity and drops

**Thesis.** No randomness in loot. Section 8 already makes a guaranteed
drop an equipped item, visible on `look` before the fight, and every
named item is a builder's prototype with intrinsic effects. What you see
is what you get, and the world's hints (6.1) can point at a specific
reward.

**Antithesis.** Then every guard drops the same spear forever. Once a
player has seen an area's gear, killing there again gives coin and
experience and nothing to hope for, and an area is finished the day it
is explored. The chance that *this* kill is the good one is the oldest
reason to fight one more mob, and a fixed table cannot provide it. A
graphical or companion front end built on these rules (discussed
2026-10-11) leans on that pull even harder than the text game does.

**Synthesis (leaning, 2026-10-11; revised the same day).** Keep both.
Fixed items stay fixed, and ordinary gear can come out better than its
prototype. Rarity is a *count of effects on the instance*, as 5.4
asked: an affix is an effect (5.4) rolled onto an instance, stored in
the instance's applied effects the same way an enchantment is, so
nothing new is persisted but the tier's name. Once randomness is in,
everything that rolls is gated by the item's level, so a level-2 rat
cannot drop the best sword in the game.

- **Item level.** Every rolled instance has an item level: the
  victim's level for a drop, the higher of the reset's level and the
  prototype's for a reset item, the recipe's level for a craft (5.5).
  Item level decides which tiers can roll, which affixes can roll, and
  how strong they are. It is not a level requirement (5.2).
- **Tiers.** A tier is a range of effect counts, and the highest tier
  is open-ended:

  | Tier | Effects | Rolls from item level |
  |---|---|---|
  | common | none | 1 |
  | uncommon | 1 to 2 | 1 |
  | rare | 3 to 4 | 5 |
  | epic | 5 to 6 | 15 |
  | legendary | 7 to 8 | 30 |
  | relic | 9 or more | 40 (a roll gives 9 or 10) |

  A tier above an item's level is removed from the table before the
  draw, and the rest renormalized. The tier is the count however the
  effects got there: an uncommon sword enchanted (5.6) to a third
  effect is rare. A roll reaches ten at most; only enchanting goes
  past, at the rising risk 5.6 puts on it, so the top of relic is
  where the gamble lives. The tier is shown in the item's color and
  on `look`.
- **Named items and artifacts are not tiers.** A prototype with
  intrinsic effects is a builder's named item: never rolled, shown
  without a tier, exactly as written, which keeps everything the world
  hints at exact. Its name has a color of its own, apart from every
  tier's, so it stands out from plain gear as a rolled item does. A
  material (6.2) is shown in the color of the tier its rarity names,
  and a deterministic one in the named color. Artifacts are named items of a further kind, kept out
  of loot altogether and designed separately (Trackstar); the word
  "unique" is not used for a tier.
- **Names.** Every affix lends a name both a prefix word and a suffix
  phrase, and any affix may supply either; there is no split of
  affixes into prefixes and suffixes and no limit per side, as there is
  in Diablo. The strongest roll names the suffix and the next strongest
  the prefix: "a merciless rusty sword of the bear". Both sets of words
  work as keywords.
- **What can roll.** Weapons and armor with a vnum and no intrinsic
  effects, and not flagged `quest`, `newbie`, or `norandom`.
- **When.** At creation by a reset, which covers the gear a mob
  spawns wearing (still visible before the fight, so 8 holds) and
  gear lying in rooms; at a mob's death, as an extra *drop*; and on a
  successful craft (5.5). Merchants and quest vendors sell plain items;
  what they sell is the prototype.
- **Affixes are gated by item level and by slot.** Each affix names
  the lowest item level it rolls at and the slots it goes on, so early
  gear reads simply ("mighty", "sturdy") and the procs and keystones
  arrive as a character goes deeper. An enchantment (5.6) obeys the
  same gates. Starting gates:

  | From item level | Affix (slots) |
  |---|---|
  | 1 | the five stats (any); damage, honed (weapon); defense (any armor); reinforced (armor slots with baseline defense) |
  | 5 | crit (weapon, hands, finger); dodge (feet, legs, body); protect (body, shield, head, shoulders, arms); steady (any) |
  | 10 | leech (weapon, neck, finger); ignite, chill, venom (weapon); thorns (body, shield); soak (body, shield, belt, legs) |
  | 15 | crit damage (weapon, neck, finger); opportunist (weapon); shield block (shield); regen (neck, finger, body, belt) |
  | 20 | extra swings (weapon, hands); thunder, execute (weapon); brambles (body, shield); last stand (body, neck) |
  | 30 | shorter cooldowns (any); true sight and hawk sight (head, face) |

  `hold` and `light` take only the affixes open to any slot. Where an
  item's open pool is smaller than its count, affixes repeat and
  stack, as before.
- **Magnitudes.** An affix is a starting value scaled by a quality
  drawn from its tier's band, the higher tiers drawing from a better
  band. Stats and defense also grow with item level, as the baselines
  do (5.2); ratios (crit chance, dodge, leech, and the like) do not,
  since a share that grew with level would pass any cap in time.
  Affixes are existing effect kinds, so every one already works in the
  pipeline and `describeEffect` already words it.
- **Honed and reinforced (2026-10-12).** Two affixes raise the item's
  own number rather than the wearer's: honed (of the whetstone) adds a
  tenth of a weapon's own damage, reinforced (of the anvil) a tenth of
  a piece's own defense, each at least 1, and each roll adds its own
  tenth of the unraised number, so two make a fifth. The tenth is fixed,
  not scaled by quality: what makes one honed sword better than another
  is the sword. Reinforced goes only on the slots that carry baseline
  defense (5.2), since a tenth of nothing would be the floor alone. The
  floor is what makes them matter early, where a tenth is under a
  point; late, where a weapon's own damage is a small share of the
  power unit, honed is the weakest weapon affix, and is meant to be:
  it is the plain craftsman's improvement, beside cruel, which raises
  the whole hit with its added damage. An attuned item's raised number
  grows with its wielder as the rest of it does.
- **Added damage (decided 2026-10-12, built the same day).** Since a
  weapon's own damage falls behind the power unit with level (5.2),
  most of a late weapon's damage is affixes. The `added {kind, power}`
  effect adds `power` units of damage of a kind (4.8) to every hit. It
  is part of the hit: what scales the swing (Strength, level, damage
  effects, a crit) scales it, and armor, soak, and protection take from
  the whole hit, so armor keeps its meaning however a weapon's damage
  is made up. Only resistance weighs the parts apart, each by its own
  kind, so a weapon with three kinds of added damage loses one part,
  not the whole hit, to a creature immune to one of them, and a
  creature immune to the blade still takes its fire. (Built first past
  armor, as a proc; the 4.5 re-measure showed that made a weapon whole
  plus a quarter late, where armor takes most, so it was moved inside
  the hit the same day.) Added damage on a weapon rides only that
  weapon's swings; on anything else (a ring, a spell, a mob's own) it
  rides every swing. Four weapon affixes roll it from item level 5,
  weight 2 each: fire (flaming, of flame), cold (icy, of ice),
  lightning (sparking, of sparks), and poison (noxious, of the asp).
  A full-quality roll is `1 - weaponShare(L)` units, exactly what the
  flatter curve leaves a weapon of item level `L` short of the power
  unit, so one such affix makes a plain weapon whole again: a tenth of
  a unit at level 5, two fifths at 20, four fifths at 60. Quality
  scales it as any roll, so an epic's or relic's can exceed it.
- **Effects on gear are sized at the item's level.** Every effect
  whose power is in units (proc, added, ignite, envenom, aura) is
  sized at the level of the item it is on, not its wearer's, so old
  gear falls behind as the weapon itself does. A rolled affix records
  the level it was rolled at (`level` in its params), which may be
  above the prototype's when a deeper reset or drop rolled it; an
  intrinsic effect uses the prototype's level. An attuned item's
  effects, and a character's own (a skill's venom, a mob's prototype
  effects), are sized at the character's level. Items rolled before
  this have no recorded level and use the prototype's.
- **Named weapons** are strong the same way as rolled ones: several
  affixes, several procs, or one large one, never a stated damage
  number. Every named weapon from item level 5 carries added damage
  of its own theme worth exactly what the flatter curve took from it,
  `1 - weaponShare(L)` units (split 60 and 40 between two kinds where
  it has two), so it stands where it stood against a character in the
  kit (4.5), and what it was named for (its procs, crits, stats) still
  sets it apart. The theme is the item's: its existing procs' kinds,
  then its lore (Frostmourne cold and drain, the Master Sword holy,
  Thor's axe lightning); a weapon that is merely very good, a fine
  sword or a gun, adds its own physical kind (2026-10-12).
- **Drops.** On a death the rules say how many extra items the corpse
  gets and how wide a level band they come from. The engine draws each
  from the rollable weapon and armor prototypes of the victim's own
  area within that band, so a drop fits where it fell, and rolls it.
  An area with no such prototype drops nothing extra, so every area
  carries plain gear up to the level of its strongest mobs; a
  prototype flagged `drop` is gear made for this, placed by no reset
  and found only in corpses.
- **Deeper is better, more than more.** The chance of a drop rises
  slowly with the victim's level, and the tier weights rise faster.
  A drop that is likelier to matter is worth more than a bag that
  fills faster.
- **Loot tables.** Section 8 makes a worn item a guaranteed drop and
  leaves what is not worn to here. A mob prototype may carry a `loot`
  list, each entry an item vnum and a chance; on a death the engine
  draws each entry once and puts what comes up in the corpse, before
  the extra drops above. This is how a boss drops its named blade one
  time in twenty, how a key or a quest token can be carried without
  being worn, and how deterministic materials (6.2) enter the world.
  A rollable item that comes from a table rolls as a drop; a named
  item comes out as written. A table drop is not visible before the
  fight, which is the point of it: a worn item is the reward you can
  plan for, a table entry the one you hope for. An area file may carry
  a `loot` list too, drawn for every mob of the area that dies, for
  the area's farmed materials.

Starting values, arbitrary in the sense of 3.3, as weights per thousand
at item level 1 and at 40, linear between and held past 40, before
tiers above the item level are removed:

| Tier | Reset, level 1 | Reset, 40 | Drop, level 1 | Drop, 40 |
|---|---|---|---|---|
| common | 750 | 600 | 450 | 250 |
| uncommon | 220 | 250 | 400 | 350 |
| rare | 30 | 100 | 150 | 230 |
| epic | 0 | 40 | 0 | 120 |
| legendary | 0 | 9 | 0 | 40 |
| relic | 0 | 1 | 0 | 10 |

A kill drops an extra item with a chance of 15 percent at level 1,
rising to 25 percent at 40 and held there, one level either side of
the victim's; a mob flagged `boss` always drops two. A craft rolls
from the drop column, shifted by the crafter's rating (5.5). These are
for playtest: the `loot` builder command samples the table so a change
can be seen before anyone farms it.

Consequences: an optional `rollItem(item, context)` hook returning
`{rarity, color, prefix, suffix, effects}`, with `context.source` one
of `reset`, `drop`, or `craft`; an optional `lootRules(victim)`
returning `{drops, levelBand}`; without them nothing rolls. Affixes
carry a minimum level and a slot list in the rules. The item instance
gains `rarity`, `color`, `prefix`, and `suffix`, saved in the player
file; the item view carries `rarity`, and an enchantment that changes
the count asks the rules for the tier again. Mob and area prototypes
gain a `loot` list of `{item, chance}`, checked at load like resets.
Listings stop folding a rolled item into a stack of its plain
prototype. Open: whether merchants should pay more for a higher tier.

### 5.4 Effects

**Thesis.** Items carry modifiers, as ROM's affects do: +2 strength,
+10 hit points, +5 damroll. Builders understand them, and they are a
flat list of numbers the engine can add up.

**Antithesis.** A list of flat modifiers is a bonus economy. Every item
becomes a bag of small numbers, the interesting properties (a chance to
crit, a burn on hit, a resistance) do not fit the list and grow a
second system beside it, and 3.1 already forbids flat additions. Worse,
buffs on characters, procs on weapons, and enchantments on gear end up
as three implementations of one idea.

**Synthesis (leaning, 2026-09-24, resized the same day).** One
vocabulary: the *effect*. An effect is not a modifier. It is a small
piece of behaviour with parameters, and what it can do is arbitrary:
raise a stat, roll the weapon's damage twice and keep both, reflect a
fraction of damage taken, cast a spell when the wearer is hit, add an
attack, change who a swing targets. Each effect is a named kind, and a
kind is *code in the rules scripts* that attaches handlers to points in
the combat and tick pipelines. The engine never interprets an effect;
it stores the kind and parameters blindly (D15), keeps the list on the
item or character, and manages lifetimes.

**Where the behaviour runs.** Inside the existing hooks. `resolveAttack`
as the scripts implement it is a pipeline with fixed phases, and every
effect on the attacker, the defender, and their equipment is offered
each phase in turn. A first cut of the phases:

| Phase | What an effect can do here | Example kinds |
|---|---|---|
| `prepare` | change attack count, target, or which weapon is used | extra attack, cleave |
| `roll` | replace or repeat the damage draw (4.1) | roll twice keep both, roll twice keep best |
| `modify` | scale or add to the rolled amount | `stat` multiplier, `crit` |
| `dodge` | change the defender's dodge try (4.3 stage 1) | light armor bonus, blind |
| `block` | change the defender's block try (4.3 stage 2) | shield, parry |
| `reduce` | change how `D` applies (4.3 stage 3) | armor pierce, ward, reflect |
| `onhit` | do something because damage landed | burn, poison, `lifesteal`, `onhit` spell |
| `after` | do something because the attack finished | on-kill triggers |

Phase order is fixed, which is what makes "roll twice then double" and
"double then roll twice" unambiguous: rolling is `roll`, doubling is
`modify`, and `roll` always comes first. Within a phase, effects run in
a stable order (attacker's, then attacker's equipment, then defender's,
then defender's equipment, each in list order) so the outcome is
reproducible in the simulator. `onTick` and `derivedStats` have their
own smaller phase lists.

**Reach is bounded by the hooks.** An effect can do anything a hook can
see and return, and nothing else. "When you enter a room" needs an
`onMove` hook that does not exist yet. "Summon a creature" needs a
return channel the engine acts on. Every such wish is a contract
widening, tracked and costed as its own story, not a feature of the effect
system. This is the bound that keeps "arbitrary" implementable: the
effect vocabulary grows by adding kinds in scripts, cheaply and
hot-reloaded; the *reach* grows by adding hooks in Go, deliberately.

**State.** Some effects need memory: charges left, stacks, a cooldown.
Scripts cannot keep state between calls, so each effect instance
carries a small `state` map beside its `params`. Hooks return updated
state and the engine persists it. A `params` map is the definition and
never changes; `state` is what the effect has done so far.

Effects have three sources and one lifetime axis:

| Source | Lives on | Lifetime | Example |
|---|---|---|---|
| Intrinsic | item prototype | permanent | a sword that rolls twice |
| Applied | item instance | permanent, or charges | an enchantment |
| Cast or triggered | character | timed in rounds, or charges | a buff, a poison |

The same kind means the same thing wherever it sits. A `stat` effect
on a worn ring and one from a spell both raise the stat before the
multiplier. A `crit` effect from a feat and one from a dagger are the
same code.

**Sizing.** This is the largest single piece of rules work in the
document, and it is script work, not engine work. The engine's share is
small and fixed: an `effects` list with `kind`, `params`, and `state` on
item instances and characters, persisted with the player file;
lifetimes counted in rounds; the two lists exposed in the views; a
return channel to attach, update, and remove effects. The scripts' share
is the pipeline, the ordering rule, and every kind, and it is
unbounded by design. Because it is scripts, it is hot-reloaded and the
simulator can run it, so kinds can be added one at a time against a
regression set of fights. The first target should be the
pipeline with two or three kinds, not a library.

**Open within 5.4:**

- Stacking: two effects of the same kind on the same character. Leaning
  is that stacking is a property of the kind (`stat` adds, `crit` takes
  the larger, `roll twice` does not stack), decided per kind in its
  code, not a global rule.
- A cap on effects per item: none, per 2.2. The count is the tier
  (5.3), and enchanting past a roll's ten is bounded by risk (5.6),
  not by a number.
- Feats (7.2) are permanent effects a character holds; no distinct
  system.
- Whether effects can be conditional on the target (only against
  undead, only when below half health). Cheap in the pipeline; the
  question is whether the views carry enough to test the condition.

### 5.5 Crafting

**Thesis.** Crafting as most games do it: a recipe lists materials, you
have them, you press the button, you get the item. Reliable, and a
sink for the material pool (6.2).

**Antithesis.** A guaranteed craft makes materials into a currency and
crafting into a shop with extra steps. Nothing is risked, so nothing is
felt, and every crafted item is identical to every other. That is the
opposite of what discovery (6.1) and effectiveness (7.4) do elsewhere:
there, the world hands out possibilities and the character's skill
decides what comes of them.

**Synthesis (decided 2026-09-25, design only).** Crafting is an
*attempt*. A recipe names the materials it consumes and the item it
makes; the attempt consumes the materials whether it succeeds or not,
and it can fail. Crafting is a skill (7.4) with a rating: the rating
sets the chance of success and, on success, how good the result is.
The pieces, tied to what exists:

- **Materials** come from the one pool of 6.2. A recipe is a list of
  `{material, count}` like a spell's cost, and the same farming, rarity,
  and deterministic drops feed it. A rare recipe is one that wants a
  rare material.
- **Recipes** are content. The basic ones come with the crafting skill
  when a trainer teaches it; better ones are *found*, as patterns or as
  the words of someone who knows, in the discovery spirit of 6.1. There
  is no recipe list a character can read that the world did not give
  them.
- **Stations.** A recipe may need a place: a forge, a loom, a still. A
  room flag, as temples are.
- **Skills**, one per craft: smithing, leatherwork, alchemy, and so on,
  each taught (7.4) and each rated. Which crafts exist is content.
- **Failure** consumes the materials and nothing else. That is the
  consequence, and with rare materials it is a real one. Note against
  4.2: success is a binary roll, admitted for the same reason saves are
  (6.4): it is rare, it is costed, and no fight turns on it.
- **Quality** (revised 2026-10-11). On success the item is the
  recipe's item at the recipe's level, resolved through the baseline
  like any other (5.2), and then rolled for a tier (5.3) with the
  recipe's level as its item level, so crafting can make every tier a
  drop can, under the same gates. The crafter's rating shifts the
  weights toward the higher tiers, and the rarest material in the
  recipe sets how far they can shift: a common recipe in a master's
  hands makes good uncommon and rare things, and a relic wants a rare
  material as well as the hands. How much is a starting value for
  playtest. Crafting is where the roll is; enchanting (5.6) is where
  the player chooses.
- **Crafted items are marked** as such on the instance, for the record
  and for whatever later cares who made a thing.

Starting formula, in the shape the rest of the document uses:

    chance = rating / (rating + difficulty)

where `difficulty` is the recipe's number, in rating points. At a
rating equal to the difficulty, one attempt in two succeeds; the rating
never reaches certainty, and improves with each attempt as any skill
does (7.4), so failure is also practice.

### 5.6 Enchanting

**Thesis.** Any item can be enchanted, as many times as its owner can
afford; enchanting is how gear grows.

**Antithesis.** Then the best item in the game is whatever has been
enchanted most, and every item is a blank to be filled. Something has
to make the tenth enchantment rarer than the first, and something has
to make one sword worth enchanting and another not.

**Synthesis (decided 2026-09-25, revised the same day, design only;
slots removed 2026-10-11).** Any item can be enchanted, with no count
of slots and no cap. The limit is risk. An earlier version gave each
prototype a fixed number of enchantment slots, where rarity lived;
rarity now lives in the count of effects (5.3), and a second number
beside it would only say the same thing worse.

An enchantment is an attempt, like a craft: it names materials, it can
fail, and success adds one *permanent* effect (5.4), the applied-effect
storage the engine already persists. The effect counts toward the
item's tier (5.3), so enchanting is how a rare becomes an epic and how
a relic goes past the ten a roll can give; the effect must be open at
the item's level and slot under 5.3's gates. Failure consumes
the materials and does nothing, strips every affix from the item, or,
past twelve effects, destroys it. So enchanting a sword up to twelve
effects costs mana, materials, and patience, and the thirteenth onward
is a gamble with something irreplaceable, paid for in materials that
double with every step.

- **Enchantments are spells (decided 2026-10-12, built the same
  day).** An enchantment is an arcane spell of the enchantment school
  cast on an item in the caster's inventory (`cast 'enchant weapon'
  sword`), with the mana, cast time, and interruption of any spell
  (6.4). Each names exactly one thing it does, and the enchanter
  chooses which to cast. The Enchanting skill (7.4) is the rating
  behind all of them: the first enchantment cast gives it, at 20, and
  every cast improves it, never a fight. The first four, each 150 mana,
  which is a dedicated caster's whole pool around level 25:

  | Spell | From | Does |
  |---|---|---|
  | Enchant Weapon | 25, Int 20 | the weapon's own damage up 5 to 10 percent, at least 1 (the `baseDamage` kind honed rolls, 5.3) |
  | Enchant Armor | 25, Int 20 | a piece's own defense up 5 to 10 percent, at least 1 (`baseDefense`) |
  | Restore Item | 25, Int 20 | the item back to its prototype: every affix, rolled or enchanted, comes off, and its tier and name with them |
  | Permanency | 30, Int 22 | nothing can enchant or restore the item again |

  Each Enchant is another effect on the item, so it counts toward the
  tier and the next attempt's difficulty, and can be cast again and
  again. Permanency's mark is not an effect for either. A named item's
  intrinsic effects are the item itself: Restore leaves them, a strip
  leaves them, and a named item stays without a tier however it is
  enchanted. Difficulty (the `difficulty` below): 20 for the two
  Enchants, 15 for Restore, 40 for Permanency. Basic enchantments are
  innate to the school; better ones are found (6.1's discovery rule).
- **Every enchantment can fail, three ways (decided 2026-10-12).** A
  failure spends the mana and does one of: nothing; strips every
  affix, rolled and enchanted, leaving the item as its prototype; or
  destroys the item. Up to twelve effects, a failure strips one time
  in twenty and otherwise does nothing; past twelve, it destroys as
  the formula below says, strips one time in ten, and otherwise does
  nothing. Restore can fail too, so the cure for a strip is not free.
- **Difficulty rises with the count (decided 2026-10-12).** `n` is
  every effect already on the item, however it got there: rolled
  affixes, a named item's intrinsic effects, and earlier enchantments
  alike. It is the same count as the tier (5.3), so a dropped rare and
  an enchanted rare are equally hard to take further, and a relic is
  hard to enchant whether it dropped or was made. Counting only
  enchantments would make a dropped relic a safe blank.
- **Easy to twelve, hard past it (decided 2026-10-12).** Up to twelve
  effects, enchanting is a matter of materials and the rating: each
  effect on the item makes an attempt a little harder, a failure loses
  only the materials, and the cost does not grow. From the
  thirteenth, three things climb at once: success falls with the
  square of the effects past twelve, a failure may destroy the item,
  and the materials double with each step. Eighteen to twenty is
  possible and very expensive; past twenty nothing forbids it, and the
  curves make it a legend.
- **Tiers are gated by item level, for enchanting too (decided
  2026-10-12).** An enchantment cannot raise an item's count into a
  tier its item level could not roll (5.3): a level-2 dagger stops at
  two effects, a level-10 sword at four, a level-15 piece at six, a
  level-30 at eight, and only from item level 40 is the count open.
  Without this, ten stat affixes on a level-2 dagger, each a point of
  Strength, would be the best item a new character could carry, and
  the gate that keeps a rat from dropping it would be undone by an
  enchanter. The open-ended top stays where 5.3 puts it.
- **Powerful items are rare by construction.** An item has no
  ceiling but the risk and the cost, and both are steep exactly where
  items are worth most. Twelve chosen effects are a well-made item;
  twenty is a project, with spare items and a hoard of materials.
- **Materials** are the one pool. The effect decides the cost: a stat
  effect wants common things, a crit or attack effect wants rare ones,
  and the deterministic drops (5.3's loot tables) are what the last
  effects on the best items are for.
- **Crafted items** (5.5) enchant like any other; a crafted relic is
  a starting point no drop is likelier to give.

Starting formulas, arbitrary in the sense of 3.3, with `n` the effects
already on the item and `e` those past twelve (`max(0, n - 12)`):

    success              = rating / (rating + difficulty + 3n + 8e²)
    destroyed on failure = e / (e + 25)
    stripped on failure  = 1/20 up to twelve effects, 1/10 past
    materials            = the enchantment's own, times 2^e

(decided 2026-10-12, for playtest). For an enchantment of difficulty
20, the chance one attempt succeeds, and the chance a failure destroys
the item:

| Effects on the item | Novice (25) | Journeyman (50) | Master (100) | Destroyed on failure | Materials |
|---|---|---|---|---|---|
| 0 | 56% | 71% | 83% | none | 1x |
| 6 | 40% | 57% | 72% | none | 1x |
| 12 | 31% | 47% | 64% | none | 1x |
| 14 | 21% | 35% | 52% | 7% | 4x |
| 16 | 11% | 20% | 34% | 14% | 16x |
| 18 | 6% | 12% | 22% | 19% | 64x |
| 19 | 5% | 10% | 18% | 22% | 128x |

Compounded over the attempts it takes, from twelve effects:

| Goal | Journeyman (50) | Master (100) |
|---|---|---|
| 16 | 2 items in 3 reach it; about 55 enchantments' materials | 4 in 5; about 35 |
| 18 | 1 in 5; about 500 | 2 in 5; about 250 |
| 20 | 1 in 33; about 6,700 | 1 in 8; about 2,200 |

The materials are counted as the enchantment's base cost, spent per
success and including the attempts on items that were lost. So a
master reaches twenty with eight items of twelve effects and some two
thousand enchantments' worth of materials; a journeyman needs three
times that and thirty items, which is what "requires a master" means.
The levers, if playtest finds the top too easy or too hard: the
threshold (12), the square's weight (8), `R` (25), and the doubling.

Consequences for the engine: for enchanting (built 2026-10-12), an
`item` spell target, resolved by a `resolveEnchant(caster, item,
spell)` hook that returns the item's applied effects and naming after
the attempt, or that it was destroyed, with an `enchantBlocker` hook
asked before mana is spent; the item view carries `applied`, its own
effects apart from the prototype's. The first four spells cost mana
only; the materials and their doubling past twelve wait on a spell
whose materials the rules can size per cast. For crafting, a
`recipeList` with materials, difficulty, station, and result; a
`craft` command through a `resolveCraft` hook; a `crafted` mark on
instances, persisted; station flags on rooms;
`look` shows the effects and the tier; `describeEffect` already covers what
an enchantment does.

---

## 6. Magic

Worked dialectically on 2026-09-24. The shape is set by three
commitments from the designer: any character can potentially use magic,
each school of magic must be discovered and unlocked through play, and
casting consumes gathered materials. Magic is meant to be very strong
and to carry real costs. Everything below follows from those. The third
commitment changed on 2026-10-11: casting costs mana, and materials are
an extra cost only some spells carry (6.2).

### 6.1 Schools are discovered and unlocked by consuming a totem

**Thesis.** Spells are learned from trainers as a character levels, as
in ROM. Simple, predictable, and the level table is the whole design.

**Antithesis.** Then magic is a function of level. Every character of
level N has the same options, magic is another number going up, and the
world's content has nothing to do with what a character can do.

**Synthesis (leaning, revised 2026-09-24).** Magic has two branches,
as in Dungeons and Dragons, and each is unlocked by something found in
the world rather than by level:

- **Arcane.** Organised into the eight schools of Dungeons and Dragons
  (decided 2026-09-24): abjuration, conjuration, divination,
  enchantment, evocation, illusion, necromancy, transmutation. A school
  is unlocked by finding its *totem*, an item placed in the world, and
  consuming it. Scaled by Intelligence (3.2).
- **Divine.** Three deities, unnamed for now (decided 2026-09-24): a
  Supreme Good, a Supreme Neutral, and a Supreme Evil god, each with a
  domain of spells. Divine casting is unlocked by a *great sacrifice*:
  make or find an item the god wants, carry it to that god's temple,
  and give it up there. The character becomes an apostle of that god.
  The hints that lead to the item and the temple are content (6.1's
  discovery rule); the mechanism is one command at one place. Scaled by
  Wisdom (3.2). Leaning: one god at a time, and a new sacrifice to
  another god ends the old apostleship.

A character holds what it has unlocked permanently. Level does not
grant magic. Two characters of the same level can differ entirely in
what they can cast, based on where they have been and what they found.
Whether a god objects to arcane practice, and what an apostle owes
after the sacrifice, are open and are exactly the kind of consequence
6.3 wants magic to carry.

Discovery is the point, and it is done with content, not a quest
system. The world carries hints: room descriptions, what NPCs say, books,
the look of the totem itself. A player who reads and explores finds
schools; one who does not, does not. There is no quest log, no
objective marker, and no NPC that says "bring me five pelts" *for
magic*. The questmaster of 7.6 (added 2026-09-25) draws its tasks
from the live world at random and never points at a totem; quests are
one more way to place a hint, not the mechanism.

This makes magic a fourth pillar of power beside level, gear, and stats
(2.2): earned by content rather than by numbers, and uncapped like the
others.

Consequences: a `totem` item type carrying a school (5.1). The
character keeps a persistent set of unlocked schools and deities, saved
in the player file, that `resolveCast` reads. Consuming the totem is the
first use of the `consume` return (section 1). Divine access needs a
`sacrifice <item>` command that works only in a room flagged as a
god's temple, consumes the item through the `consume` return, and
records the deity beside the schools. Hints are a builder concern:
builder tooling should make it easy to see which totems exist and
where their hints are. No dependency on a quest system.

**Learning spells within a school (decided 2026-10-11).** The designer
asked for more spells, most of them trainable. Read against the
antithesis above, a trainer is exactly what 6.1 rejected: magic as a
function of level and coin. The answer is to keep the door and train
the rooms behind it. Unlocking a school or a god still decides
*whether* a character has that magic at all, and nothing else grants
it; within an unlocked school or god, a spell is learned in one of
three ways:

- **Innate.** A school's or god's first spells are known the moment it
  is unlocked, so a found totem is usable at once. The thirteen spells
  of 6.5's first list are innate, which also means no character loses
  a spell it could cast before this change.
- **Taught.** Most spells are sold by trainers placed in the world, for
  coin, to a character that has the school or god, the level, and the
  stat (6.4). A trainer teaches only what it knows, and the trainers
  for a school live where that school would be studied, so finding the
  teacher is a smaller discovery inside the large one.
- **Found.** The strongest spell of each school and god is taught by
  no one. It is learned from a scroll found in the world, by studying
  it (the scroll is used up), with the same requirements. This keeps
  the top of every school a project, as 6.2's deterministic materials
  once did.

A learned spell is a permanent effect on the character, as a learned
skill is (7.4), so it needs no new storage and survives death.

**Revised 2026-10-12: common spells, one specialty, and the Towers of
High Sorcery.** Measured in play (story 1022), the door above was
closed too tight. Four of the eight schools had no totem a player could
reach, the only school with a first-level attack sat behind a level-14
kill, and a caster at levels 1 to 5 was weaker than a fighter, because
its points went into a stat its locked magic could not use. Read
against the antithesis, the fault was putting *all* magic behind
discovery. A character can differ by what it has earned without being
unable to cast anything until it has. The designer's answer keeps
discovery for the top of the scale and opens the bottom:

- **Three tiers.** A *common* spell is simple enough that anyone with
  the level and the stat can learn it from a trainer, for coin. A
  *school* spell needs the caster to be that school's specialist. A
  *rare* spell is found on a scroll, as in the "found" way above. Every
  arcane spell still belongs to a school, common ones included, because
  the opposition rule below reads it. Requirements stack (scroll,
  level, stat, specialty, material), each set by how strong the spell
  is; a rare spell may want all of them.
- **Six schools in three Orders.** Divination and transmutation are
  cut. Divination was mostly self-buffs that read as common spells, and
  transmutation folds into enchantment, which now holds both mind and
  body. Each Order of High Sorcery holds two schools, and each school
  has an opposite in another Order:

  | Order | Tower | Schools |
  |---|---|---|
  | White | Daltigoth | abjuration, evocation |
  | Red | Wayreth | conjuration, illusion |
  | Black | Palanthas | necromancy, enchantment |

  Opposites: evocation and necromancy (damage: burst against drain);
  abjuration and conjuration (holding the field against filling it);
  enchantment and illusion (commanding the mind against deceiving it).
  The pairs are like against like on purpose, so that every ban costs a
  specialist about the same thing and no specialty is the free one.
- **One specialty, for good.** An arcane caster may specialise in
  exactly one school, and never changes it. The specialist gains a
  bonus to that school's spells and can no longer cast the opposite
  school's spells, common ones included. Everything else stays open:
  common spells of the four other schools, and any spell with no
  school. This answers 7.3's open question on a limit to schools: one.
- **Totems leave the world.** A specialty is earned at the tower of the
  school's Order, by passing that tower's Test of High Sorcery. The
  Test is content: a way into the tower that is hard to find and hard
  to survive, ending in a room where a character that meets the level
  and Intelligence swears to one of the Order's two schools. The towers
  lie past Solace and are not meant to be easy to reach, so the
  specialty is the discovery that 6.1 was always about, and common
  spells carry a caster until then. Totems survive only in the balance
  lab, or are replaced there by a builder's way to grant a specialty.
- **Divine stays simple.** There is no divine magic without a god.
  Becoming a god's apostle (the sacrifice above, unchanged) grants a set
  of common divine spells that every god's apostle shares, among them
  Mend, plus the god's own spells, taught, innate, or found. Being one
  god's apostle already excludes the other gods' spells, so divine needs
  no opposition rule; Devotion (6.6) is its specialist's bonus. A hidden
  Fallen God, whose disciples give up some divine spells for others, is
  an idea for later.
- **Existing characters are reset.** A character that brandished
  totems before this change loses its schools and specialises like
  everyone else.

Leaning, to be set when it is built: the specialist's bonus is +30
percent to its own school's spells (spellPower), above Spell Focus's
+20 percent to everything; a school spell, and the Test, want level 10
and 15 Intelligence. The tables in 6.5 still list the eight schools
until the spells are retagged. Consequences: each spell gains a `tier`
(common, school, rare) beside its `school`; the character's `Schools`
list becomes one `specialty`; `cast`, `practice`, and `spells` refuse
an opposite-school spell with the reason; a Test room carries its
Order, and a command there sets the specialty once.

### 6.2 Casting costs mana; some spells also want items

**Thesis.** Casting draws from a mana pool that regenerates. Cheap to
implement, and the engine already has `mana`, `manaMax`, and `manaDelta`.

**Antithesis.** A regenerating pool makes magic free at the margin. The
only question is whether the pool is full, and it always is at the start
of an outing. That cannot support "very strong with significant
consequences"; the consequence of casting is waiting.

**Synthesis (decided 2026-09-24).** Every cast consumes materials:
items gathered from the world and carried in inventory. Materials are
*not* bound to a school or a deity. There is one common pool, and each
material in it has a rarity. A spell names which materials it needs
and how many, from that pool, so a rare spell is one that wants a rare
material, and two schools may compete for the same one. Materials
come from *farming*: random drops and harvest points that yield from
the pool by rarity, so a player can grind for common ones; and some
are *deterministic*, dropped by one thing in the world, very rare or
very hard to reach, so that a particular spell is a project. Both
branches draw from the same pool; an offering is a material. The cost is paid before the
outing, in time spent gathering, and again at the moment of casting,
when the materials are gone. Strength is balanced by cost and access,
not by shrinking the effect. A character who has the category and the
materials should feel powerful; one who has spent the materials is back
to steel.

Mana is kept behind a flag, not removed (decided 2026-09-24). The engine
keeps `mana`, `manaMax`, and `manaDelta`; the rules set `manaMax` to
zero and no cast requires it, and the prompt hides a zero pool. If the
material system turns out not to work, mana is there to fall back to
without an engine change. Until then a second cost would dilute the
first, so it stays off.

Consequences: an item type for materials (5.1) that stacks by count.
Rarity is a field on the material's prototype and the drop tables in
5.3 draw by it. Harvest points are a room feature (a `forage` or
`gather` command at a flagged room, on a timer). The first pool and
list are proposed in 6.5. `resolveCast` lists the materials
it used in its `consume` return and the engine removes them (section 1).

**Revised 2026-10-11: mana, with items for some spells.** The designer
asked to drop the material requirement from spells in general and move
to mana, keeping items for the spells where an item is the point:
performing certain enchantments, opening portals, and the like. The
material synthesis above is kept as the record of what was tried.

Why, in the terms of this section: a material on every spell made
casting a logistics job. A character who found a school could not use
it until it had farmed the right things, and every cast was a small
inventory check, which reads poorly in text and worse on a phone. The
antithesis still holds, so the synthesis has to answer it: a pool that
is always full at the start of an outing makes magic free at the
margin. The answer is where mana comes back. It refills out of a fight,
over about the same time health does (2.5), and not at all during one,
so the decision 2.1 and 6.3 want is still there: a fight has a budget,
and an outing is a chain of budgets with rests between them. Casting in
an even fight spends what the next fight might need.

- **Mana.** Every spell has a mana cost, spent when the cast begins, so
  an interrupted cast loses it, as it lost materials (6.4). The pool is
  a base plus a little per level, times the better of the Intelligence
  and Wisdom multipliers, so either branch's stat makes a deeper pool.
  It refills from empty in 90 seconds standing out of a fight (30
  rounds of 3 seconds, `manaRegenSeconds`),
  faster resting and asleep at the same multipliers as health, and not
  at all in a fight (a parameter, `manaFightRegen`, if a trickle is
  wanted).
- **Items, for some spells.** A spell may still name materials, as an
  extra cost on top of its mana, consumed the same way. They are for
  spells whose effect is about an object or a place: an enchantment
  that wants a particular reagent, a portal that wants a focus, a
  summoning that wants a token. The first such spells do not exist
  yet; the engine path for them does.
- **The material pool** (6.5) stays. Its uses are now those spells,
  crafting (5.5), enchanting (5.6), skills taught for items (7.4),
  and offerings, rather than every cast.

Starting values, arbitrary in the sense of 3.3: a pool of 30 plus 3 a
level before the stat multiplier, and costs from 6 (Firebolt, Daze) to
20 (Sanctuary), so a level-1 caster with no points in either stat casts
about five cheap spells from full. Cost does not grow with level, so
early spells get cheaper relative to the pool as a character grows.
This supersedes the mana fallback in D18 and answers the question it
left open; see D23.

Consequences: `spellList` entries gain `mana`; `cast` refuses a spell
the caster cannot pay for and spends the mana with any materials when
the cast begins; `derivedStats` returns a real `manaMax` and `onTick`
a `manaDelta`; the prompt shows mana once the pool is above zero;
`spells` lists each cost. Materials keep their item type and drops.

### 6.3 Magic is very strong

**Thesis.** Magic sits at parity with weapons so that casters and
fighters are balanced against each other.

**Antithesis.** There are no casters and fighters (7.3); every character
can potentially cast. Parity would make the discovery and the cost a
tax on an effect you could have got from a sword. If magic costs more
than a swing it has to do more than a swing.

**Synthesis (leaning).** Per use, magic is the strongest tool in the
game. The mana cost (6.2, revised 2026-10-11: a pool that does not
refill in a fight) and discovered access are what balance it, so its
output is not held back to weapon levels. In terms of 2.1, casting is
the decision a fight offers: when to spend mana, on which fight. An
even fight at level is winnable with steel alone; magic is what makes a
fight above level winnable, and spending it on an even fight is a
choice to trade the next fight's mana for safety and speed now.

Consequences: the simulator must run fights with and without casting,
and report mana spent per fight against the pool, since that is the
cost being balanced. 4.5 gains a row for it.

### 6.4 Casting mechanics

Decided 2026-09-24 from the designer's answers; the syntheses below
record where a simplification was taken and why.

**Cast time.** A spell has a cast time in rounds, and it varies by
spell. The designer described three kinds: *instant* (outside the
round entirely), *zero-round* (inside the round, but able to happen
alongside another spell), and *N-round*. Thesis: model all three.
Antithesis: instant and zero-round both mean "costs no action and no
time"; they differ only in whether the spell resolves on the tick it is
typed or at the next round boundary, and no player will feel that
difference against a round of two or three seconds. Two concepts for one experience
is a cost with no return. **Synthesis (leaning):** one number,
`castRounds`. Zero means the spell resolves on the tick it is typed
and does not take the round's action (2.1), so it can be chained with
another spell or with an attack in the same round. One means it takes
this round's action and completes at the round boundary. N means N
rounds of casting. A caster has at most one spell in progress; starting
another cancels it. Free spells are limited by their mana cost and
by an optional per-spell cooldown, not by the round. If a real
difference between instant and zero-round appears in play, the doc
comes back here.

**Interruption.** Movement always interrupts a cast in progress. Damage
interrupts only spells that say so (`interruptOnDamage`). Leaning:
mana and any materials are committed when the cast begins, so an
interrupted cast loses them. That is the consequence 6.3 asks magic to carry, and it is
why a long cast is a real decision in a fight. Open: whether an
interrupted cast also triggers the cooldown.

**Early spells hold, and grow slowly (decided 2026-10-12).** Measured
in story 1022: the round resolves swings before casts, and a mob of the
caster's level lands nearly every round, so a solo caster at levels 1
to 5 lost about five casts in six to blows, and the mana with them, until
Combat Casting (level 6) or Steadfast Prayer (level 7) arrived. Below
those feats, magic in a solo fight was a tax rather than a tool. The
designer's answer: early spells are not easily broken, and in exchange
they do not scale as well as later ones, so they carry a beginner and
then give way. Built 2026-10-12 (`earlySpellLevel` in the rules): a
spell of level 5 or below ignores blows (movement still cancels it),
and its power unit grows on the weapon curve (1.07 a level, 5.2) rather
than the power unit's 1.10, so at level 20 it hits about 60 percent as
hard as it would on the full curve.

Measured as in story 1022 (kills per hour in even fights, resting to
full between them, against the balance dummies; the fighter takes no
feats, the casters their caster feats):

| Build | L1 | L3 | L5 | L7 | L10 |
|---|---|---|---|---|---|
| fighter, Strength, Constitution, Dexterity | 90 | 92 | 109 | 127 | 140 |
| arcane, all Intelligence: Firebolt | 116 | 147 | 206 | 251 | 282 |
| arcane: Magic Missile, then Firebolt | 117 | 144 | 198 | 200 | 232 |
| arcane, 2 Intelligence to 1 Constitution | 82 | 104 | 154 | 198 | 240 |
| arcane: Chill Touch, Drain | 99 | 138 | 164 | 197 | 241 |
| divine, Good: Wrath, Mend under half | 108 | 149 | 217 | 240 | 270 |
| divine, Evil: Rotting Touch, Dread, Blight, Wrath | 96 | 125 | 126 | 131 | 180 |
| divine, Neutral: Entangle, Stillness, Wrath | 109 | 124 | 116 | 129 | 170 |

Before the change the arcane rows began at 60 to 62 and stayed there
until level 6. Now no caster starts below a fighter, and the gap that
6.3 wants opens by level 5. Nobody died in the arcane or Good rows; the
Neutral row lost one run in thirty at level 7. The Evil and Neutral
rows lean on control spells, which the simple casting policy used here
spends mana on rather than on Wrath.

**Requirements (leaning, 2026-10-11; access revised 2026-10-12, 6.1).**
Since 2026-10-12 access is by tier: a common spell needs no specialty
(but is barred to the opposite school's specialist), a school spell its
specialty, a divine spell a god. The rest of this paragraph holds as
written. Unlocking a school or a god
(6.1) gives access to its spells, and learning one (innate, taught, or
found, 6.1) makes it the caster's; each spell also names a level and a
trained stat (Intelligence for arcane, Wisdom for divine) the caster
must have before it can be cast, so finding the evocation totem at
level one does not put Fireball in a beginner's hands. A spell that is
not ready yet still shows in `spells all`, with what it needs, so the
player knows what to work toward; bare `spells` keeps to what can be
cast or learned now. Like skills (7.4), requirements
read trained stats, not gear. The numbers are in 6.5's tables. A
trainer will not sell a spell whose requirements are unmet, and a
scroll cannot be studied before them, so a spell, once learned, is
always castable.

**Saves.** A target resists a spell by a saving throw in the Dungeons
and Dragons manner, and the spell names which: *reflex* (Dexterity),
*fortitude* (Constitution), or *will* (Wisdom). A successful save does
what the spell says: negates it, or halves it. Some spells allow no
save. Note against 4.2: a save is a binary roll, the kind of variance
4.2 keeps out of the weapon fight. It is admitted here on purpose,
because a spell is rare, costs mana, and is meant to be strong
(6.3); a fight is not decided by a single save the way it would be by
a hit roll on every swing. The simulator should report save rates.
Starting formula, asymptotic like everything else here:

    save score  S = target's save-stat multiplier * target's level multiplier
    spell power C = caster's spell-stat multiplier * caster's level multiplier
    chance to save = S / (S + 2 C)

which is one in three at parity and never reaches one.

**Scaling.** A spell's output scales by a combination the spell names:
the caster's level, one or more stats (Intelligence for arcane,
Wisdom for divine by default, 3.2), or both. There is no other
multiplier. Whether a spell also carries a use-based effectiveness
rating (7.4) is **open**; the leaning is no, since magic is discovered
rather than practised, and its growth comes from stats and level.

**Targets.** A spell is *single* (one target), *area* (the room), or,
left for later, *global* (the world). Area, for a hostile spell, means
every character in the room who is not the caster or in the caster's
group (4.7); for a helpful spell, the caster's group. Whether a hostile
area spell can catch allies (friendly fire) is open and leans no.
`resolveCast` receives the target set already built.

**Spell definition fields**, so the list can be written: `id`, `name`,
`branch` (arcane or divine), `school` or `deity`, `castRounds`,
`interruptOnDamage`, `cooldown`, `mana`, `materials` (kind and count,
for the few spells that want items),
`target` (single, area, self), `save` (reflex, fortitude, will, none)
and `saveEffect` (negate, half), `scaling` (stats and whether level),
and the effect: damage, healing, or effects to attach (5.4).

Consequences: the engine tracks one cast in progress per character with
rounds remaining, cancels it on movement, and calls a hook on damage to
ask whether the spell in progress is interruptible. `resolveCast` runs
at completion with the caster, the target set, and the spell, and
returns damage or healing per target, effects to attach, and materials
to consume. Cooldowns are counted in rounds on the character.

### 6.5 First pool and spell list (proposal, 2026-09-24)

A starting point to edit, sized so that every school and deity has one
spell and the engine has something to resolve. Damage is written as a
multiple of the power unit at the caster's level (5.2; the standard
weapon's swing before weapons took the flatter curve), before the
spell's stat and level scaling, so the list stays true when the curve
moves. Durations are rounds.

**Materials pool.** Rarity sets how often farming yields it; the last
tier is never random. Since 2026-10-11 no spell on the lists below
needs a material (6.2); the pool feeds item-bound spells, crafting,
enchanting, and trainers.

| Material | Rarity | Where |
|---|---|---|
| ash | common | any hearth or fire; drops from most mobs |
| salt | common | the coast, the market |
| tallow | common | animals |
| bone dust | uncommon | undead, graves |
| nightshade | uncommon | forest harvest points |
| quicksilver | uncommon | the smithy, alchemists |
| star iron | rare | one meteor site; deep drops |
| heartwood | rare | a single ancient tree, on a long timer |
| phoenix feather | deterministic | one creature, once |

**Arcane**, one per school.

| Spell | School | Cast | Interrupt | Cooldown | Mana | Level, needs | Target | Save | Effect |
|---|---|---|---|---|---|---|---|---|---|
| Firebolt | evocation | 1 | yes | 0 | 6 | 1, 11 Int | single | reflex, half | damage 2.0x |
| Ward | abjuration | 0 | no | 10 | 8 | 2, 11 Int | self or one ally | none | +defense equal to medium armor at level, 10 rounds |
| Acid Splash | conjuration | 1 | yes | 0 | 7 | 3, 12 Int | single | fortitude, half | damage 1.5x, then 0.3x per round for 3 rounds |
| Foresight | divination | 0 | no | 20 | 8 | 4, 12 Int | self | none | +10 percent dodge, 10 rounds |
| Daze | enchantment | 1 | yes | 5 | 6 | 3, 12 Int | single | will, negate | target speed halved, 3 rounds |
| Blur | illusion | 1 | no | 15 | 8 | 5, 13 Int | self | none | +15 percent dodge, 5 rounds |
| Drain | necromancy | 1 | yes | 3 | 7 | 5, 13 Int | single | fortitude, half | damage 1.5x; caster heals half of it |
| Haste | transmutation | 2 | yes | 20 | 15 | 8, 15 Int | self | none | +1 swing per round, 5 rounds |
| Fireball | evocation | 2 | yes | 8 | 15 | 10, 16 Int | area | reflex, half | damage 1.5x to each enemy |

**Divine**, one per god, plus a second for Good since healing is the
thing a group (4.7) needs first.

| Spell | Deity | Cast | Interrupt | Cooldown | Mana | Level, needs | Target | Save | Effect |
|---|---|---|---|---|---|---|---|---|---|
| Mend | Good | 1 | yes | 0 | 8 | 1, 11 Wis | self or one ally | none | heals 30 percent of the target's maximum |
| Sanctuary | Good | 2 | yes | 30 | 20 | 8, 15 Wis | group | none | damage taken halved, 5 rounds |
| Stillness | Neutral | 1 | yes | 10 | 12 | 4, 12 Wis | area | will, negate | every enemy's speed halved, 2 rounds |
| Blight | Evil | 1 | yes | 6 | 12 | 6, 14 Wis | area | fortitude, half | 0.4x per round to each enemy, 5 rounds |

Scaling for all of the above: the branch stat (Intelligence or Wisdom)
times the level multiplier, per 6.4. Fireball and Blight are the first
area spells and the reason 4.7 lands first. Sanctuary is the first
spell to need a group. Haste is the first to touch the swing meter
through an effect, which the `attacks` kind already supports.

What the list deliberately leaves out: anything that creates a mob or
an item (conjuration proper), anything that moves a character, and
anything global. Each needs an engine return channel that does not yet
exist.

---

*Retagged 2026-10-12 (6.1): the tables below give each spell's tier and
its school after the cut. Divination's spells became common or moved to
abjuration, illusion, enchantment, and evocation; transmutation's moved
to enchantment. Each school's first school spell is known at once by
its specialist; common arcane spells are bought, since there is no
school to unlock them. Every god's apostle knows Mend and Wrath at once
and may buy Bless. The enchantments of 5.6 are common, at level 25.*

**The expanded list (2026-10-11).** The first list above became the
innate spells of each school and god (6.1). Around them each school
grew to six or seven spells and each god to six: simple damage early,
then defence, buffs and debuffs, stronger spells taught at higher
levels, and one spell per school and god learned only from a scroll
(6.1), around levels 36 to 45. They live in
`data/scripts/spells_arcane.js` and `spells_divine.js`, which append to
the list in `rules.js`. Damage is in power units, as above; the
effect column names the kinds attached and their rounds; "Learned" is
innate, a trainer's price, or a scroll. Trainers are named in each
area's README. Two capstones are deliberately extreme and are the first
to watch in play: Time Stop (every enemy to a tenth of its speed for
five rounds, no save) and Power Word Stun.

The neutral god, which had no reachable temple, now has one: the
heart tree in Winterfell's godswood, with a weirwood seed as its
sacrifice.

**Arcane** (56 spells, six schools)

| Spell | School | Tier | Level, needs | Mana | Cast | Cooldown | Target | Save | Effect | Learned |
|---|---|---|---|---|---|---|---|---|---|---|
| Shield | abjuration | common | 1, 11 Int | 6 | 0 | 10 | self | none | block 5r | 100 silver |
| Ward | abjuration | common | 2, 11 Int | 8 | 0 | 10 | ally | none | defense 10r | 100 silver |
| Foresight | abjuration | common | 4, 12 Int | 8 | 0 | 20 | self | none | dodge 10r | 120 silver |
| Stoneskin | abjuration | school | 12, 16 Int | 14 | 1 | 20 | ally | none | soak 10r | known at once |
| Insight | abjuration | school | 16, 17 Int | 16 | 1 | 20 | group | none | dodge 8r | 1600 silver |
| Repulsion | abjuration | school | 18, 18 Int | 16 | 1 | 12 | area | will, negate | speedMult 3r | 1800 silver |
| Globe of Invulnerability | abjuration | school | 26, 19 Int | 26 | 2 | 30 | group | none | protect 5r | 3000 silver |
| Greater Foresight | abjuration | rare | 36, 21 Int | 30 | 1 | 40 | group | none | dodge 6r; crit 6r | scroll |
| Prismatic Ward | abjuration | rare | 38, 21 Int | 35 | 2 | 40 | group | none | protect 6r; thorns 6r | scroll |
| Acid Splash | conjuration | common | 3, 12 Int | 7 | 1 | 0 | single | fortitude, half | 1.5x damage; dot 3r | 100 silver |
| Mage Armor | conjuration | common | 5, 13 Int | 8 | 1 | 0 | ally | none | defense 40r | 500 silver |
| Acid Arrow | conjuration | common | 7, 14 Int | 10 | 1 | 4 | single | reflex, half | 1x damage; dot 4r | 700 silver |
| Web | conjuration | school | 9, 15 Int | 10 | 1 | 10 | area | reflex, negate | speedMult 3r | known at once |
| Ice Storm | conjuration | school | 16, 17 Int | 18 | 2 | 8 | area | reflex, half | 1.7x damage | 1600 silver |
| Cloudkill | conjuration | school | 24, 19 Int | 22 | 2 | 12 | area | fortitude, half | dot 6r | 2400 silver |
| Incendiary Cloud | conjuration | rare | 40, 22 Int | 38 | 3 | 25 | area | reflex, half | 1.5x damage; dot 6r | scroll |
| True Strike | enchantment | common | 1, 11 Int | 6 | 0 | 10 | self | none | crit 3r | 100 silver |
| Stone Fist | enchantment | common | 2, 11 Int | 6 | 1 | 10 | self | none | damage 6r; unarmedMult 6r | 150 silver |
| Daze | enchantment | common | 3, 12 Int | 6 | 1 | 5 | single | will, negate | speedMult 3r | 80 silver |
| Mind Spike | enchantment | common | 5, 13 Int | 8 | 1 | 0 | single | will, half | 1.8x damage | 500 silver |
| Sleep | enchantment | school | 7, 14 Int | 10 | 1 | 8 | single | will, negate | speedMult 3r | known at once |
| Haste | enchantment | school | 8, 15 Int | 15 | 2 | 20 | self | none | attacks 5r | known at once |
| Heroism | enchantment | school | 10, 16 Int | 12 | 1 | 20 | ally | none | damage 10r; steady 10r | 1000 silver |
| Slow | enchantment | school | 12, 16 Int | 14 | 1 | 10 | area | will, negate | speedMult 3r | 1200 silver |
| Confusion | enchantment | school | 18, 18 Int | 18 | 1 | 12 | area | will, negate | blind 3r; speedMult 3r | 1800 silver |
| Iron Body | enchantment | school | 22, 18 Int | 20 | 1 | 30 | self | none | soak 8r; defense 8r; speedMult 8r | 2400 silver |
| Tenser's Transformation | enchantment | school | 24, 19 Int | 22 | 2 | 30 | self | none | attacks 8r; damage 8r; defense 8r | 2600 silver |
| Hold Monster | enchantment | school | 28, 20 Int | 22 | 1 | 15 | single | will, negate | speedMult 4r | 3000 silver |
| Disintegrate | enchantment | school | 34, 21 Int | 28 | 2 | 15 | single | fortitude, half | 3.5x damage | 4000 silver |
| Power Word Stun | enchantment | rare | 38, 21 Int | 30 | 0 | 30 | single | none | speedMult 3r; protect 3r | scroll |
| Time Stop | enchantment | rare | 45, 22 Int | 40 | 1 | 50 | area | none | speedMult 5r | scroll |
| Firebolt | evocation | common | 1, 11 Int | 6 | 1 | 0 | single | reflex, half | 2x damage | 60 silver |
| Magic Missile | evocation | common | 2, 11 Int | 6 | 1 | 0 | single | none | 1.4x damage | 150 silver |
| Find Weakness | evocation | school | 10, 16 Int | 10 | 1 | 10 | single | will, negate | protect 4r | 1000 silver |
| Fireball | evocation | school | 10, 16 Int | 15 | 2 | 8 | area | reflex, half | 1.5x damage | known at once |
| Fire Shield | evocation | school | 12, 16 Int | 12 | 1 | 20 | self | none | thorns 10r | 1200 silver |
| Lightning Bolt | evocation | school | 14, 17 Int | 14 | 1 | 4 | single | reflex, half | 2.5x damage | 1400 silver |
| Cone of Cold | evocation | school | 20, 18 Int | 20 | 2 | 10 | area | reflex, half | 1.8x damage; speedMult 2r | 2000 silver |
| Chain Lightning | evocation | school | 30, 20 Int | 28 | 2 | 12 | area | reflex, half | 2.2x damage | 3600 silver |
| Meteor Swarm | evocation | rare | 42, 22 Int | 40 | 3 | 20 | area | reflex, half | 3.5x damage; dot 3r | scroll |
| Color Spray | illusion | common | 2, 11 Int | 7 | 1 | 6 | area | will, negate | blind 2r | 150 silver |
| Blur | illusion | common | 5, 13 Int | 8 | 1 | 15 | self | none | dodge 5r | 200 silver |
| See Invisible | illusion | common | 6, 13 Int | 8 | 1 | 0 | self | none | detectInvisible 60r | 500 silver |
| Invisibility | illusion | school | 6, 13 Int | 10 | 1 | 10 | ally | none | invisible 30r | known at once |
| Mirror Image | illusion | school | 9, 15 Int | 12 | 1 | 20 | self | none | dodge 4r | 900 silver |
| Phantasmal Killer | illusion | school | 20, 18 Int | 18 | 2 | 8 | single | will, half | 2.6x damage | 2000 silver |
| True Seeing | illusion | school | 28, 20 Int | 20 | 1 | 30 | self | none | detectInvisible 10r; detectHidden 10r; crit 10r; dodge 10r | 3000 silver |
| Mislead | illusion | school | 30, 20 Int | 22 | 1 | 30 | self | none | dodge 6r; crit 6r | 3600 silver |
| Weird | illusion | rare | 38, 21 Int | 36 | 2 | 25 | area | will, half | 2.5x damage; speedMult 2r | scroll |
| Chill Touch | necromancy | common | 1, 11 Int | 6 | 1 | 0 | single | fortitude, half | 1.6x damage; stat 3r | 100 silver |
| Drain | necromancy | common | 5, 13 Int | 7 | 1 | 3 | single | fortitude, half | 1.5x damage; drains 50% | 200 silver |
| Ray of Enfeeblement | necromancy | school | 8, 15 Int | 10 | 1 | 8 | single | fortitude, negate | stat 8r | known at once |
| False Life | necromancy | school | 12, 16 Int | 12 | 1 | 20 | self | none | regen 10r | 1200 silver |
| Vampiric Touch | necromancy | school | 16, 17 Int | 14 | 1 | 6 | single | fortitude, half | 2.2x damage; drains 75% | 1600 silver |
| Circle of Death | necromancy | school | 32, 20 Int | 30 | 2 | 15 | area | fortitude, half | 2x damage | 3800 silver |
| Finger of Death | necromancy | rare | 44, 22 Int | 38 | 1 | 30 | single | fortitude, half | 4.5x damage; drains 25% | scroll |

**Divine** (19 spells; "any" is the common set every apostle shares)

| Spell | God | Tier | Level, needs | Mana | Cast | Cooldown | Target | Save | Effect | Learned |
|---|---|---|---|---|---|---|---|---|---|---|
| Mend | any | common | 1, 11 Wis | 8 | 1 | 0 | ally | none | heals 30% | known at once |
| Wrath | any | common | 1, 11 Wis | 6 | 1 | 0 | single | will, half | 1.8x damage, of its god's kind | known at once |
| Bless | any | common | 3, 12 Wis | 8 | 1 | 10 | ally | none | defense 15r; protect 15r | 240 silver |
| Rotting Touch | evil | school | 1, 11 Wis | 6 | 1 | 0 | single | fortitude, half | dot 4r | 60 silver |
| Dread | evil | school | 4, 12 Wis | 8 | 1 | 6 | single | will, negate | speedMult 4r; damage 4r | 320 silver |
| Blight | evil | school | 6, 14 Wis | 12 | 1 | 6 | area | fortitude, half | dot 5r | known at once |
| Malediction | evil | school | 12, 16 Wis | 12 | 1 | 8 | single | will, negate | stat 8r; stat 8r; stat 8r | 1080 silver |
| Harm | evil | school | 28, 19 Wis | 26 | 2 | 8 | single | fortitude, half | 2.8x damage; drains 33% | 2800 silver |
| Unholy Word | evil | rare | 44, 22 Wis | 40 | 2 | 25 | area | will, half | 2.2x damage; speedMult 3r; damage 3r | scroll |
| Sanctuary | good | school | 8, 15 Wis | 20 | 2 | 30 | group | none | protect 5r | known at once |
| Smite | good | school | 12, 16 Wis | 12 | 1 | 2 | single | will, half | 2.2x damage | 1080 silver |
| Prayer of Healing | good | school | 28, 19 Wis | 28 | 2 | 12 | group | none | heals 25%; regen 6r | 2800 silver |
| Holy Word | good | rare | 42, 22 Wis | 38 | 2 | 25 | area | will, half | 2.5x damage; blind 2r | scroll |
| Entangle | neutral | school | 2, 11 Wis | 7 | 1 | 4 | single | reflex, negate | speedMult 3r; dodge 3r | 160 silver |
| Stillness | neutral | school | 4, 12 Wis | 12 | 1 | 10 | area | will, negate | speedMult 2r | known at once |
| Barkskin | neutral | school | 7, 14 Wis | 10 | 1 | 15 | ally | none | soak 12r; defense 12r | 560 silver |
| Call Lightning | neutral | school | 14, 17 Wis | 16 | 2 | 6 | single | reflex, half | 2.8x damage | 1260 silver |
| Restoration | neutral | school | 26, 19 Wis | 22 | 2 | 10 | ally | none | heals 35%; regen 5r | 2600 silver |
| Storm of Vengeance | neutral | rare | 44, 22 Wis | 40 | 3 | 25 | area | reflex, half | 2.2x damage; speedMult 3r | scroll |

### 6.6 Strengthening magic: caster feats and caster gear (decided 2026-10-11)

**Thesis.** Magic grows with the branch stat and level (6.4) and needs
nothing else; feats and gear are for steel.

**Antithesis.** Then a caster's build is one number. Every level's
feat pick (7.2) is a combat feat a caster does not want, gear is
chosen for its Intelligence or Wisdom alone, and two casters of a
school differ only in how many points they put in one stat. 7.3 says
there are no classes, so a build has to be made of choices, and a
caster has had none.

**Synthesis.** Casters get feats and gear of their own, built on new
effect kinds the spell path reads, and the arcane and divine branches
reward different things, so that the choice of branch is felt in the
build and not only in the spell list:

- **Arcane rewards specialisation and force.** Spell Focus (more power
  for one school, taken per school), Empower (more damage from damage
  spells), Penetration (saves are harder), Arcane Reservoir (a deeper
  mana pool), Quicken and Combat Casting (shorter casts, casting that
  damage does not break), and a chance for spells to strike critically.
- **Divine rewards devotion and the group.** Devotion (more power for
  your god's spells), Healer's Grace (stronger healing), Channel
  (harder saves against your god's spells), Martyr (healing another
  heals you a little), Divine Favor (mana returns slowly even in a
  fight), Shield of Faith (casting a helpful spell guards you).

A caster feat needs its branch (7.2): an arcane feat wants an arcane
school unlocked, a divine one an apostleship.

**Revised 2026-10-12.** Spell Focus is one feat, not eight: +20 percent
to every spell the character casts, for any caster. Archmage (arcane)
and Saint (divine) rise from +15 to +60 percent and become the late
reward of a caster's build, so that a level-20 caster has a pick
worth waiting for. With common spells (6.1), a school is no longer
what lets a character cast, so an arcane feat wants a learned arcane
spell, and Archmage wants a specialty; a divine feat still wants an
apostleship. The new effect kinds,
which items may also carry (5.4):

| Kind | Params | Read where |
|---|---|---|
| `spellPower` | `mult`, and one of `school`, `deity`, `branch`, or `kind` (damage, heal) | spell power, before saves |
| `spellCrit` | `chance`, `mult` | each damaging or healing target |
| `penetration` | `amount` (a multiplier on the caster's side of the save) | saves |
| `manaMax` | `amount` (percent of the pool) | derived stats |
| `manaFight` | `fraction` (of the pool per round, in a fight) | regeneration |
| `castCost` | `mana` (multiplier), `rounds` (cut), `steady` (no interruption), with an optional `branch` | the engine's cast, through a `castCost` hook |
| `martyr` | `fraction` (of healing given, back to the caster) | resolution |
| `faithShield` | `mult`, `rounds` (a `protect` on the caster after a helpful spell) | resolution |

Casting cost, cast time, and interruption are engine-owned (6.4), so
the rules reach them through one optional hook, `castCost(caster,
spell)`, which returns the mana, cast rounds, and whether damage
interrupts, defaulting to the spell's own numbers. Everything else is
rules-only.

Caster gear uses the same kinds: staves and wands, orbs and tomes held
in the off hand, robes, circlets, and holy symbols, arcane pieces
leaning on spell power, mana, and penetration, divine ones on healing
and devotion. Their power is priced in the item budget like any
effect (5.2), a step below the feats so that gear supplements a build
rather than replacing it: on one item, spell power at most 15 percent,
spell criticals 8 percent, penetration 20 percent, mana 25 percent,
in-fight mana 2 percent a round, casting at most 15 percent cheaper,
Martyr 20 percent, a faith shield no stronger than 15 percent, and no
quicker casting at all, which stays a feat.

## 7. Progression

### 7.1 Experience and levels

**Thesis.** A quadratic curve, as most MUDs use: total experience for
level N grows with N squared, so each level costs a bit more than the
last. Familiar, and easy to tune with one constant.

**Antithesis.** Experience per kill grows with the victim's level, and
a player at level N fights mobs at level N. Under a quadratic total,
the *kills* per level come out constant, ten at every level from 2 to
40. The curve looks steeper in experience and feels flat in play, and
with no level cap (decided 2026-09-24) it never stops: level 40 arrives
after 26 sessions of the same pace as level 5. A curve that is to feel
"not much at first, then larger and larger" has to outgrow the kill
reward, not merely grow.

**Synthesis (leaning, 2026-09-24).** The cost of a level is the
experience to go from N to N+1, and it grows geometrically with a
linear factor in front:

    cost(N) = base * N * r^(N-1)

The `r^(N-1)` is what makes late levels rare in practice without a cap;
the `N` in front cancels the kill reward's own growth in the early
levels, so kills per level rise from the first level and never dip.
That dip is real: a purely geometric cost with the kill reward growing
linearly gets *cheaper* in kills for the first ten levels, so a player
would race from level 3 to 12 in under two sessions and then hit a
wall. The hybrid has no valley.

The candidates, in kills per level and cumulative sessions, using the
2.5 targets (about fifteen even kills per session) and a kill reward of
ten times the victim's level:

| Reach level | A: quadratic | B: geometric r=1.20 | C: geometric r=1.30 | D: hybrid r=1.15 |
|---|---|---|---|---|
| 2 | 10 kills, 0.7 sessions | 5, 0.3 | 5, 0.3 | 5, 0.3 |
| 5 | 10, 2.7 | 2, 0.8 | 3, 0.9 | 8, 1.7 |
| 10 | 10, 6.0 | 2, 1.6 | 5, 2.1 | 15, 5.6 |
| 20 | 10, 12.7 | 7, 4.5 | 30, 11.5 | 62, 29 |
| 30 | 10, 19.3 | 28, 15 | 267, 91 | 250, 126 |
| 40 | 10, 26.0 | 131, 63 | 2740, 880 | 1013, 515 |

A is flat in play. B and C both have the early valley (two kills a
level around level 5 to 10, which is a feat every few minutes). D
rises monotonically: a level every outing early, one every few
sessions in the teens, one every ten or more past twenty. Its effective
ceiling sits near level 30 for a regular player, and level 40 is a
matter of years, which is what "no cap" should mean: nothing stops you,
and almost nobody gets there.

Starting values, arbitrary and for playtesting like 3.3's: `base` 50,
`r` 1.15. If the teens feel slow, lower `r` toward 1.12 before touching
`base`; `base` moves the whole curve and `r` moves where the wall is.

Kill reward: ten times the victim's level at an even fight, decaying
with level gap in both directions (2.2): a mob well below the player's
level gives nothing, and one well above gives more, but not so much
more that fighting up is the efficient path (2.3 wants death to come
from that choice, not the curve to reward it). The decay shape is a
starting value for the simulator, not a decision here.

Death (2.3) costs a fraction of the progress toward the next level.
Whether it can cross a level boundary is still open there; under this
curve a late level is many sessions of progress, so the fraction, not
the boundary rule, is what decides how much death stings.

**Alternative argued on 2026-09-24: a fixed reward per mob.** Each mob
prototype carries an `xp` number the builder sets, and the total grows
geometrically. Why would that not work? It would. The kills-per-level
valley above is a property of the *ratio* of cost to reward, and
fixing the reward per mob moves the reward's growth out of a formula
and into the builder's hands. If builders give higher-level mobs
proportionally more experience, the valley comes straight back; if
they give every mob about the same, a pure geometric cost has no valley
and the `N` factor is unnecessary. So the alternative works exactly as
well as the content is authored, and it has real merits:

- Legibility. A mob is worth what it says. A boss can be worth a lot
  without a formula having to know it is a boss.
- The level-gap decay downward comes for free: a late level costs so
  much that a rat's fixed reward is nothing, with no "grey mob" rule.
- `xpForKill` becomes trivial and group split is a plain division.

And real costs:

- The curve's feel now depends on every area author, and the valley is
  a content bug that no formula change can fix.
- There is no decay *upward*. A mob far above the player gives its
  full reward, so fighting up is rewarded by the curve, which 2.3 does
  not want. That needs a separate rule, and then a formula is back.

The synthesis is to take the legibility without giving up the formula:
the mob prototype carries an optional `xp` field that *overrides* the
formula's default, and the default is the formula above. A builder
sees what a mob is worth (the `stat` command shows the computed value),
can pin it for a boss or a set piece, and the decay still applies on
top of either. This is the smallest reading of the alternative that
keeps the curve's shape out of the content.

Consequences: `xpToLevel(level)` is the running sum of `cost` and the
engine already treats it as a total. `xpForKill` gains the decay and
reads an optional `xp` on the victim's prototype. The simulator should
report levels per session against this table.

### 7.2 What a level grants, and making each one count

**Thesis.** A level grants what ROM grants: a few hit points, a few
practices, a stat point now and then. Steady, predictable growth.

**Antithesis.** Steady growth is growth nobody feels. Five hit points
on a hundred is noise, and this game has already given away the two
things that made a ROM level an event: new spells arrive by discovery
(6.1), not by level, and there are no class tables to open. If a level
only nudges numbers, it competes with gear (2.2) and loses, because a
new sword is visible and +3 health is not.

**Synthesis (leaning, 2026-09-24, revised the same day).** A level is
felt when it is *rare*, *large*, and *chosen*. Three grants, with the
starting quantities in 3.3:

1. **A slight level multiplier, and stat points.** Level itself
   multiplies the character's numbers by a small factor per level, in
   keeping with 3.1's rule that power is multiplicative. It is the
   smallest of the grants on purpose: it keeps level a real pillar
   (2.2) without making it the visible one. Alongside it, each level
   grants stat points the player spends on any of the five (decided
   2026-09-24; the count is a 3.3 starting value). This is the
   continuous build 7.3 promised.
2. **Feats.** One or two per level, chosen by the player from what is
   available, or granted by a trainer found in the world. A feat is a
   permanent effect (5.4) the character holds forever: a crit chance,
   ROM's second and third attack (4.4), a stronger block, a wider
   dodge, a proc. Every feat carries a minimum level, so the list a character
   can choose from grows as it levels, and the limit of one or two per
   level is the pace at which a build assembles. A feat that is an
   action or a proc may carry an effectiveness rating that improves
   with use (7.4); a flat grant does not. Feats are the answer
   to the "skills" question and need no new system: items
   give intrinsic effects, discovery gives magic, levels give feats.
   This is what makes a level-up a decision the player remembers.
3. **Health, grown as a multiplier.** `healthMax` is a base that grows
   with level, times the Constitution multiplier (3.1). A level should
   raise the base enough to be seen on the prompt.

Rarity is the lever the feats depend on: the fewer levels there are,
the more each can grant. The leaning is few levels far apart, so that a
level is a career-scale event (2.5) that takes several sessions, while
gear is the session-scale one. No hard level cap, per 2.2; the
`xpToLevel` curve instead steepens so that late levels are rare in
practice, the same asymptote-instead-of-cap shape used everywhere else
in this document. Death's experience loss (2.3) is what makes the gap
between levels feel like something held, not just waited for.

Consequences: the engine needs a `feats` list on the character, which
is the character's effects list with a permanent lifetime (5.4), so no
new storage; banked counts of unspent stat points and feat picks; and
a command to spend either (`train` is ROM's word, and a trainer NPC can use the same
path). `onLevel` returns the picks granted and the message; the choice
itself is a command, since a hook cannot ask a question. Feats have a
`level` field and the pick command enforces it.

**Requirements (decided 2026-10-11).** The designer asked for feats,
skills, and spells to be gated by level and stats. A feat names, beside
its `level` and the feats it `requires`, the trained `stats` it wants
and, for a caster feat, its `branch` (6.6): an arcane feat needs an
arcane school unlocked, a divine feat an apostleship (since 2026-10-12,
a learned arcane spell, and a specialty for Archmage; 6.6). Combat feats
want the stat that powers them (Strength for Brawn's heirs and the
extra attacks, Dexterity for evasion and finesse, Constitution for
toughness and soaking), rising with the feat's level as skills' do
(7.4). As with skills, requirements read trained stats, not gear, and a
feat once taken is kept even if a later change raises its bar. `feat
all` lists every feat with what it needs, so the build is visible before
the points are spent; bare `feat` shows only what can be taken now.

**Fighting styles (decided 2026-10-11).** Two feat lines go with the
hands of 4.4, each a chain where a feat requires the one before it.
Two weapons, on Dexterity: Dual Wield (3; removes the untrained
penalty), Two-Weapon Fighting (10) and Greater Two-Weapon Fighting
(18), each a swing a round for the second weapon. A two-handed weapon,
on Strength: Great Weapon Fighting (3), Great Weapon Mastery (10), and
Devastating Blows (18), each 20 percent more damage with it. Nothing
stops a character taking both lines or Shield Focus beside them; the
hands decide which one is in use at a time. 3.3's per-level
multiplier and 7.1's curve are the next things to set, and they should
be set together against a target of levels per session.

### 7.3 Classless

**Thesis.** Classes, as in ROM: warrior, mage, cleric, thief. Players
know them, and a class is a build chosen once, which makes the early
game legible.

**Antithesis.** Every class is a balance target against every other
class, and 2.4 already forbids any class that cannot solo. The work of
keeping four classes fair is work that produces no content. And 6.1
already gives characters a way to differ by what they have earned.

**Synthesis (leaning, 2026-09-24).** No classes. A character is level,
stats, gear, and the set of magic schools it has unlocked. Build is
chosen continuously: by stat allocation, and by which totems the player
finds and pursues. There is nothing to balance against a class because there is
no class; there are only pillars, and those are uncapped by design.

**Open: a limit on unlocked schools.** Over a long enough career a
character could unlock every school, at which point veteran builds
converge and discovery stops meaning anything. Three options:

- No limit. Differences come from which schools a character has
  unlocked *so far* and which materials it has on hand. Consistent with
  no caps (2.2); revisit only if convergence is observed.
- A soft limit. Each further school costs more to unlock or to cast,
  so specialising is cheaper than breadth without being enforced.
- A hard limit. A fixed number of slots, with a way to give one up.

Leaning: no limit for now, per 2.2. This is the decision most likely to
be revisited once there are enough schools to matter.

**Decided 2026-10-12: one.** A character specialises in one school, for
good, and loses the opposite school (6.1). Breadth comes from common
spells, which every caster can learn; depth from the specialty. Veteran
builds cannot converge, because no character can hold opposite schools,
and the three Orders (White, Red, Black) give two arcane characters of
the same level a visible difference, which is what this section wanted
from the start.

### 7.4 Effectiveness: skills and some feats improve with use

**Thesis.** ROM's skill percentage: a skill starts low, rises with
practice and use, and the number is the chance the attempt works. A
kick at 40 percent fails six times in ten.

**Antithesis.** A failure chance makes a low skill feel like a coin
that mostly comes up wrong. It is binary randomness of the kind 4.2
rules out, it punishes trying a new thing, and it says nothing about
*how well* the thing worked when it did.

**Synthesis (decided 2026-09-24).** Skills and some feats carry an
*effectiveness* from their starting value up to 100 percent. It is not
a chance of failure. It is how much of the ability's full potential the
attempt delivers, and what that means is the ability's own business:
for one it scales damage, for another it scales the chance to land or
the duration of an effect, for a third it scales a resource cost. An
ability can be used the moment it is gained, at its starting
effectiveness, and every use has a chance to raise it, until it reaches
100 and stops. A feat that is a flat grant (+2 Constitution) has no
rating; one that is an action or a proc may.

What improves it: use, as in ROM. The rate and the starting value are
per ability, set in the rules. Whether a trainer can raise it, and
whether it decays, are open.

"Skills" here are the *actions* of 2.1: the one thing a player may do
each round beyond the auto-attack, plus a few passives that work on
their own. They are distinct from feats (chosen, permanent, no rating)
and from spells (discovered, material-costed).

**Requirements (leaning, 2026-10-11).** With mana (6.2) and stamina
(7.7) in, the designer asked for skills to need stats as well as a
level, so that a build is something a character works toward rather
than buys. Every taught skill names the trained stats a trainer wants
before teaching it: one stat, matching what the skill does (Strength
for Bash and Sunder, Dexterity for Backstab and Circle, Constitution
for Rescue and Second Wind), rising with the skill's level from 11 or 12
for the first skills to 17 or 18 for the last. Innate skills need none.
Requirements read trained stats, not gear, so a ring cannot lend one
and the stat points a character spends are the build; trained stats
never fall, so a requirement once met stays met. Skills learned before
this keep working. With six points at creation and two a level, a level
12 character has 28 to spend: 16 in one stat for its signature skills
leaves room for a second. The numbers are starting values (the Needs
column below).

**How skills are gained (decided 2026-09-25).** Most skills are
taught: a trainer in the world teaches a set of skills, each for a
price in coin (7.5) and, for some, items handed over as well (named as
materials, the same way spells name theirs, so a beast-master can ask
for wolf fangs), and `practice` at the trainer buys one at its starting
rating, level permitting. A few skills are *innate*: every
character has them at level with no teacher (Kick is the first). This
keeps trainers and coin meaningful without making the basics gated,
and nothing competes with feats for the level-up choice.

**First skills**, starting values:

| Skill | Level | Needs | Kind | Cooldown | Stamina | Start | Taught | What the rating scales |
|---|---|---|---|---|---|---|---|---|
| Kick | 1 | none (innate) | action, single target |  | 20 | 30 | innate | strike: 1.8 rounds of the user's weapon at full skill, no weapon needed |
| Bash | 3 | 12 Str | action, single target | 3 | 40 | 25 | 3 gold | the chance the target loses its next round of swings; 0.5x a standard swing aside |
| Twin Strike | 5 | 13 Dex | passive | | | 20 | 15 gold | the fraction of an extra swing per round; 100 is a full second swing |
| Rend | 3 | 12 Str | action, single target |  | 30 | 30 | 1 gold and two wolf fangs | strike: 1.2 rounds, and a bleed of 40 percent of it for three rounds |
| Riposte | 4 | 12 Dex | passive | | | 20 | 5 gold | block chance with a weapon in hand, up to 20 percent at full skill |

**Combat skills** (approved 2026-09-28), after ROM 2.4 and its
descendants. Each has trainers in the areas whose level band fits it,
and all of them are sold in the balance lab.

| Skill | Level | Needs | Kind | Cooldown | Stamina | Start | Taught | What the rating scales |
|---|---|---|---|---|---|---|---|---|
| Dodge | 1 | 11 Dex | passive | | | 20 | 2 gold | up to +10 percent dodge |
| Trip | 2 | 11 Dex | action, single | 3 | 30 | 25 | 2 gold | -5 percent dodge for two rounds, the chance of a lost round; 0.3x damage aside |
| Hand to Hand | 2 | 11 Str | passive | | | 20 | 3 gold | unarmed damage, up to double |
| Dirt Kicking | 3 | 12 Dex | action, single | 4 | 40 | 25 | 3 gold | blindness: up to 40 percent of swings miss for two rounds |
| Shield Block | 3 | 12 Con | passive | | | 20 | 4 gold | up to +15 percent block, shield worn |
| Backstab | 4 | 13 Dex | action, single |  | 50 | 25 | 5 gold | strike: 4 rounds; dagger, and only to open a fight |
| Fast Healing | 4 | 12 Con | passive | | | 20 | 4 gold | resting regeneration, up to +50 percent; improves while resting |
| Enhanced Damage | 5 | 13 Str | passive | | | 15 | 10 gold | up to +15 percent damage on swings |
| Hamstring | 6 | 13 Dex | action, single | 4 | 40 | 25 | 6 gold | a slow to 70 percent for four rounds; 0.4x damage aside |
| Disarm | 6 | 14 Dex | action, single | 6 | 50 | 20 | 7 gold | one to three rounds fighting bare-handed; needs a weapon each side, in hand or in a mob's natural attack |
| Feint | 7 | 14 Dex | action, single, cannot be dodged | 3 | 30 | 25 | 7 gold | -10 percent dodge and block for three rounds |
| Berserk | 8 | 15 Str | action, self | 20 | 60 | 25 | 9 gold | six rounds of up to +3 Strength and +0.5 swings, at -5 percent dodge |
| Envenom | 9 | 15 Dex | action, self | 10 | 40 | 25 | 9 gold | ten rounds of poison on every hit, two doses deep |
| Rescue | 10 | 14 Con | action, ally | 2 | 30 | 30 | 10 gold | always turns the ally's attackers on you; the rating shields the ally (up to 30 percent less damage for two rounds) |
| Circle | 10 | 16 Dex | action, single | 2 | 40 | 20 | 12 gold | strike: 2.4 rounds; dagger, mid-fight only |
| Sunder | 12 | 16 Str | action, single | 6 | 50 | 20 | 15 gold | up to half a set of armor stripped for five rounds; heavy weapon; 0.6x damage aside |
| Second Wind | 12 | 16 Con | action, self | 30 | 0 | 25 | 15 gold | an instant heal of 10 to 25 percent of maximum health |
| Triple Strike | 12 | 16 Dex | passive | | | 15 | 40 gold | a further fraction of a swing, only on top of Twin Strike |
| Whirlwind | 15 | 17 Str | action, area | 3 | 60 | 20 | 25 gold | strike: 1.2 rounds to every foe fighting you |
| Coup de Grace | 18 | 18 Str | action, single |  | 50 | 20 | 30 gold | strike: 5 rounds against a foe below a quarter of its health, 0.6 rounds otherwise |

Consequences (built 2026-09-28): skill targets are `single`, `area`
(every foe fighting the user; one use, one improvement roll, one
cooldown), `ally` (a player or group member someone is fighting;
`taunt` in the result turns their attackers), and `none`. A result may
carry `heal` for the user and `cooldown` to replace the skill's own for
that use (Relentless).

**Strikes and utility (decided 2026-10-11).** The designer found the
martial skills underwhelming: with stamina and cooldowns as their
price, a skill meant to deal damage should deal enough to be worth it.
Measured in units of a standard swing, a damage skill fell behind as a
character's gear and extra swings grew, and at its starting rating a
Kick was a third of a swing. So the skills split in two:

- A *strike* (Kick, Rend, Backstab, Circle, Whirlwind, Coup de Grace)
  is measured in rounds of the user's own weapon: its damage and the
  added damage it carries (5.3) times its speed, or a power unit if
  that is more, so a dagger and a maul of the same damage per round
  give the same Kick. The rating takes it
  from half its potential to all of it rather than from nothing, it can
  crit as a swing does, and Enhanced Damage adds to it. Multipliers are
  in the tables above.
- A *utility* skill (Bash, Trip, Hamstring, Sunder, Disarm, Dirt
  Kicking, Feint) is for what it does. Its damage, where it has any,
  stays a small flat share of a power unit that gear does not grow;
  the rating scales its effect.

Measured at level 10 in standard kit on an even mob: a round of
auto-attack about 14, a Kick at full skill about 25 (16 before), a fresh
Kick about 16 (5 before), a Bash about 7.

Disarm needs a weapon on both sides. A mob with no weapon item whose
natural attack is a weapon by its verb (slash, stab, cleave, shoot, and
the like) counts as armed; disarmed, it hits for half until it
recovers. A bite or a claw cannot be disarmed.

Twin Strike is the first of a chain: Triple Strike follows (and Quad
Strike, if ever, for special cases), each a further swing. Improvement: each use (each round in a fight, for passives) has
a chance to raise the rating by one to three points, the chance
shrinking as the rating nears 100. One active skill per round.

Consequences (built 2026-09-25): a skill's rating lives in the `state`
of a `skill` effect on the character (`params.skill` names it), so it
persists like any effect. `useSkill` resolves an active skill and any
hook may return a `skills` map of updated ratings, which the engine
stores: `useSkill` for the skill used, `onTick` for passives in use.
Typing a skill's name uses it; the command table is searched first, so
a skill never shadows a command. Open: the simulator should be able to
pin a rating for a run, so "at 60" and "at 100" can both be measured.

### 7.5 Money

**Status: decided 2026-09-25.** Coin as in ROM: silver and gold, a
hundred silver to the gold. A character carries a wallet (one number,
in silver, shown as gold and silver). Coins in the world are piles that
go into the wallet on `get`; `give` and `drop` take an amount and a
unit. Death puts the wallet in the corpse with everything else (4.6).

Sources: mobs carry coin, about three silver per level with a wide
spread from the rules' `moneyFor` hook, or a stated amount on the
prototype. Sinks: trainers (7.4) and merchants. Open: whether anything
else costs coin.

Merchants (decided 2026-09-28): a mob with a `trades` list sells each
listed item for a stated price in silver, in items taken from the
buyer's inventory, or both, so a smith can ask coin and three pelts
for a better shield. The prices are the builder's, stated on the
prototype; nothing is haggled and stock never runs out. A merchant
buys anything with a value for a tenth of it, a weapon or armor without
a stated value counting ten silver a level, and nothing flagged `quest`
or `newbie`. What it buys is destroyed, not resold.

Coin made scarcer (2026-10-12). Played at the middle levels, gold came
too easily, and mostly from selling loot rather than from purses.
Builders value gear at about ninety silver a level, so at half value a
single looted piece sold for nine purses of its level; with a piece on
nearly every mob and the chance drops besides, a level-thirty character
earned a forty-gold spell in a handful of kills. A merchant now pays a
tenth (a looted piece is worth about three purses, and a named one
still more), and purses are three fifths of what they were: the hook's
rate fell from five silver a level to three, and every amount stated on
a mob was cut by the same share. The early levels, where gear sells for
little, feel the purse cut more than the sale cut, so trainers' first
skills take somewhat longer to afford. Quest silver (7.6) is unchanged
and is now a larger share of income. The numbers are for playtest.

### 7.6 Quests

**Status: decided 2026-09-25.** After ROM's questmaster. A mob flagged
`questmaster` hands out one task at a time, drawn at random from the
whole live world and scaled to the player: the engine picks a fightable
mob within a band of the player's level (widening the band when nothing
is near), which fixes the area and the difficulty, and then either asks
for that kill or, half the time, plants a quest item somewhere in that
mob's area and asks for it back. The task has a clock. Finishing and
returning with `quest complete` pays quest points, a second currency
beside coin, that a mob with a `sells` list exchanges for items at the
prices the builder stated.

What the engine owns: the draw (live mobs only, never the peaceful, the
service mobs, safe rooms, or detached areas), the clock, the item stamp
(a planted item belongs to one player, crumbles when the clock runs out,
and is swept up when the quest ends), the credit (any mob of the
target's kind counts, for every grouped player in the room, so a target
that died to someone else is not a dead end), the cooldown before the
next request, and a longer one after `quit` so quitting is not a
reroll. A quest survives logout; a planted item that was lying about is
planted again at login, since rooms are not saved.

What the rules decide, in `rules.js`: `questRules(player)` returns the
level band and the timers in rounds; `questReward(player, quest)` is
asked once, when the quest is given, and returns the points, silver,
and experience on offer and an optional message. The engine shows those
numbers and pays them on completion. Defaults without the hooks: three
levels either way, fifteen minutes (in rounds of the rules' `roundSeconds`), five points
plus the target's level.

Sinks are content: the herald in the Long Hall sells starter and
mid-level gear. Open: whether quest points should also buy services
(a resurrection, a rename), and whether a vendor's stock should rotate.


### 7.7 Stamina

**Status: leaning, 2026-10-11.** Asked for by the designer: a resource
that physical characters manage and trade against, without limiting
movement.

**Thesis.** ROM's movement points: walking spends them, resting brings
them back. Familiar, and a reason to rest.

**Antithesis.** A movement tax limits the one thing the game should
never make tedious, getting somewhere, and makes no decision in a fight.
And a second resource that is mana by another name (spent, refilled
between fights) only adds a number to the prompt; the fighter would
manage exactly what the caster manages.

**Synthesis (leaning).** Stamina is the pacing resource *inside* a
fight, where mana (6.2) is the budget *across* fights. It refills
quickly, in a fight too, and skills (7.4) spend it. The question it
poses is when to burst within this fight, where mana's question is
which fight to spend on. Walking never costs stamina.

- **Pool.** The same for everyone at every level (revised
  2026-10-12, below). Costs are a share of it, so a skill weighs the
  same at level 40 as at level 4.
- **Regeneration.** From empty to full in about ten rounds in a fight,
  twice as fast out of one, and faster again resting or asleep at the
  multipliers health uses. One even fight (4.5) is about one refill.
  The better of Strength and Constitution speeds it, at half the slope
  of a stat multiplier (3.3), so a classless character still feels its
  build here as it does in mana.
- **Gear.** Heavy and plate armor slow it by the share of the body
  they cover, and a heavy weapon slows it a little. This is the cost
  of heavy gear that 4.3's dodge penalty alone did not carry, and the
  reason a skill-heavy character leans to light armor.
- **Exhaustion.** At zero a character can use no skill and dodges a
  little less until it recovers. It never hurts by itself (2.3).
- **Costs replace most cooldowns.** Before stamina, every active skill
  was paced by a cooldown. Now each costs stamina, and a cooldown is
  kept only where a skill would otherwise chain into a lock (stuns,
  slows, disarms, blinding) or is a once-a-fight move (Berserk, Second
  Wind). Plain damage skills (Kick, Rend, Backstab, Coup de Grace)
  have none: spend as fast as the pool allows, one skill a round as
  before (2.1). Relentless, which cut cooldowns, also cuts stamina
  costs by a fifth.

Starting values, arbitrary in the sense of 3.3: a pool of 100; 10 a
round back in a fight at a 10 stat, 2.5 percent more for each point of
the better of Strength and Constitution above 10; heavy armor slows
regeneration by a quarter and plate by two fifths at full cover, a
heavy weapon by a tenth; exhausted, 5 percent less dodge. Costs run
from 20 (Kick) to 60 (Berserk, Whirlwind), so a character kicks five
times from full and then every other round, which is the pace Kick's
old cooldown set, and it takes a stat of 50 to kick every round.

**Revised 2026-10-12: a fixed pool.** The designer found stamina
throwaway at higher levels. The pool had been 20 plus 1 a level, times
the better Strength or Constitution multiplier, with regeneration a
tenth of the pool a round; costs were flat. So regeneration grew with
level and stats while costs did not: by level 20 at base stats it paid
for a kick every round, and by level 40 with a strong build it paid
for nearly any skill every round, with the pool left over. With no
level cap and no stat ceiling (2.2, 3.1), raising costs or slowing
regeneration once would only have put the drift off. Fixing the pool
and pricing skills against it holds the ratio at every level; stats
keep a softened hand on recovery alone. Costs went up fivefold with
the pool, so a level-1 character paces as before. Second Wind is free and keeps its long cooldown: it is
how a character recovers, not a burst.

Open: whether `flee` should cost stamina, so an exhausted character
cannot run as easily; and a power-attack style toggle that drains
stamina each round for damage. Neither is built.

Consequences: `derivedStats` returns `staminaMax` and `onTick` a
`staminaDelta`; `skillList` entries gain `stamina`; the engine refuses a
skill the user cannot pay for and spends the cost when the use goes
ahead (a dodged skill was still attempted); `useSkill` may return
`stamina` to replace the cost for one use; the pool is saved, shown on
the prompt as `st`, on `score`, and in client state.
---

### 7.8 Stealth

**Status: decided 2026-10-11.** Asked for by the designer with
aggression (8.1): sneak, hide, and the like, so that the gear and
spells that grant sight are worth wearing.

**Thesis.** ROM's thief skills: `hide` makes you unseen until you act,
`sneak` keeps your movements out of the room's messages, and detect
hidden sees through both. Familiar, and it gives sight a job.

**Antithesis.** Stealth that hides nothing that matters is flavor; in
ROM, sneak skipped a line of text and aggressive mobs bit you anyway.
And stealth that cannot be answered makes every hider untouchable.

**Synthesis.** The engine already knew invisible, hidden, and the two
detects (they are on gear and in the spell lists). Stealth gives them
something to do:

- **Hidden** (the Hide skill, gear, or a mob's nature): unseen, so not
  listed, not targetable, and not attacked by an aggressive mob, except
  by eyes with Detect Hidden. Hiding of the character's own ends when
  it moves or a fight starts; hiding from gear or a mob's prototype
  lasts, which is how an ambusher lies in wait.
- **Sneak**: seen if looked at, but not *noticed*: arrivals and
  departures go unannounced, and an aggressive mob passes the sneaker
  by, except a viewer with Detect Hidden. It lasts through movement,
  which is the point. The Sneak skill is a toggle (2026-10-12): on
  until used again, since renewing it was a chore and not a choice;
  turning it off is free, even winded or in a fight.
- **Invisible** (the spell, gear): as before, and invisibility of the
  character's own ends when a fight starts.
- Whoever is fighting you can always be seen, so an ambush is answered
  once it is sprung.

Mobs carry any of these as prototype effects: a sneaking wanderer comes
and goes unannounced, a hidden aggressive mob is an ambush that only
Detect Hidden spots before it strikes, and a mob with Detect Hidden
guards against sneaks.

| Skill | Level | Needs | Kind | Cooldown | Stamina | Start | Taught | What the rating scales |
|---|---|---|---|---|---|---|---|---|
| Hide | 2 | 12 Dex | action, self, not in a fight | | 20 | 30 | 2 gold | how long the hiding lasts, 10 to 60 rounds |
| Sneak | 4 | 13 Dex | toggle, self, on only out of a fight | | 30 to turn on | 30 | 5 gold | nothing yet |

Taught by the master thief in Midgaard, the cutpurse at the Burning
Wizard on the Sword Coast, Kashya of the Rogues, and the lab's fencing
master.

Consequences: the engine reads a `sneak` effect kind beside the four it
knew; movement messages go only to those who notice the mover; `reveal`
strips the character's own `hidden` on a move and its own `hidden` and
`invisible` when a fight starts; aggression (8.1) asks whether the mob
notices its prey, not only whether it can see it.

## 8. Mobs

**Thesis.** Mobs are a simplified table: level implies everything. A
builder writes `level: 8` and the engine knows the mob's health, damage,
and defense. Fast to author and always consistent.

**Antithesis.** Then every level-8 mob is the same mob. A caster, a
brute, and a swarm rat cannot be told apart except by level, and the
six stats (3.2), the effects (5.4), and the whole gear system stop at
the player's side of the fight.

**Synthesis (decided 2026-09-24).** A mob is a character. Its level
sets a *baseline* for everything, the same way an item's level does
(5.2), and every value is overridable in the prototype. The baseline
gives it the six stats at 10, health from the 3.3 curve, a natural
attack that is the power unit at its level (5.2) scaled by `mobDamage`
(0.45 to start), and natural armor on the `medium` armor baseline, all
at its level.

Why the scaling (found in the first simulator run, 2026-09-24): a
mirror fight can only end with the winner near empty, because equals
trade down together. For a player at level in standard gear to finish
an even fight at the 4.5 health target, a level-N mob has to hit softer
than a level-N player. Health stays equal so the fight length is set by
the player's damage; the mob's damage sets how much the player has left.
A mob that *wields* a weapon uses the weapon's full numbers, so an armed
mob is markedly stronger than its level's baseline; that is the
builder's lever for guards and champions, and it is visible on `look`
(revised 2026-10-12: an armed mob keeps power-unit damage scaled by its
weapon against a plain one of its level, 5.2, so a wielded weapon still
helps it now that weapons ride a flatter curve). A builder who writes only
`level: 8` gets a plain level-8 creature; one who writes `stats:
{strength: 14, intelligence: 6}` gets a brute; `health: 300` gets a
boss. Role tags are unnecessary because a role is just a set of
overrides, and the builder's `stat` command shows the computed
baseline beside every override.

**Mobs use equipment like a player.** A mob's wielded weapon replaces
its natural attack, and armor it wears replaces its natural armor,
through the same pipeline and the same hooks. This is already the
engine's shape (D15: one Character type). Two things follow:

- A guaranteed drop is simply an equipped item. The corpse holds what
  the mob was wearing and wielding, and nothing else needs a drop
  rule. Loot that is not worn (a key, a material) is a 5.3 concern.
- A player who looks at a mob sees its equipment, as they would a
  player's. What a mob drops is visible before the fight, which makes
  "kill the guard for his spear" a decision the world can hint at
  (6.1) rather than a lookup.

Mobs use abilities and effects the same way players do: a prototype
can list feats (7.2) and unlocked schools (6.1), and the same
`resolveCast` and pipeline apply. Whether mobs *choose* to cast is an
AI question for the engine's behaviour scripts, not a rules question;
the rules only need the mob to be able to.

**Resistances (decided 2026-10-12, content built the same day).** A
mob carries `resist` effects (4.8) like any other: a skeleton resists
pierce, a fire elemental is immune to fire and weak to cold. Full
immunity is a builder's choice for named creatures and bosses, and is
visible before the fight (4.8).

The shipped world gives 269 creatures a profile by what they are, each
a few graded entries:

| Archetype | Resists | Weak to |
|---|---|---|
| skeleton | poison 0.75, pierce 0.5, cold 0.5, slash 0.25 | blunt 0.25, holy 0.5 |
| corporeal undead (zombie, ghoul, mummy) | poison 0.75, drain 0.5, cold 0.25 | fire 0.25, holy 0.5 |
| ghost | poison 0.75, physical 0.5, drain 0.5, cold 0.5 | holy 0.5 |
| wight, White Walker | cold 0.75, poison 0.75, drain 0.5 | fire 0.5 |
| vampire | drain 0.5, cold 0.5, poison 0.5 | fire 0.25, holy 0.5 |
| demon | unholy 0.5, fire 0.5, poison 0.25 | holy 0.5 |
| celestial | holy 0.75 | unholy 0.5 |
| construct, droid | poison 0.75, drain 0.75, psionic 0.75 | lightning 0.25 |
| fire creature, red dragon | fire 0.75 (a dragon also physical 0.25) | cold 0.5 (a dragon 0.25) |
| frost creature | cold 0.75 | fire 0.5 |
| plant | poison 0.5, blunt 0.25, pierce 0.25 | fire 0.5 |
| ooze | acid 0.75, blunt 0.5, psionic 0.5, pierce 0.25 | |
| venomous beast | poison 0.5 | |
| psionic being (Protoss, the Mule) | psionic 0.5 | |
| bug (Klendathu, the Zerg) | acid 0.5, poison 0.25 | |

Named creatures and bosses hold their archetype's first resistance at
immunity: Strahd and his brides cannot be drained, the Butcher and Baal
are immune to unholy, the Red Dragon and Durin's Bane to fire.
Individuals whose source gives them a twist have their own lines
(Diablo resists lightning, Mephisto and Duriel cold, the Kurast
council each its element, the firebat fire, Twinrova fire and cold,
Death is immune to drain). No creature is immune to the physical
family or to `all`: a fighter can always hurt anything, if slowly. A
test holds the content to both rules.

Consequences: a `mobBaseline(prototype)` hook alongside
`itemBaseline`, same contract. Mob prototypes gain `baseline`
overrides for stats, health, natural attack, and natural armor, plus
optional `feats`, `schools`, and `xp` (7.1). `look` at a mob renders
its equipment.

### 8.1 Aggression

**Status: decided 2026-10-11.** Asked for by the designer: some mobs
should attack on sight, as their descriptions already say they would.

**Thesis.** ROM's aggressive flag: an aggressive mob attacks anyone it
can see in its room. The wolves hunt, the tomb's guardians guard, and
a dangerous place feels dangerous.

**Antithesis.** Unbounded, it punishes the wrong people. A level-3
character who takes a wrong turn into a level-15 area dies before it
can read the room, which teaches nothing; and a level-40 character
crossing a starter area is stopped every few steps by rats it kills in
one swing, which is only a chore.

**Synthesis.** An aggressive mob attacks only players whose level lies
in its *aggro range*. By default the range runs from 4 levels below
the mob to 5 above it, so a mob leaves alone a player it would simply
kill and a player who has outgrown it; a builder may state either end
(`aggro: {min, max}`) where the story wants otherwise, such as a
dragon that attacks anyone, or a bandit who only robs the weak. The
mob must see its prey (invisibility and hiding work, and sight that
sees through them works for mobs too, and a sneaking player is passed by unless the mob sees the hidden, 7.8); it never attacks in a safe
room, never attacks an admin or a link-dead character, and picks one
player a round while it is not fighting. The fight starts with a
warning and the mob's first swings come the next round, so a player
has a moment to flee. A player above the range can still start the
fight.

Consequences: the `aggressive` flag and the optional `aggro` field on
mob prototypes; `mobBaseline` returns `aggro`, defaulting either end
the prototype leaves out (`AGGRO` in the rules); `stat` on a mob shows
the range. Which mobs carry the flag is content, decided area by area
from their descriptions.

One consequence the default range had (story 1022, 2026-10-12): it made
the hardest prizes easiest at the lowest levels. A level-1 character
could walk past level-16 to 18 guards to the Evil god's altar, because
it was too weak for them to notice, and take the god's favour there.
So a mob that guards something meant to be earned, a god's offering or
the way to a Tower's Test, states `aggro: {min: 1}`: being far under it
is no pass. The rest of the world keeps the default.
