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

### Hook points (implemented; milestone 8 combat core, 2026-09-24)

Each hook is a global function in `data/scripts/*.js`. Inputs are
read-only snapshots; outputs are plain objects the engine applies. The
live rules are `data/scripts/rules.js`. Hooks marked optional may be
left out; the engine then uses a quiet default.

| Hook | Called when | Inputs | Returns |
|---|---|---|---|
| `resolveAttack` | each swing (the engine's swing meter decides when) | attacker, defender, weapon (null if unarmed), round | `{hit, damage, crit, verb, stage}`; `stage` is `dodge`, `block`, or `miss` when `hit` is false |
| `derivedStats` | login, spawn, equipment change, level, effect change, script reload | character | `{healthMax, manaMax, speed}`; `speed` is swings per round, fractional allowed |
| `onTick` | once per round for every character | character | `{healthDelta, manaDelta, skills:{id: rating}}` |
| `xpForKill` | a player kills a mob | killer, victim | integer |
| `xpToLevel` | after any experience gain, and on death | level | integer (total xp needed to reach it) |
| `onLevel` | a character gains a level | character, new level | `{statPoints, featPicks, statDeltas:{}, message}` |
| `onCreate` (optional) | a character's first login, or one with no stats | character | `{stats:{}, statPoints, featPicks, message}` |
| `deathRules` (optional) | every death | | `{xpFraction, xpLevelCap, corpseRounds, respawnHealth}` |
| `itemBaseline` (optional) | content load, script reload | item prototype as stated | `{weapon:{damage, spread, speed, hands, kind, verb}, armor:{defense, spread}}` |
| `mobBaseline` (optional) | content load, script reload | mob prototype as stated | `{stats:{}, health, xp, attack:{...}, armor:{...}}` |
| `featList` (optional) | the `feat` command, level-up, `score` | | `[{id, name, level, description, requires:[ids], effect:{kind, params}}]` |
| `standardKit` (optional) | `simulate fighter:N` | level | `[{name, type, slot, baseline, weapon, armor}]`, built into resolved prototypes at that level |
| `skillList` (optional) | typing a skill's name, `skills`, `practice`, level-up | | `[{id, name, level, passive, target, cooldown, start, innate, price, requires:[{material, count}], description}]` |
| `moneyFor` (optional) | a mob dies | victim | silver the corpse holds; the prototype's `silver` overrides |
| `questRules` (optional) | `quest request`, and when a quest ends | player | `{levelBand, rounds, cooldown, quitCooldown}`, the band in levels and the timers in rounds (7.6) |
| `questReward` (optional) | a quest is drawn | player, quest `{kind, target, name, level, area, room, rounds}` | `{points, silver, xp, message}`, shown on offer and paid on completion |
| `useSkill` | an active skill is used | user, target or null, skill, the skill's effect (with its rating in `state`) | `{ok, message, hit, stage, damage, verb, effects:[{on, kind, params, rounds}], skills:{id: rating}}` |
| `spellList` (optional) | `cast`, `spells` | | `[{id, name, branch, school, deity, castRounds, interruptOnDamage, cooldown, materials:[{material, count}], target, save, saveEffect, description}]` |
| `resolveCast` | a cast completes | caster, targets (array of views), spell | `{ok, message, consume:[item ids], targets:[{index, damage, heal, saved, negated, effects:[{kind, params, rounds}], message}], casterEffects:[...]}` |

### What scripts see

- Character: `{name, level, xp, stats:{}, statPoints, featPoints, silver, health,
  healthMax, mana, manaMax, speed, equipment:{slot: item}, effects:[],
  schools:[], deity, isPlayer, fighting, group:[brief], enemies:[brief],
  casting:{spell, rounds} or absent, cooldowns:{id: rounds},
  room:{vnum,name,area}}`, plus for mobs `vnum`, `flags`, and
  `mob:{vnum, flags, health, xp, attack, armor, effects}` carrying the
  prototype's resolved natural numbers. A brief is `{name, level,
  health, healthMax, isPlayer}`, so views do not recurse. `stats` is whatever the rules put
  there; the engine does not define the stat set.
- Item: `{id, vnum, name, type, slot, level, baseline, weight, value,
  flags, weapon:{damage, spread, speed, hands, kind, verb},
  armor:{defense, spread}, effects:[], mods:{}}`, plus `material` and
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
  `speed`; every whole point is one `resolveAttack` call.
- Effect lifetimes (5.4): timed effects count down once per round and
  are dropped at zero; `derivedStats` is re-run when a list changes.
- Death (4.6): the corpse (a container holding everything carried and
  worn, decaying after `corpseRounds`), the experience loss with its cap
  and floor, respawn at the start room at `respawnHealth`.
- Stat points and `train`: banked from `onLevel` and `onCreate`, spent
  one at a time on any stat the character has.
- Feat picks and `feat`: banked the same way; `feat` lists every feat
  with its status (learned, available, needs level N, needs another
  feat) and `feat <name>` spends a pick. A learned feat is a permanent
  effect whose params carry `feat: <id>`, so the rules see it like any
  other effect and stacking follows the kind.
- Baseline resolution: `itemBaseline` and `mobBaseline` run for every
  prototype at load and after every successful script reload; a live
  mob keeps the base stats it spawned with but its derived values move.
- Casting (6.4): one cast in progress per character with rounds left;
  `castRounds` 0 resolves on the command; movement always cancels;
  damage cancels when the spell says so; materials leave the inventory
  when the cast begins; cooldowns count down per round. Targets are
  built by the engine from the spell's `target`: single (named or the
  current target), ally (named or self), self, group (members here),
  area (everyone here not in the caster's group). A hostile cast starts
  the fight. Access: `consume <totem>` adds its school; `sacrifice
  <item>` in a room whose `temple` matches the item's `sacrifice` makes
  the character that god's apostle (one god at a time).
- Groups (4.7): `follow`, `group`, `gtell`, `assist`; followers move with
  the leader; grouped players here auto-assist when a member's fight
  starts; a mob flagged `assist` joins a mob of its kind; `kill` on a new
  target switches; a fighter whose target is gone turns on whoever is
  still fighting it; every grouped player here earns a kill's experience
  as if alone; a room flagged `safe` forbids fighting and hostile casts.

### Time

- A tick is `timing.tick_ms` (100 ms). Commands are processed once per tick.
- A round is `timing.round_ms` (2000 ms as of 2026-09-24, down from 3 s).
  Combat, regeneration, and effect durations are counted in rounds. The
  field is in milliseconds so fractional seconds can be tried in
  playtesting; it must be at least one tick.

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
is the progression curve made visible. Stats *multiply* and are the
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
roughly ten fights with rests between them, each fight lasting fifteen
to twenty seconds, which is eight to ten rounds at two seconds a round. A longer delve is several outings
chained by a player who chooses not to return to town, which is exactly
the attrition risk 2.3 wants. Session-scale goals (a level, a piece of
gear) span a few outings; career-scale goals span many sessions.

Consequences: 4.5's table is filled from these numbers. Regeneration
(`onTick`) is tuned so resting to full between fights takes about thirty
seconds, long enough to be a choice and short enough not to be a chore.
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
two jobs when milestone 8 starts is merged into its neighbour. The
table is the decision; the names are the flexibility.

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
| Charisma | what NPCs tell you: hint depth in discovery (6.1) | prices, and group benefits (2.4: an aura effect on allies) | |

Intelligence and Wisdom split the way D&D splits them: arcane power
against divine power (6.1), so the two are told apart by which branch
of magic a character pursues rather than by class. Wisdom's optional
third job, narrowing the stat band, is a proposal: it fits the name but
means one stat modifies the randomness of the others, and it should be
adopted only if the simulator shows it is felt. Charisma as the
discovery stat is the one that makes it not a
dump stat here specifically: in a game where power comes from finding
totems, the stat that makes NPCs talk is a power stat. If discovery
turns out to be authored so that Charisma does not matter to it,
Charisma is the first merge candidate.

Mobs use the same six with prototype values; the builder sets what is
needed and the rest default. A mob's stats are how "brute" or "caster"
is expressed (8), not a separate role system.

Consequences: 4.3's open questions close (dodge from Dexterity, block
from Strength, `D` from Constitution). 4.4 honours Dexterity as the
speed multiplier. `derivedStats` reads all six.

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
| Baseline stat | 10 in each of the six | D&D-familiar; multiplier of exactly 1 |
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
| Default weapon and armor spread | 0.2 | the value the variance tables were computed at |

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
two flat numbers cannot express. Armor gets the same treatment: a
defense number and a spread.

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

Consequences: `resolveAttack` gains nothing; `weapon.spread` and
`armor.spread` are added to the item view (section 9). Damage stays
positive after a low draw; the floor is zero, not negative. 4.3 should
choose percentage reduction so that spread on armor is expressed in the
same units as spread on weapons. The simulator should report the
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

`D` is the sum over worn armor of each piece's draw from its spread
(4.1), multiplied by the defending stat's multiplier. `K` is a unit,
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
gives Dexterity its second job as promised), plus any additive swings
from effects. The additive part is where ROM's second and third attack
live: each is a feat (7.2) that adds one full swing per round to the
meter, level-locked, deterministic, no roll. A half-swing feat (one
extra swing every other round) is equally expressible and is a
plausible lower-level rung.

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
`flee`, an item. Dual wield is open and is best treated as a feat that
lets an off-hand weapon add its speed to the meter at a penalty.

Consequences: `weapon.speed` on the item and in the view (section 9),
default 1. `derivedStats` returns `attacksPerRound` as the total speed
and the engine's combat loop keeps the meter, or the rules keep it in
effect state; the leaning is the engine, since the meter is a lifetime
concern like an effect's rounds. Dexterity multiplies speed. Feats
`second attack` and `third attack` are `prepare`-phase effects adding
one swing each.

### 4.5 Pacing targets

**Status: leaning (2026-09-23).** Numbers exist only to hit these. Set
them first, then let the simulator tell you whether a formula does. The
values below follow from 2.3 and 2.5 and are the first thing `simulate`
should be checked against.

| Target | Value |
|---|---|
| Rounds for an even fight (equal level, standard gear) | 8 to 10 (15 to 20 seconds) |
| Health remaining after an even fight at full health | 50 to 70 percent |
| Win rate at +1 level, +3 levels, +5 levels | 80, 40, under 10 percent |
| Rounds to recover from a fight to full | about 15 (30 seconds) |
| Fights per outing before returning to town | about 10 |
| Even fights chained with no rest before death is likely | 3 to 4 |
| Materials consumed per outing when magic is used freely | to be set with 6.2 |
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
  target auto-attacks one of them and may switch. Mobs can gang up, and
  a mob may assist another of its kind or its group (a builder flag).
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
Logged in section 9.

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
- Armor: `defense` and `spread`, plus `slot`.
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
`defense` and `spread`. An item prototype names its level and its
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

| Baseline | Total `D` at level, across all slots | Spread | Notes |
|---|---|---|---|
| `medium` | `3 * growth` | 0.2 | the reference: 13 percent reduction against an attacker of the same level, at every level |
| `light` | 0.7 of medium | 0.1 | carries an intrinsic effect raising dodge |
| `heavy` | 1.3 of medium | 0.3 | carries an intrinsic effect lowering dodge |

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
computes them (D15, D16). Section 9 lists it.

### 5.3 Rarity and drops

**Status: open.** Rephrased in terms of 5.4. Rarity, if it exists, is a
count or quality of effects. A "random affix" is an effect rolled onto
an instance at spawn. A fixed item is a prototype with intrinsic
effects. Where items come from (drops, shops, crafting) is untouched.

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
widening, listed and costed in section 9, not a feature of the effect
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
regression set of fights. The first milestone-8 target should be the
pipeline with two or three kinds, not a library.

**Open within 5.4:**

- Stacking: two effects of the same kind on the same character. Leaning
  is that stacking is a property of the kind (`stat` adds, `crit` takes
  the larger, `roll twice` does not stack), decided per kind in its
  code, not a global rule.
- A cap on effects per item: none, per 2.2. Rarity (5.3) may make the
  count expensive rather than bounded.
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
- **Quality.** On success the item is the recipe's item at the recipe's
  level, resolved through the baseline like any other (5.2). The rating
  can add: a higher rating gives a chance at a *fine* result, which
  carries an intrinsic effect the recipe names, or a level above the
  recipe's. How much is a starting value for playtest.
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

**Synthesis (decided 2026-09-25, revised the same day, design only).**
Any item can be enchanted. Each item carries a fixed number of
*enchantment slots*, set on its prototype, and that number is where
rarity lives: most found gear has none or one, good gear has a few, and
the rare and deterministic items of 6.2 have many. There is no global
cap on slots; a builder may make an item with twenty. Balance is done
per item, later, by the count.

An enchantment is an attempt, like a craft: it names materials, it can
fail, and success fills one slot with one *permanent* effect (5.4), the
applied-effect storage the engine already persists. Failure consumes
the materials, and has a small chance of destroying the item, a chance
that grows with every enchantment already on it. So the first
enchantment on a plain sword is cheap and safe, and the twelfth on a
sword with eleven is a gamble with something irreplaceable.

- **Enchantments** are content: each names its materials, the effect it
  attaches, and its difficulty. Basic ones come with the enchanting
  skill; better ones are found (6.1's discovery rule).
- **Difficulty rises with the count.** With `n` effects already on the
  item, the attempt is harder than at zero, and on failure the chance
  the item is destroyed is small at zero and rises with `n`. Both are
  asymptotic, never certain, the shape the document uses everywhere.
- **Powerful items are rare by construction.** An item's ceiling is its
  slot count, so an item that could become extraordinary is one the
  world places rarely and guards well, and filling it means surviving a
  rising risk of losing it. The unbounded top end exists, and almost
  nobody reaches it.
- **Enchanting is a skill** (7.4) with a rating that lowers the failure
  chance; open, leaning one enchanting skill shared across crafts.
- **Materials** are the one pool. The effect decides the cost: a stat
  effect wants common things, a crit or attack effect wants rare ones,
  and the deterministic drops are what the last slots on the best items
  are for.
- **Crafted items** (5.5) are ordinary items with slots like any other;
  a fine result from a high crafting rating may carry an extra slot,
  which is one reason to craft.

Starting formulas, arbitrary in the sense of 3.3:

    success  = rating / (rating + difficulty + n * step)
    destroyed on failure = n / (n + R)

with `step` and `R` parameters. At `R` 10, the first failure never
destroys, the fifth destroys one time in three, the twentieth two times
in three.

Consequences for the engine (both sections, section 9): a `recipeList`
and `enchantList` from the rules with materials, difficulty, station,
and result; `craft` and `enchant` commands that take an attempt through
`resolveCraft` and `resolveEnchant` hooks and apply the result, which
for an enchant may be "destroyed"; a `slots` field on item prototypes
and a `crafted` mark on instances, persisted; station flags on rooms;
`look` shows slots used and free; `describeEffect` already covers what
an enchantment does.

---

## 6. Magic

Worked dialectically on 2026-09-24. The shape is set by three
commitments from the designer: any character can potentially use magic,
each school of magic must be discovered and unlocked through play, and
casting consumes gathered materials. Magic is meant to be very strong
and to carry real costs. Everything below follows from those.

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
milestone 7 tooling should make it easy to see which totems exist and
where their hints are. No dependency on a quest system.

### 6.2 Casting consumes gathered materials

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

### 6.3 Magic is very strong

**Thesis.** Magic sits at parity with weapons so that casters and
fighters are balanced against each other.

**Antithesis.** There are no casters and fighters (7.3); every character
can potentially cast. Parity would make the discovery and the materials a
tax on an effect you could have got from a sword. If magic costs more
than a swing it has to do more than a swing.

**Synthesis (leaning).** Per use, magic is the strongest tool in the
game. The material cost and discovered access are what balance it, so
its output is not held back to weapon levels. In terms of 2.1, casting
is the decision a fight offers: when to spend materials, on which fight.
An even fight at level is winnable with steel alone; magic is what makes
a fight above level winnable, and spending it on an even fight is a
choice to trade materials for safety and speed.

Consequences: the simulator must run fights with and without material
use, and report material consumption per fight, since that is the cost
being balanced. 4.5 gains a row for it.

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
difference against a two-second round. Two concepts for one experience
is a cost with no return. **Synthesis (leaning):** one number,
`castRounds`. Zero means the spell resolves on the tick it is typed
and does not take the round's action (2.1), so it can be chained with
another spell or with an attack in the same round. One means it takes
this round's action and completes at the round boundary. N means N
rounds of casting. A caster has at most one spell in progress; starting
another cancels it. Free spells are limited by their material cost and
by an optional per-spell cooldown, not by the round. If a real
difference between instant and zero-round appears in play, the doc
comes back here.

**Interruption.** Movement always interrupts a cast in progress. Damage
interrupts only spells that say so (`interruptOnDamage`). Leaning:
materials are committed when the cast begins, so an interrupted cast
loses them. That is the consequence 6.3 asks magic to carry, and it is
why a long cast is a real decision in a fight. Open: whether an
interrupted cast also triggers the cooldown.

**Saves.** A target resists a spell by a saving throw in the Dungeons
and Dragons manner, and the spell names which: *reflex* (Dexterity),
*fortitude* (Constitution), or *will* (Wisdom). A successful save does
what the spell says: negates it, or halves it. Some spells allow no
save. Note against 4.2: a save is a binary roll, the kind of variance
4.2 keeps out of the weapon fight. It is admitted here on purpose,
because a spell is rare, costs materials, and is meant to be strong
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
`interruptOnDamage`, `cooldown`, `materials` (kind and count),
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
multiple of the standard weapon's per-swing damage at the caster's
level (5.2), before the spell's stat and level scaling, so the list
stays true when the curve moves. Durations are rounds.

**Materials pool.** Rarity sets how often farming yields it; the last
tier is never random.

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

| Spell | School | Cast | Interrupt | Cooldown | Materials | Target | Save | Effect |
|---|---|---|---|---|---|---|---|---|
| Firebolt | evocation | 1 | yes | 0 | 1 ash | single | reflex, half | damage 2.0x |
| Ward | abjuration | 0 | no | 10 | 1 salt | self or one ally | none | +defense equal to medium armor at level, 10 rounds |
| Acid Splash | conjuration | 1 | yes | 0 | 1 salt, 1 ash | single | fortitude, half | damage 1.5x, then 0.3x per round for 3 rounds |
| Foresight | divination | 0 | no | 20 | 1 nightshade | self | none | +10 percent dodge, 10 rounds |
| Daze | enchantment | 1 | yes | 5 | 1 bone dust | single | will, negate | target speed halved, 3 rounds |
| Blur | illusion | 1 | no | 15 | 1 quicksilver | self | none | +15 percent dodge, 5 rounds |
| Drain | necromancy | 1 | yes | 3 | 1 bone dust | single | fortitude, half | damage 1.5x; caster heals half of it |
| Haste | transmutation | 2 | yes | 20 | 2 quicksilver | self | none | +1 swing per round, 5 rounds |
| Fireball | evocation | 2 | yes | 8 | 2 ash, 1 star iron | area | reflex, half | damage 1.5x to each enemy |

**Divine**, one per god, plus a second for Good since healing is the
thing a group (4.7) needs first.

| Spell | Deity | Cast | Interrupt | Cooldown | Materials | Target | Save | Effect |
|---|---|---|---|---|---|---|---|---|
| Mend | Good | 1 | yes | 0 | 1 tallow | self or one ally | none | heals 30 percent of the target's maximum |
| Sanctuary | Good | 2 | yes | 30 | 1 salt, 1 heartwood | group | none | damage taken halved, 5 rounds |
| Stillness | Neutral | 1 | yes | 10 | 1 salt, 1 tallow | area | will, negate | every enemy's speed halved, 2 rounds |
| Blight | Evil | 1 | yes | 6 | 1 bone dust, 1 nightshade | area | fortitude, half | 0.4x per round to each enemy, 5 rounds |

Scaling for all of the above: the branch stat (Intelligence or Wisdom)
times the level multiplier, per 6.4. Fireball and Blight are the first
area spells and the reason 4.7 lands first. Sanctuary is the first
spell to need a group. Haste is the first to touch the swing meter
through an effect, which the `attacks` kind already supports.

What the list deliberately leaves out: anything that creates a mob or
an item (conjuration proper), anything that moves a character, and
anything global. Each needs an engine return channel that does not yet
exist and is listed in section 9.

---

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
   grants stat points the player spends on any of the six (decided
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
   to the "skills" question in section 9 and need no new system: items
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
`level` field and the pick command enforces it. 3.3's per-level
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

| Skill | Level | Kind | Cooldown | Start | Taught | What the rating scales |
|---|---|---|---|---|---|---|
| Kick | 1 | action, single target | 2 | 30 | innate | damage: 1.2x a standard swing at full skill, no weapon needed |
| Bash | 3 | action, single target | 4 | 25 | 3 gold | damage (0.6x a swing) and the chance the target loses its next round of swings |
| Twin Strike | 5 | passive | | 20 | 15 gold | the fraction of an extra swing per round; 100 is a full second swing |
| Rend | 3 | action, single target | 3 | 30 | 1 gold and two wolf fangs | damage (0.8x a swing) and a bleed of a quarter of it for three rounds |
| Riposte | 4 | passive | | 20 | 5 gold | block chance with a weapon in hand, up to 20 percent at full skill |

Twin Strike is the first of a chain: Triple Strike and Quad Strike
follow at higher levels, the last for special cases, each a further
swing. Improvement: each use (each round in a fight, for passives) has
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

Sources: mobs carry coin, about five silver per level with a wide
spread from the rules' `moneyFor` hook, or a stated amount on the
prototype. Sinks: trainers (7.4). Open: shops, which are deferred past
milestone 8 in `MILESTONES.md`, and whether anything else costs coin.

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
levels either way, fifteen minutes at a two-second round, five points
plus the target's level.

Sinks are content: the herald in the Long Hall sells starter and
mid-level gear. Open: whether quest points should also buy services
(a resurrection, a rename), and whether a vendor's stock should rotate.

---

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
attack that is a `standard` weapon at its level scaled by `mobDamage`
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
builder's lever for guards and champions, and it is visible on `look`. A builder who writes only
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

Consequences: a `mobBaseline(prototype)` hook alongside
`itemBaseline`, same contract. Mob prototypes gain `baseline`
overrides for stats, health, natural attack, and natural armor, plus
optional `feats`, `schools`, and `xp` (7.1). `look` at a mob renders
its equipment. Section 9 lists the hook.

---

## 9. Open questions log

Anything not yet placed in a section above.

- **Groups and many-to-many combat** (4.7): target switching, the set of
  attackers per character, `Group`, experience split, `follow`,
  `group`, `assist`, `gtell`, mob assist flag, safe-room flag, and
  `group` and `enemies` in the views. Engine work, no rules decision
  outstanding; should land before `resolveCast` because spells target
  sets.
- **Before magic is built.** Decided 2026-09-24: cast time, interruption,
  saves, scaling, targets, schools, deities, sacrifice (6.1, 6.4). Still
  open: none. The material model is decided (6.2) and a first pool and
  spell list are proposed in 6.5 for the designer to edit. Engine work
  is listed above; 4.7 goes first.
- **Hint authoring.** 6.1 relies on the world carrying hints toward
  each totem. That is content, but it needs a builder-side view of which
  totems exist and which rooms and NPCs mention them, or hints will rot
  as areas change. Belongs in milestone 7.
- **Crafting and enchanting** (5.5, 5.6): `recipeList`, `enchantList`,
  `resolveCraft`, `resolveEnchant`, `craft` and `enchant` commands, a
  `slots` field on prototypes, a `crafted` mark on instances, station
  room flags. Design done; build after milestone 8.
- **Contract widenings still owed.** Done on 2026-09-24: item spread,
  speed, level, baseline, and effects in the view; mob natural attack,
  armor, health and xp overrides; the two baseline hooks; effects on
  characters and instances with persistence and lifetimes; stat points
  and `train`; `deathRules`; fractional speed. Still owed: `resolveCast`
  with the caster's unlocked schools and deities in the view, the
  `consume` return, a return channel for attaching effects and updating
  effect `state` (now also needed by 7.4's effectiveness ratings, which
  live in `state`), a skill-use command and hook (7.4), and the hooks
  effects will eventually want, each a separate widening: `onMove`,
  `onDamaged`, `onDeath`. Feat picks from `onLevel` and the `feat`
  command landed 2026-09-24.
- **Feats** (the designer's word, replacing "skills") are permanent
  effects with a minimum level, gained one or two per level by choice or
  from a trainer (7.2, leaning). No new system; the engine's share is a
  pick command, a banked pick count, and a `level` field on the effect. Item consumption is settled (section 1,
  D17) and lands with the `cast` command.
- **Round length.** Set to 2 s on 2026-09-24, and the config field is
  now `round_ms` so 1.5 s is a one-line change when playtesting wants
  it. The variance tables in 4.2 and 4.3 were computed at seven swings;
  at eight to ten rounds they are slightly conservative.
- **Mana** stays in the engine behind a flag and is not required by any
  rule (6.2). Remove from the contract only if the material system is
  confirmed after milestone 8.
