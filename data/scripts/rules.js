// Urth rules. The design is docs/RULES.md; this file is its first
// implementation, and every number in P is a starting value (RULES 3.3)
// meant to be changed in playtesting. Edits take effect on the next round
// without a restart. A syntax error keeps the previous version running and
// warns admins.
//
// Globals from the engine:
//   random.int(n)   random.float()   random.roll(count, sides)   log(...)
//
// Views (RULES section 1): characters carry name, level, xp, stats{},
// statPoints, health, healthMax, speed, offSpeed, equipment{slot: item}, effects[],
// isPlayer, fighting, and for mobs mob{vnum, flags, health, xp, attack,
// armor, effects}. Items carry vnum, name, type, slot, level, baseline,
// weapon{damage, spread, speed, hands, kind, verb}, armor{defense},
// effects[]. Effects are {kind, params, state, rounds}.

// ---------------------------------------------------------------------
// Parameters (RULES 3.3, 4.3, 4.5, 4.6, 7.1)
// ---------------------------------------------------------------------
var P = {
  // The round is timing.round_ms in config; keep this in step with it.
  // Fights, cooldowns, and effects are counted in rounds and run at its
  // pace; the timers below meant as wall-clock time are kept in seconds or
  // minutes and turned into rounds from it (after this object).
  roundSeconds: 3,      // ROM 2.4's combat round (decided 2026-10-11, from 2 s)
  statBaseline: 10,     // a stat of 10 multiplies by exactly 1
  statK: 0.05,          // multiplier per point above or below 10
  statBand: 0.10,       // ±10% jitter on every stat multiplier (4.2)
  levelK: 0.02,         // slight per-level multiplier (7.2)
  levelGrowth: 1.10,    // every baseline grows by this factor per level (RULES 5.2, decided 2026-09-24)
  weaponGrowth: 1.07,   // a weapon's own damage grows by this instead (5.2, decided 2026-10-12); what it carries makes up the rest
  healthBase: 50,       // health at level 1; grows by levelGrowth
  mobDamage: 0.45,      // a mob's natural attack as a fraction of a standard weapon (RULES 8)
  dodgeBase: 0.03,      // intrinsic dodge before stats and effects (4.3)
  blockBase: 0.0,       // nothing blocks without a shield or a feat
  K: 20,                // reduction = D / (D + K) at level 1; K is a unit, not a cap
  kGrowth: 1.0,         // K grows with the attacker's level as levelGrowth^((L-1)*kGrowth); at 1.0 a tier of armor reduces the same share at every level
  regenSeconds: 90,     // empty to full out of a fight, standing
  restRegen: 2,         // resting regenerates twice as fast (45 s)
  sleepRegen: 3,        // sleeping three times as fast (30 s)
  xpBase: 50, xpR: 1.15, // cost(N) = xpBase * N * xpR^(N-1)
  xpPerLevelKill: 10,   // an even kill is worth 10 * victim level
  deathXpFraction: 0.20, deathXpLevelCap: 0.50,
  corpseMinutes: 20,    // how long a corpse lies before it rots
  respawnHealth: 0.25,
  pointsAtCreation: 6, pointsPerLevel: 2,
  featsPerLevel: 1,     // feat picks per level (7.2: one or two)
  // Mana (RULES 6.2, revised 2026-10-11): the pool is manaBase plus
  // manaPerLevel a level, times the better of the Intelligence and Wisdom
  // multipliers, and refills from empty in manaRegenRounds standing out
  // of a fight (resting and sleeping speed it as they do health). In a
  // fight it comes back at manaFightRegen of that rate.
  manaBase: 30, manaPerLevel: 3, manaRegenSeconds: 90, manaFightRegen: 0,
  // Stamina (RULES 7.7): what skills spend, refilled within a fight. The
  // pool is staminaPool for everyone at every level, so costs keep their
  // weight as a character grows; it refills from empty in
  // staminaRegenRounds in a fight and staminaRestMult times faster out
  // of one (resting and sleeping faster still). The better of Strength
  // and Constitution speeds that by staminaStatK a point over the
  // baseline, half what a stat's multiplier gives. Heavy and plate
  // armor, by the share of the body they cover, and a heavy weapon slow
  // it. Empty, a character dodges exhaustedDodge less.
  staminaPool: 100, staminaRegenRounds: 10, staminaRestMult: 2, staminaStatK: 0.025,
  staminaHeavyArmor: 0.25, staminaPlateArmor: 0.4, staminaHeavyWeapon: 0.1,
  exhaustedDodge: 0.05,
  // A disarmed mob whose natural attack is a weapon (armedNatural) hits
  // for this share of it until the weapon is back in hand (RULES 7.4).
  disarmedNatural: 0.5,
  // Two weapons without the Dual Wield feat (RULES 4.4): every swing of
  // the main hand hits for dualMain of itself and every swing of the off
  // hand for dualOff, so the pair deals about what one weapon does and the
  // shield is gone. The feat lifts both to 1.
  dualMain: 0.6, dualOff: 0.4
};
// The engine warns at start when this disagrees with timing.round_ms.
function roundSeconds() { return P.roundSeconds; }

// Wall-clock timers in rounds of P.roundSeconds.
P.regenRounds = P.regenSeconds / P.roundSeconds;
P.manaRegenRounds = P.manaRegenSeconds / P.roundSeconds;
P.corpseRounds = Math.round(P.corpseMinutes * 60 / P.roundSeconds);

var STATS = ["strength", "dexterity", "constitution", "intelligence", "wisdom"];

// ---------------------------------------------------------------------
// Baselines by level (RULES 5.2, 8). An item or mob prototype names its
// level and baseline; anything the builder stated explicitly wins.
// ---------------------------------------------------------------------
// growth is the geometric level factor: 1 at level 1, levelGrowth^(L-1)
// after. A gap of N levels is the same ratio at every level, which is
// what lets one win-rate row hold everywhere (RULES 4.5).
function growth(L) { return Math.pow(P.levelGrowth, Math.max(0, (L || 1) - 1)); }

// weaponGrowth is the flatter curve a weapon's own damage rides (5.2,
// decided 2026-10-12): weaponGrowth^(L-1). The rest of a late weapon's
// damage is what it carries (added damage, procs: 5.3).
function weaponGrowth(L) { return Math.pow(P.weaponGrowth, Math.max(0, (L || 1) - 1)); }

// Every weapon baseline deals about the standard's damage per round
// (damage * speed near 6 at level 1); they differ in rhythm, spread, and
// the trait in BASELINE_TRAITS. hands is the default when an item states
// none.
var WEAPON_BASELINES = {
  standard: function (L) { return { damage: 6 * weaponGrowth(L), speed: 1.0, spread: 0.2, verb: "hit", kind: "slash" }; },
  dagger:   function (L) { return { damage: 3 * weaponGrowth(L), speed: 2.0, spread: 0.1, verb: "stab", kind: "pierce" }; },
  heavy:    function (L) { return { damage: 12 * weaponGrowth(L), speed: 0.5, spread: 0.4, verb: "smash", kind: "blunt" }; },
  unarmed:  function (L) { return { damage: 1.5 * weaponGrowth(L), speed: 1.0, spread: 0.2, verb: "punch", kind: "blunt" }; },
  axe:      function (L) { return { damage: 7.2 * weaponGrowth(L), speed: 0.8, spread: 0.35, verb: "hack", kind: "slash" }; },
  mace:     function (L) { return { damage: 6.6 * weaponGrowth(L), speed: 0.9, spread: 0.15, verb: "crush", kind: "blunt" }; },
  spear:    function (L) { return { damage: 5.4 * weaponGrowth(L), speed: 1.1, spread: 0.15, verb: "thrust", kind: "pierce" }; },
  flail:    function (L) { return { damage: 8.4 * weaponGrowth(L), speed: 0.7, spread: 0.5, verb: "flail", kind: "blunt" }; },
  whip:     function (L) { return { damage: 2.8 * weaponGrowth(L), speed: 2.1, spread: 0.2, verb: "lash", kind: "slash" }; },
  staff:    function (L) { return { damage: 5.4 * weaponGrowth(L), speed: 1.0, spread: 0.1, verb: "strike", kind: "blunt", hands: 2 }; }
};

// Armor baselines are a whole set's D; a piece takes its slot's share
// (SLOT_WEIGHT). Shields carry no slot weight, so the shield baselines
// state a piece's own number.
var ARMOR_BASELINES = {
  medium: function (L) { return { defense: 3 * growth(L) }; },
  light:  function (L) { return { defense: 0.7 * 3 * growth(L) }; },
  heavy:  function (L) { return { defense: 1.3 * 3 * growth(L) }; },
  cloth:  function (L) { return { defense: 0.4 * 3 * growth(L) }; },
  plate:  function (L) { return { defense: 1.6 * 3 * growth(L) }; },
  shield: function (L) { return { defense: 0.08 * 3 * growth(L), piece: true }; },
  tower:  function (L) { return { defense: 0.15 * 3 * growth(L), piece: true }; }
};

// BASELINE_TRAITS is what a family does beyond its numbers, read from the
// baseline an item names. Weapon traits hold while it is wielded. Armor
// traits scale with the set: each piece gives its slot's share, so a full
// set gives the whole amount; a shield gives all of its own.
//   crit    added to the chance of a double-damage crit
//   pierce  share of the defender's armor ignored
//   dodge   added to the wearer's dodge (negative lowers it)
//   block   a shield's block chance, when the shield states none itself
//   overShield  shields do not block it
//   slow    {chance, mult, rounds} on each hit
//   spell   added to spell power, as a share
var BASELINE_TRAITS = {
  axe:   { crit: 0.05 },
  mace:  { pierce: 0.25 },
  spear: { dodge: 0.03 },
  flail: { overShield: true },
  whip:  { slow: { chance: 0.10, mult: 0.75, rounds: 2 } },
  staff: { spell: 0.10 },
  cloth: { dodge: 0.05 },
  plate: { dodge: -0.05 },
  shield: { block: 0.10 },
  tower:  { block: 0.18, dodge: -0.03 }
};

function traitsOf(it) { return (it && BASELINE_TRAITS[it.baseline]) || {}; }

// wielded is the weapon c swings with, or null when unarmed or disarmed.
function wielded(c) {
  if (hasEffect(c, "disarmed")) return null;
  var it = c.equipment && c.equipment.wield;
  return it && it.weapon ? it : null;
}

// offhand is the second weapon c swings with (RULES 4.4), or null: none
// in the off hand, or disarmed.
function offhand(c) {
  if (hasEffect(c, "disarmed")) return null;
  var it = c.equipment && c.equipment.second;
  return it && it.weapon ? it : null;
}

// dualMult is what fighting with two weapons does to a swing of hand
// ("wield" or "second"): nothing with the Dual Wield feat or without a
// second weapon, else the untrained penalty on both hands.
function dualMult(c, hand) {
  if (!(c.equipment && c.equipment.second) || hasEffect(c, "dualWield")) return 1;
  return hand === "second" ? P.dualOff : P.dualMain;
}

// twoHanded reports whether a weapon item takes both hands.
function twoHanded(it) { return !!it && !!it.weapon && it.weapon.hands >= 2; }

function weaponTrait(c, field) { return num(traitsOf(wielded(c))[field]); }

// armorTrait sums an armor trait over what c wears: slot share for set
// pieces, the whole amount for a shield.
function armorTrait(c, field) {
  var total = 0, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (!it || it.type !== "armor") continue;
    var t = traitsOf(it)[field];
    if (!t) continue;
    total += num(t) * (it.slot === "shield" ? 1 : (SLOT_WEIGHT[it.slot] || 0));
  }
  return total;
}

function shield(c) { return c.equipment && c.equipment.shield ? c.equipment.shield : null; }

// shieldBlock is the block c's shield gives: its own block effects, or its
// baseline's when it states none, plus feats and Shield Block that need
// one. Zero without a shield.
function shieldBlock(c) {
  var sh = shield(c);
  if (!sh) return 0;
  var own = 0;
  if (sh.effects) for (var i = 0; i < sh.effects.length; i++) if (sh.effects[i].kind === "block") own += num(sh.effects[i].params.chance);
  if (!own) own = num(traitsOf(sh).block);
  return own + sumEffects(c, "shieldBlock", "chance") + 0.15 * skillRating(c, "shield_block") / 100;
}

function isDagger(it) {
  return !!it && (it.baseline === "dagger" || (it.weapon && it.weapon.kind === "pierce" && it.weapon.speed >= 1.5));
}

function isHeavy(it) {
  if (!it || !it.weapon) return false;
  return it.baseline === "heavy" || it.baseline === "axe" || it.baseline === "mace" || it.baseline === "flail" || it.weapon.hands >= 2;
}

// A set's total D is split across slots by these weights (5.2), which sum
// to one, so a full set at level N is the baseline D at level N. Slots
// not listed (shield, neck, finger, light, hold) carry no baseline
// defense; they are for effects, or for stated numbers.
var SLOT_WEIGHT = { body: 0.30, legs: 0.15, head: 0.10, arms: 0.10, feet: 0.08, hands: 0.07,
                    shoulders: 0.08, face: 0.04, belt: 0.04, wrist: 0.02 };

function itemBaseline(p) {
  var L = p.level || 1;
  var out = {};
  if (p.type === "weapon") {
    var w = p.weapon || {};
    var base = (WEAPON_BASELINES[p.baseline] || WEAPON_BASELINES.standard)(L);
    out.weapon = {
      damage: w.damage || base.damage,
      spread: w.spread || base.spread,
      speed:  w.speed  || base.speed,
      hands:  w.hands  || base.hands || 1,
      kind:   w.kind   || base.kind,
      verb:   w.verb   || base.verb
    };
  }
  if (p.type === "armor") {
    var a = p.armor || {};
    var ab = (ARMOR_BASELINES[p.baseline] || ARMOR_BASELINES.medium)(L);
    var weight = ab.piece ? 1 : (SLOT_WEIGHT[p.slot] || 0);
    out.armor = {
      defense: a.defense || ab.defense * weight,
      spread:  a.spread  || ab.spread
    };
  }
  if (p.effects && p.effects.length > 0) out.color = NAMED_COLOR;
  else if (p.material) out.color = materialColor(p.rarity);
  return out;
}

function mobBaseline(p) {
  var L = p.level || 1;
  var stats = {};
  for (var i = 0; i < STATS.length; i++) {
    stats[STATS[i]] = (p.stats && p.stats[STATS[i]]) || P.statBaseline;
  }
  var atk = p.attack || {};
  // A creature's natural attack is a level-appropriate weapon scaled by
  // mobDamage (RULES 8). A mirror fight can only end with the winner near
  // empty, so a level-N mob must hit softer than a level-N player in
  // standard gear for an even fight to end at the 4.5 health target.
  var base = { damage: unitAt(L) * P.mobDamage, spread: 0.2, speed: 1.0 };
  var arm = p.armor || {};
  var ab = ARMOR_BASELINES.medium(L);
  return {
    stats: stats,
    health: p.health || 0,
    xp: p.xp || 0,
    attack: {
      damage: atk.damage || base.damage,
      spread: atk.spread || base.spread,
      speed:  atk.speed  || base.speed,
      hands:  0, kind: atk.kind || "blunt",
      verb:   atk.verb || "hit"
    },
    armor: { defense: arm.defense || ab.defense },
    aggro: aggroRange(L, p.aggro)
  };
}

// AGGRO: an aggressive mob attacks on sight only players from `below`
// levels under it to `above` levels over it (RULES 8.1), so a weaker
// player is not run down by what would kill them and a much stronger one
// is not pestered. A prototype's stated aggro bounds win, end by end.
var AGGRO = { below: 4, above: 5 };

function aggroRange(L, stated) {
  stated = stated || {};
  return { min: stated.min || Math.max(1, L - AGGRO.below), max: stated.max || L + AGGRO.above };
}

// ---------------------------------------------------------------------
// Effects (RULES 5.4). A kind is a function of the pipeline context; the
// engine stores {kind, params, state, rounds} and never looks inside.
// Kinds in play so far: stat, crit, dodge, block, attacks, defense,
// baseDamage, baseDefense, protect, speedMult, dot, the power kinds below, and the combat kinds of
// the skills and feats: blind, disarmed, damage, powerAttack, critMult,
// shieldBlock, finesse, blindFighting, steady, opportunist, cooldownCut,
// unarmedMult, envenom, and the fighting styles' dualWield,
// offhandAttacks, and greatWeapon.
// The engine itself reads four kinds for visibility (world/visibility.go):
// invisible and hidden on a character or its gear make it unseen, and
// detectInvisible and detectHidden on the viewer see through them.
// ---------------------------------------------------------------------
// fn gets the item an effect is on as its second argument, or nothing
// for the character's own effects and its prototype's.
function eachEffect(c, fn) {
  var i, s;
  if (c.effects) for (i = 0; i < c.effects.length; i++) fn(c.effects[i]);
  if (c.mob && c.mob.effects) for (i = 0; i < c.mob.effects.length; i++) fn(c.mob.effects[i]);
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.effects) for (i = 0; i < it.effects.length; i++) fn(it.effects[i], it);
  }
}

function num(v) { return typeof v === "number" ? v : (parseFloat(v) || 0); }

// sumEffects adds params[field] over every effect of kind (stacking: add).
function sumEffects(c, kind, field) {
  var total = 0;
  eachEffect(c, function (e) { if (e.kind === kind) total += num(e.params[field]); });
  return total;
}

// bestEffect returns the effect of kind with the largest params[field]
// (stacking: take the larger).
function bestEffect(c, kind, field) {
  var best = null;
  eachEffect(c, function (e) {
    if (e.kind === kind && (best === null || num(e.params[field]) > num(best.params[field]))) best = e;
  });
  return best;
}

// hasEffect reports whether an item or character view carries kind.
function hasEffect(v, kind) {
  if (!v || !v.effects) return false;
  for (var i = 0; i < v.effects.length; i++) if (v.effects[i].kind === kind) return true;
  return false;
}

// sneaking reports whether a character's own Sneak is on (not gear's).
function sneaking(c) {
  if (!c.effects) return false;
  for (var i = 0; i < c.effects.length; i++) {
    var e = c.effects[i];
    if (e.kind === "sneak" && e.params && e.params.source === "sneak") return true;
  }
  return false;
}

// ---------------------------------------------------------------------
// Power kinds (the outfit set, cmdOutfit). Their sizes are in units: one
// unit is a standard weapon's swing at the owner's level, so a kind means
// the same at level 1 and level 50 and nothing runs away with the curve.
// Stacking, per kind (5.4):
//   attuned    the item's weapon and armor numbers follow its wearer's level
//   proc       each rolls on its own: {chance, power, verb}, on every hit
//   added      adds, each part on its own: {kind, power} more damage on every hit (5.3)
//   ignite     a burn per hit, stacking up to maxStacks: {power, rounds, maxStacks}
//   chill      slows the target; never stacks: {chance, mult, rounds}
//   leech      adds, capped at LIMITS.leech: {fraction} of damage dealt healed
//   thorns     adds, capped: {fraction} of a landed hit returned, {verb}
//   soak       adds: {power} units off every hit taken, never below LIMITS.soakFloor
//   lastStand  multiplies: take {mult} damage while below {threshold} health
//   execute    takes the larger: deal {mult} damage to foes below {threshold}
//   regen      adds, capped: {fraction} of maximum health back every round
//   aura       each strikes on its own: {power, verb} to every foe, every round
//   resist     adds, a player's total limited: {kind, amount} (RULES 4.8)
// proc, aura, and thorns take an optional {kind} (4.8): magic, pierce for
// thorns, when none is stated. On an item, the powers of proc, added,
// ignite, envenom, and aura are units at the item's level, or at
// {level} when the roll set one (effectUnit).
// ---------------------------------------------------------------------
var LIMITS = { leech: 0.5, thorns: 1.0, soakFloor: 0.4, regen: 0.10, resist: 0.75 };

// ---------------------------------------------------------------------
// Damage kinds and resistance (RULES 4.8). Every part of a hit has a kind;
// a resist effect names a kind, a family, or "all". Amounts add, kind and
// family alike; a part is multiplied by 1 - total, so 1 is immunity and a
// negative total a vulnerability. A player's total stops at LIMITS.resist:
// immunity is for mobs.
// ---------------------------------------------------------------------
var KIND_FAMILIES = {
  physical: ["slash", "pierce", "blunt", "bite", "claw"],
  elemental: ["fire", "cold", "lightning", "acid", "poison"],
  other: ["magic", "psionic", "drain", "holy", "unholy"]
};

// damageKinds is every kind content may name, for `urth check`.
function damageKinds() {
  return KIND_FAMILIES.physical.concat(KIND_FAMILIES.elemental, KIND_FAMILIES.other);
}

function kindFamily(kind) {
  for (var f in KIND_FAMILIES) if (KIND_FAMILIES[f].indexOf(kind) >= 0) return f;
  return "";
}

// resistance is c's total resistance to kind.
function resistance(c, kind) {
  if (!kind) return 0;
  var fam = kindFamily(kind), r = 0;
  eachEffect(c, function (e) {
    if (e.kind !== "resist") return;
    var k = e.params.kind;
    if (k === kind || k === "all" || (fam && k === fam)) r += num(e.params.amount);
  });
  return c.mob ? r : Math.min(r, LIMITS.resist);
}

// resisted is what is left of dmg of kind after c's resistance.
function resisted(c, kind, dmg) { return dmg * Math.max(0, 1 - resistance(c, kind)); }

// immune reports whether c takes nothing from kind.
function immune(c, kind) { return resistance(c, kind) >= 1; }

// resistDamage is the engine's hook for a strike it lands on several
// targets itself (an aura): the damage left on target, after its
// resistance to kind.
function resistDamage(target, kind, damage) { return Math.round(resisted(target, kind, damage)); }

function unit(c) { return unitAt(c.level); }

// unitAt is the power unit at level L, 6 * growth(L): what a standard
// weapon's swing was before weapons took the flatter curve (5.2).
function unitAt(L) { return 6 * growth(L); }

// effectUnit is the unit an effect of c's is sized in (5.3). One on an
// item is sized at the item's level: the level it was rolled at, if the
// roll says, or the prototype's. So old gear falls behind as its weapon
// does. An attuned item's, and c's own, are sized at c's level.
function effectUnit(c, e, it) {
  if (!it || hasEffect(it, "attuned")) return unit(c);
  return unitAt(num(e.params.level) || it.level || c.level);
}

// weaponShare is the part of the power unit a weapon's own damage
// carries at level L (5.2): all of it at level 1, a fifth at 60.
function weaponShare(L) { return weaponGrowth(L) / growth(L); }

// enhanced is an item's own number (a weapon's damage, a piece's
// defense) raised by the item's effects of kind: each adds its fraction
// of the number, at least 1 (5.3). Nothing is added to a zero.
function enhanced(it, base, kind) {
  base = num(base);
  if (base <= 0 || !it || !it.effects) return base;
  var add = 0;
  for (var i = 0; i < it.effects.length; i++) {
    var e = it.effects[i];
    if (e.kind === kind) add += Math.max(1, base * num(e.params.fraction));
  }
  return base + add;
}

// weaponDamage and armorDefense are an item's own damage and defense
// with its honed and reinforced affixes.
function weaponDamage(it) { return enhanced(it, it.weapon.damage, "baseDamage"); }
function armorDefense(it) { return enhanced(it, it.armor.defense, "baseDefense"); }

// attunedScale is how much an attuned item's numbers grow on c: the
// baseline ratio between c's level and the item's own, on the weapon
// curve for a weapon and the power curve for armor.
function attunedScale(it, c) {
  if (!hasEffect(it, "attuned")) return 1;
  if (it.type === "weapon") return weaponGrowth(c.level) / weaponGrowth(it.level);
  return growth(c.level) / growth(it.level);
}

function capped(c, kind, field, cap) { return Math.min(cap, Math.max(0, sumEffects(c, kind, field))); }

// burningStacks counts the dots from source (ignite, venom) already on c.
function burningStacks(c, source) {
  var n = 0;
  source = source || "ignite";
  if (c.effects) for (var i = 0; i < c.effects.length; i++) {
    var e = c.effects[i];
    if (e.kind === "dot" && e.params.source === source) n++;
  }
  return n;
}

// onHit is the onhit phase (5.4) of a swing that landed for dmg: the
// attacker's procs, burns, chills, and leech, and the defender's thorns.
function onHit(att, def, dmg, out) {
  var procs = [], effects = [], dealt = dmg;
  eachEffect(att, function (e, it) {
    var p = e.params;
    if (e.kind === "proc" && random.float() < (p.chance === undefined ? 1 : num(p.chance))) {
      var kind = p.kind || "magic";
      if (immune(def, kind)) { procs.push({ verb: p.verb || "magic", damage: 0, kind: kind, resisted: true }); return; }
      var d = Math.round(resisted(def, kind, spreadRoll(effectUnit(att, e, it) * num(p.power), 0.2) * protectMult(def) * lastStandMult(def)));
      if (d > 0) { procs.push({ verb: p.verb || "magic", damage: d, kind: kind }); dealt += d; }
    }
  });
  var stacks = burningStacks(def);
  eachEffect(att, function (e, it) {
    var p = e.params;
    if (e.kind === "ignite" && stacks < (num(p.maxStacks) || 1)) {
      effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(effectUnit(att, e, it) * num(p.power))), source: "ignite", kind: "fire" }, rounds: num(p.rounds) || 3 });
      stacks++;
    }
  });
  // Envenom: poison on the blade, its own stack beside the burns.
  var venom = burningStacks(def, "venom");
  eachEffect(att, function (e, it) {
    var p = e.params;
    if (e.kind === "envenom" && venom < (num(p.maxStacks) || 1)) {
      effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(effectUnit(att, e, it) * num(p.power))), source: "venom", kind: "poison" }, rounds: num(p.rounds) || 3 });
      venom++;
    }
  });
  var chilled = false;
  eachEffect(def, function (e) { if (e.kind === "speedMult" && e.params.source === "chill") chilled = true; });
  eachEffect(att, function (e) {
    var p = e.params;
    if (e.kind === "chill" && !chilled && random.float() < num(p.chance)) {
      effects.push({ on: "target", kind: "speedMult", params: { mult: num(p.mult) || 0.5, source: "chill" }, rounds: num(p.rounds) || 2 });
      chilled = true;
    }
  });
  // A whip's lash can tangle the legs; it never stacks with a chill.
  var lash = traitsOf(wielded(att)).slow;
  if (lash && !chilled && random.float() < num(lash.chance)) {
    effects.push({ on: "target", kind: "speedMult", params: { mult: num(lash.mult), source: "chill" }, rounds: num(lash.rounds) || 2 });
  }
  var thorns = capped(def, "thorns", "fraction", LIMITS.thorns);
  if (thorns > 0) {
    var te = bestEffect(def, "thorns", "fraction");
    var tk = te.params.kind || "pierce";
    var back = Math.round(resisted(att, tk, dmg * thorns));
    if (back > 0) procs.push({ verb: te.params.verb || "thorns", damage: back, back: true, kind: tk });
    else if (dmg > 0 && immune(att, tk)) procs.push({ verb: te.params.verb || "thorns", damage: 0, back: true, kind: tk, resisted: true });
  }
  var leech = capped(att, "leech", "fraction", LIMITS.leech);
  out.procs = procs;
  out.effects = effects;
  out.heal = Math.round(dealt * leech);
  return out;
}

// soakAmount is the flat damage c's soak effects take off each hit.
function soakAmount(c) { return sumEffects(c, "soak", "power") * unit(c); }

// lastStandMult is the damage multiplier c's lastStand effects give while
// c is below their threshold. Stacking: multiply.
function lastStandMult(c) {
  var m = 1, frac = c.healthMax > 0 ? c.health / c.healthMax : 1;
  eachEffect(c, function (e) { if (e.kind === "lastStand" && frac < num(e.params.threshold)) m *= (num(e.params.mult) || 1); });
  return m;
}

// executeMult is the best execute bonus against def as it stands.
function executeMult(att, def) {
  var m = 1, frac = def.healthMax > 0 ? def.health / def.healthMax : 1;
  eachEffect(att, function (e) { if (e.kind === "execute" && frac < num(e.params.threshold)) m = Math.max(m, num(e.params.mult) || 1); });
  return m;
}

// ---------------------------------------------------------------------
// Stats (RULES 3.1 to 3.3): linear multipliers, no ceiling.
// ---------------------------------------------------------------------
function stat(c, name) {
  var v = (c.stats && c.stats[name]) || P.statBaseline;
  eachEffect(c, function (e) { if (e.kind === "stat" && e.params.stat === name) v += num(e.params.amount); });
  return v;
}

// mult is a stat's multiplier; with band it carries the ±statBand jitter.
function mult(c, name, band) {
  var m = 1 + P.statK * (stat(c, name) - P.statBaseline);
  if (m < 0.05) m = 0.05;
  if (band) m *= 1 + (random.float() * 2 - 1) * P.statBand;
  return m;
}

function levelMult(c) { return 1 + P.levelK * ((c.level || 1) - 1); }

function spreadRoll(mean, spread) {
  if (!spread) return mean;
  return mean * (1 + (random.float() * 2 - 1) * spread);
}

// ---------------------------------------------------------------------
// kFor is the reduction unit against an attacker of level L (RULES 4.3).
function kFor(att) { return P.K * Math.pow(P.levelGrowth, Math.max(0, (att.level || 1) - 1) * P.kGrowth); }

// The attack pipeline (RULES 4.1 to 4.4, 5.4): prepare is the engine's
// swing meter; here roll, modify, dodge, block, reduce.
// ---------------------------------------------------------------------
function naturalAttack(c) {
  if (c.mob && c.mob.attack) {
    var a = c.mob.attack;
    // Disarmed with no weapon item, a mob whose natural attack is a
    // weapon fights with what it has left.
    if (hasEffect(c, "disarmed") && armedNatural(c)) {
      return { damage: a.damage * P.disarmedNatural, spread: a.spread, speed: a.speed, hands: 0, kind: "blunt", verb: "punch" };
    }
    return a;
  }
  return WEAPON_BASELINES.unarmed(c.level || 1);
}

// ARMED_VERBS are the natural-attack verbs that mean a weapon in hand:
// a mob written with one can be disarmed though it wields no item. A
// bite, a claw, or a spell cannot.
var ARMED_VERBS = ["slash", "stab", "shoot", "cleave", "hack", "smash", "pound", "club", "thrust", "hammer",
  "strike", "smite", "whack", "poke", "jab", "crush", "cut", "las-shot", "hew", "flail", "chop", "swing",
  "spear", "slice", "skewer", "impale", "glaive", "reaper", "dakka", "lunge", "shear", "bolt", "dart"];

// armedNatural reports whether c, a mob with no weapon item, fights with
// a weapon its natural attack stands for.
function armedNatural(c) {
  if (!c.mob || !c.mob.attack || (c.equipment && c.equipment.wield)) return false;
  return ARMED_VERBS.indexOf(c.mob.attack.verb) >= 0;
}

// critMult rolls a crit for att's blow with weapon: the best crit effect
// and the weapon family's edge. It returns the multiplier, 1 for none.
function critMult(att, weapon) {
  var ce = bestEffect(att, "crit", "chance");
  var chance = (ce ? num(ce.params.chance) : 0) + num(traitsOf(weapon).crit);
  if (chance <= 0 || random.float() >= chance) return 1;
  return ((ce && num(ce.params.mult)) || 2) + sumEffects(att, "critMult", "amount");
}

// resolveAttack is one swing with weapon, held in hand: "wield" for the
// main hand, "second" for the off hand (RULES 4.4).
function resolveAttack(att, def, weapon, round, hand) {
  if (hasEffect(att, "disarmed")) weapon = null;
  var w = weapon && weapon.weapon ? weapon.weapon : naturalAttack(att);
  var verb = w.verb || "punch";

  // Blindness (Dirt Kicking): some swings go wide.
  var blind = blindMiss(att);
  if (blind > 0 && random.float() < blind) return { hit: false, damage: 0, crit: false, verb: verb, stage: "miss" };

  // Dodge: intrinsic, scaled by Dexterity, shifted by effects, armor and
  // weapon families, feats, and the Dodge skill.
  var dodge = dodgeChance(def);
  if (dodge > 0 && random.float() < dodge) return { hit: false, damage: 0, crit: false, verb: verb, stage: "dodge" };

  // Block: nothing intrinsic; a shield, a feat, or Riposte (with a weapon
  // in hand) grants it, Strength scales it. A flail goes over shields.
  var block = blockChance(def, !!traitsOf(weapon).overShield) + riposteBlock(def) * mult(def, "strength", true);
  if (block > 0 && random.float() < block) return { hit: false, damage: 0, crit: false, verb: verb, stage: "block" };

  // Roll: the weapon's spread, then Strength (or Dexterity, with Weapon
  // Finesse and a dagger) and level. An attuned weapon is drawn at its
  // wielder's level. An armed mob keeps power-unit damage (8): its weapon
  // counts against a plain one of its level, so one at its level hits as
  // its natural attack's standard would, unscaled by mobDamage.
  var base = weapon && weapon.weapon ? weaponDamage(weapon) * attunedScale(weapon, att) : w.damage;
  if (weapon && att.mob) base /= weaponShare(att.level);
  var hitMult = strikeMult(att, weapon) * levelMult(att) * swingMult(att, def, weapon) * dualMult(att, hand);

  // Modify: crits exist only as effects (4.2) and the axe's edge; execute
  // finishes the wounded.
  var cm = critMult(att, weapon), crit = cm > 1;
  hitMult *= cm * executeMult(att, def);
  // The hit is the swing and the added damage it carries (5.3), each
  // part with its own kind.
  var kind = w.kind || "blunt";
  var parts = [{ kind: kind, dmg: spreadRoll(base, w.spread) * hitMult }];
  eachAdded(att, weapon, function (e, u) {
    parts.push({ kind: e.params.kind || "magic", dmg: spreadRoll(u * num(e.params.power), 0.2) * hitMult });
  });
  var whole = 0, i;
  for (i = 0; i < parts.length; i++) whole += parts[i].dmg;

  // Reduce, the whole hit: D / (D + K), with D from armor or natural
  // hide, Constitution, and level, less what a mace ignores; then soak,
  // a last stand, and protection effects.
  var D = defense(def) * (1 - Math.min(1, num(traitsOf(weapon).pierce))), K = kFor(att);
  var dmg = whole * K / (D + K);
  dmg = Math.max(dmg * LIMITS.soakFloor, dmg - soakAmount(def));
  dmg = dmg * lastStandMult(def) * protectMult(def);
  var landed = Math.round(dmg) > 0, kept = whole > 0 ? dmg / whole : 0;

  // Resist: last, part by part (4.8), each part keeping its share of
  // what got through. A creature immune to the blade still takes its
  // fire; a hit wholly resisted still lands, so what rides on it (a
  // proc) is weighed on its own kind.
  var total = 0, shrugged = true;
  for (i = 0; i < parts.length; i++) {
    if (immune(def, parts[i].kind)) continue;
    shrugged = false;
    total += resisted(def, parts[i].kind, parts[i].dmg * kept);
  }
  var out = { hit: true, damage: Math.max(0, Math.round(total)), crit: crit && !shrugged, verb: verb, stage: "", kind: kind };
  if (shrugged) out.resisted = true;
  return landed ? onHit(att, def, out.damage, out) : out;
}

// eachAdded calls fn(effect, unit) for every part of added damage (5.3)
// riding on a swing with weapon: c's own, its prototype's, and what its
// gear carries, but a weapon's only on that weapon's swings. It is part
// of the hit: what scales the swing (Strength, level, damage effects, a
// crit) scales it, and armor, soak, and protection take from it as from
// the swing; only resistance weighs it apart.
function eachAdded(c, weapon, fn) {
  eachEffect(c, function (e, it) {
    if (e.kind !== "added") return;
    if (it && it.type === "weapon" && !(weapon && weapon.id === it.id)) return;
    fn(e, effectUnit(c, e, it));
  });
}

// defense is the defender's D: the sum of worn armor, or the mob's natural
// armor if nothing is worn, with no roll (chance in defending is parry,
// dodge and block), plus flat "defense"
// effects (wards), times Constitution and level.
function defense(c) {
  var D = 0, worn = false, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.armor && it.armor.defense) { D += armorDefense(it) * attunedScale(it, c); worn = true; }
  }
  if (!worn && c.mob && c.mob.armor) D = c.mob.armor.defense;
  D += sumEffects(c, "defense", "amount");
  return Math.max(0, D * mult(c, "constitution", true) * levelMult(c));
}

// blindMiss is the share of c's swings that go wide while blinded, the
// worst blindness counting once, halved by Blind-Fighting (take the best).
function blindMiss(c) {
  var b = bestEffect(c, "blind", "miss");
  if (!b) return 0;
  var bf = 1;
  eachEffect(c, function (e) { if (e.kind === "blindFighting") bf = Math.min(bf, num(e.params.mult) || 0.5); });
  return num(b.params.miss) * bf;
}

// dodgeChance is c's chance to dodge a swing or a skill; flat leaves out
// the stat band, for the score sheet.
function dodgeChance(c, flat) {
  return P.dodgeBase * mult(c, "dexterity", !flat) + sumEffects(c, "dodge", "amount") + sumEffects(c, "powerAttack", "dodge") +
    armorTrait(c, "dodge") + weaponTrait(c, "dodge") + 0.10 * skillRating(c, "dodge") / 100 - (exhausted(c) ? P.exhaustedDodge : 0);
}

// blockChance is c's chance to block, Riposte aside: block effects that do
// not come from the shield, then the shield's own (shieldBlock), unless
// the blow goes over it. Strength scales it; flat leaves out the band.
function blockChance(c, overShield, flat) {
  var own = 0, sh = shield(c);
  if (sh && sh.effects) for (var i = 0; i < sh.effects.length; i++) if (sh.effects[i].kind === "block") own += num(sh.effects[i].params.chance);
  var b = sumEffects(c, "block", "chance") - own + (overShield ? 0 : shieldBlock(c));
  return P.blockBase + b * mult(c, "strength", !flat);
}

// strikeMult is the stat multiplier on c's damage: Strength, or Dexterity
// with Weapon Finesse and a dagger in hand.
function strikeMult(c, weapon) {
  var finesse = false;
  eachEffect(c, function (e) { if (e.kind === "finesse") finesse = true; });
  return mult(c, finesse && isDagger(weapon) ? "dexterity" : "strength", true);
}

// damageMult is the sum of c's damage bonuses as one multiplier: "damage"
// effects (a weapon-only one needs a weapon), Power Attack, and the Great
// Weapon feats with a two-handed weapon.
function damageMult(c, weapon) {
  var add = 0, great = twoHanded(weapon);
  eachEffect(c, function (e) {
    if (e.kind === "damage" && (!e.params.weapon || weapon)) add += (num(e.params.mult) || 1) - 1;
    if (e.kind === "powerAttack") add += (num(e.params.mult) || 1) - 1;
    if (e.kind === "greatWeapon" && great) add += (num(e.params.mult) || 1) - 1;
  });
  return Math.max(0, 1 + add);
}

// hampered reports whether c is stunned, tripped, or slowed.
function hampered(c) {
  var h = false;
  eachEffect(c, function (e) { if (e.kind === "speedMult" && num(e.params.mult) < 1) h = true; });
  return h;
}

// opportunistMult is Opportunist's bonus against a hampered target.
function opportunistMult(att, def) {
  if (!hampered(def)) return 1;
  var m = 1;
  eachEffect(att, function (e) { if (e.kind === "opportunist") m = Math.max(m, num(e.params.mult) || 1); });
  return m;
}

// unarmedMult is what an empty hand gets: Hand to Hand (up to double) and
// Iron Fist. Mobs' natural attacks are not bare hands.
function unarmedMult(c) {
  var m = 1 + skillRating(c, "hand_to_hand") / 100;
  eachEffect(c, function (e) { if (e.kind === "unarmedMult") m *= num(e.params.mult) || 1; });
  return m;
}

// swingMult is every multiplier a swing gets beyond stats and level.
function swingMult(att, def, weapon) {
  var m = damageMult(att, weapon) * (1 + 0.15 * skillRating(att, "enhanced_damage") / 100) * opportunistMult(att, def);
  if (!weapon && !att.mob) m *= unarmedMult(att);
  return m;
}

// steadiness is c's footing: the best chance to shrug off a lost round,
// and the smallest share of any slow that still lands (Unstoppable).
function steadiness(c) {
  var out = { chance: 0, slow: 1 };
  eachEffect(c, function (e) {
    if (e.kind !== "steady") return;
    out.chance = Math.max(out.chance, num(e.params.chance));
    if (e.params.slow !== undefined) out.slow = Math.min(out.slow, num(e.params.slow));
  });
  return out;
}

// protectMult is the product of "protect" effects on c (Sanctuary): a
// multiplier on damage taken. Stacking: multiply.
function protectMult(c) {
  var m = 1;
  eachEffect(c, function (e) { if (e.kind === "protect") m *= (num(e.params.mult) || 1); });
  return m;
}

// speedMult is the product of "speedMult" effects (Daze, Stillness).
// Unstoppable keeps only part of every slow.
function speedMult(c) {
  var m = 1, keep = steadiness(c).slow;
  eachEffect(c, function (e) {
    if (e.kind !== "speedMult") return;
    var s = e.params.mult === undefined ? 1 : num(e.params.mult);
    m *= 1 - (1 - s) * keep;
  });
  return m;
}

// ---------------------------------------------------------------------
// Derived values, regeneration, progression (RULES 4.4, 4.5, 7.1, 7.2)
// ---------------------------------------------------------------------
function derivedStats(c) {
  var healthMax = (c.mob && c.mob.health > 0)
    ? c.mob.health
    : Math.round(P.healthBase * growth(c.level) * mult(c, "constitution") * levelMult(c));
  var w = c.equipment && c.equipment.wield && c.equipment.wield.weapon ? c.equipment.wield.weapon : naturalAttack(c);
  // An extra attack (a feat, Haste, Twin Strike) is one more weapon's
  // worth of swings, not one swing (RULES 4.4): a sword swings once more,
  // a dagger twice, a maul every other round, so it adds the same damage
  // a round whatever the weapon.
  var speed = (w.speed || 1) * (mult(c, "dexterity") + sumEffects(c, "attacks", "amount") + passiveSwings(c)) * speedMult(c);
  // A second weapon swings at its own speed, and the off-hand feats add
  // to it the same way.
  var off = offhand(c), offSpeed = 0;
  if (off) offSpeed = Math.max(0, (num(off.weapon.speed) || 1) * (mult(c, "dexterity") + sumEffects(c, "offhandAttacks", "amount"))) * speedMult(c);
  var stats = {};
  for (var i = 0; i < STATS.length; i++) stats[STATS[i]] = stat(c, STATS[i]);
  return {
    healthMax: Math.max(1, healthMax),
    manaMax: manaMax(c),
    staminaMax: staminaMax(c),
    speed: Math.max(0.1, speed),
    offSpeed: offSpeed,
    stats: stats
  };
}

function manaMax(c) {
  var m = Math.max(mult(c, "intelligence"), mult(c, "wisdom"));
  // Arcane Reservoir, Deep Faith, and caster gear deepen the pool (6.6).
  return Math.max(0, Math.round((P.manaBase + P.manaPerLevel * ((c.level || 1) - 1)) * m * (1 + sumEffects(c, "manaMax", "amount"))));
}

// manaRegen is mana back per round at a position multiplier.
function manaRegen(c, posMult) {
  if (!c.manaMax || c.mana >= c.manaMax) return 0;
  return Math.max(1, Math.round(c.manaMax / P.manaRegenRounds * posMult));
}

function staminaMax(c) { return P.staminaPool; }

// staminaStat is the factor the better of Strength and Constitution puts
// on stamina regeneration, at half a stat multiplier's slope.
function staminaStat(c) {
  var best = Math.max(stat(c, "strength"), stat(c, "constitution"));
  return Math.max(0.05, 1 + P.staminaStatK * (best - P.statBaseline));
}

// staminaGear is the factor gear puts on stamina regeneration: heavy and
// plate armor by the share of the body they cover, and a heavy weapon.
function staminaGear(c) {
  var heavy = 0, plate = 0, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (!it || it.type !== "armor") continue;
    var share = it.slot === "shield" ? 0.1 : (SLOT_WEIGHT[it.slot] || 0);
    if (it.baseline === "heavy" || it.baseline === "tower") heavy += share;
    if (it.baseline === "plate") plate += share;
  }
  var w = wielded(c);
  return Math.max(0.2, 1 - P.staminaHeavyArmor * heavy - P.staminaPlateArmor * plate - (w && isHeavy(w) ? P.staminaHeavyWeapon : 0));
}

// staminaRegen is stamina back per round at a multiplier.
function staminaRegen(c, m) {
  if (!c.staminaMax || c.stamina >= c.staminaMax) return 0;
  return Math.max(1, Math.round(c.staminaMax / P.staminaRegenRounds * m * staminaStat(c) * staminaGear(c)));
}

function exhausted(c) { return c.staminaMax > 0 && c.stamina <= 0; }

function onTick(c) {
  // Damage over time ("dot" effects: Acid Splash, Blight, ignite) ticks
  // whether or not the character is fighting; so does regen.
  var dot = 0;
  eachEffect(c, function (e) { if (e.kind === "dot") dot += resisted(c, e.params.kind, num(e.params.damage)); });
  var regen = Math.round(c.healthMax * capped(c, "regen", "fraction", LIMITS.regen));
  if (c.fighting) {
    var aura = [];
    eachEffect(c, function (e, it) {
      if (e.kind === "aura") aura.push({ verb: e.params.verb || "aura", kind: e.params.kind || "magic", damage: Math.max(1, Math.round(spreadRoll(effectUnit(c, e, it) * num(e.params.power), 0.2))) });
    });
    var fightMana = P.manaFightRegen > 0 ? manaRegen(c, P.manaFightRegen) : 0;
    // Divine Favor and the like: a trickle of mana even in a fight (6.6).
    var favor = sumEffects(c, "manaFight", "fraction");
    if (favor > 0 && c.manaMax && c.mana < c.manaMax) fightMana += Math.max(1, Math.round(c.manaMax * favor));
    return { healthDelta: regen - Math.round(dot), manaDelta: fightMana, staminaDelta: staminaRegen(c, 1),
      skills: improvePassives(c), aura: aura };
  }
  // Out of a fight health comes back, faster resting and faster still
  // asleep. Fast Healing adds up to half again, and it improves while
  // there is healing to do.
  var posMult = c.position === "sleeping" ? P.sleepRegen : c.position === "resting" ? P.restRegen : 1;
  var rest = Math.max(1, Math.round(c.healthMax / P.regenRounds * posMult * (1 + 0.5 * skillRating(c, "fast_healing") / 100)));
  var skills = {};
  var fh = skillRating(c, "fast_healing");
  if (fh > 0 && c.health < c.healthMax) {
    var next = improve(fh);
    if (next !== fh) skills.fast_healing = next;
  }
  return { healthDelta: rest + regen - Math.round(dot), manaDelta: manaRegen(c, posMult),
    staminaDelta: staminaRegen(c, P.staminaRestMult * posMult), skills: skills };
}

function levelCost(n) { return P.xpBase * n * Math.pow(P.xpR, n - 1); }

// xpToLevel is the total needed to reach level: the running sum of costs.
function xpToLevel(level) {
  var total = 0;
  for (var n = 1; n < level; n++) total += levelCost(n);
  return Math.round(total);
}

// xpForKill: an override on the prototype, else 10 * victim level, decayed
// with the level gap in both directions (7.1).
function xpForKill(killer, victim) {
  var base = (victim.mob && victim.mob.xp > 0) ? victim.mob.xp : P.xpPerLevelKill * (victim.level || 1);
  var gap = (victim.level || 1) - (killer.level || 1);
  var factor;
  if (gap <= -10) factor = 0;
  else if (gap < 0) factor = 1 + gap / 10;          // -5 levels: half
  else factor = 1 + 0.1 * Math.min(gap, 5);          // +5 levels: 1.5x, no more
  return Math.max(0, Math.round(base * factor));
}

function onLevel(c, newLevel) {
  return { statPoints: P.pointsPerLevel, featPicks: P.featsPerLevel, statDeltas: {}, message: "You raise a level!" };
}

// ---------------------------------------------------------------------
// Feats (RULES 7.2): permanent effects with a minimum level, chosen at
// level-up with the feat command. Stacking follows the kind: stat adds,
// crit takes the larger (so Deadly Edge supersedes Keen Edge), attacks
// add.
// ---------------------------------------------------------------------
var FEATS = [
  { id: "toughness", name: "Toughness", level: 1, stats: { constitution: 11 }, description: "+2 Constitution.",
    effect: { kind: "stat", params: { stat: "constitution", amount: 2 } } },
  { id: "brawn", name: "Brawn", level: 1, stats: { strength: 11 }, description: "+2 Strength.",
    effect: { kind: "stat", params: { stat: "strength", amount: 2 } } },
  { id: "nimble", name: "Nimble", level: 1, stats: { dexterity: 11 }, description: "+2 Dexterity.",
    effect: { kind: "stat", params: { stat: "dexterity", amount: 2 } } },
  { id: "keen", name: "Keen Edge", level: 3, stats: { dexterity: 12 }, description: "A 5 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.05, mult: 2 } } },
  { id: "parry", name: "Parry", level: 3, stats: { dexterity: 12 }, description: "Block one swing in ten with your weapon.",
    effect: { kind: "block", params: { chance: 0.10 } } },
  { id: "evasion", name: "Evasion", level: 5, stats: { dexterity: 13 }, description: "Dodge 5 percent more often.",
    effect: { kind: "dodge", params: { amount: 0.05 } } },
  { id: "second_attack", name: "Second Attack", level: 5, stats: { strength: 13 }, description: "One extra attack every round: a swing with a sword, two with a dagger, half with a maul.",
    effect: { kind: "attacks", params: { amount: 1 } } },
  { id: "deadly", name: "Deadly Edge", level: 8, stats: { dexterity: 14 }, requires: ["keen"], description: "A 10 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.10, mult: 2 } } },
  { id: "iron_skin", name: "Iron Skin", level: 10, stats: { constitution: 15 }, requires: ["toughness"], description: "+4 Constitution.",
    effect: { kind: "stat", params: { stat: "constitution", amount: 4 } } },
  { id: "third_attack", name: "Third Attack", level: 12, stats: { strength: 15 }, requires: ["second_attack"], description: "Another extra attack every round.",
    effect: { kind: "attacks", params: { amount: 1 } } },

  // --- approved 2026-09-28 ---
  { id: "iron_will", name: "Iron Will", level: 1, stats: { wisdom: 11 }, description: "+2 Wisdom.",
    effect: { kind: "stat", params: { stat: "wisdom", amount: 2 } } },
  { id: "keen_mind", name: "Keen Mind", level: 1, stats: { intelligence: 11 }, description: "+2 Intelligence.",
    effect: { kind: "stat", params: { stat: "intelligence", amount: 2 } } },
  { id: "thick_hide", name: "Thick Hide", level: 2, stats: { constitution: 12 }, description: "Every hit on you is a tenth of a swing smaller.",
    effect: { kind: "soak", params: { power: 0.10 } } },
  { id: "shield_focus", name: "Shield Focus", level: 3, stats: { strength: 12 }, description: "Block 8 percent more with a shield worn.",
    effect: { kind: "shieldBlock", params: { chance: 0.08 } } },
  { id: "finesse", name: "Weapon Finesse", level: 3, stats: { dexterity: 12 }, description: "With a dagger, Dexterity drives your damage instead of Strength.",
    effect: { kind: "finesse", params: {} } },
  { id: "blind_fighting", name: "Blind-Fighting", level: 4, stats: { wisdom: 12 }, description: "Blinded, you miss half as often.",
    effect: { kind: "blindFighting", params: { mult: 0.5 } } },
  { id: "sure_footing", name: "Sure Footing", level: 4, stats: { strength: 12 }, description: "An even chance to keep your feet when tripped or bashed.",
    effect: { kind: "steady", params: { chance: 0.5 } } },
  { id: "power_attack", name: "Power Attack", level: 5, stats: { strength: 13 }, description: "15 percent more damage, 5 percent less dodge.",
    effect: { kind: "powerAttack", params: { mult: 1.15, dodge: -0.05 } } },
  { id: "die_hard", name: "Die Hard", level: 6, stats: { constitution: 13 }, requires: ["toughness"], description: "Below a quarter of your health, take a quarter less damage.",
    effect: { kind: "lastStand", params: { threshold: 0.25, mult: 0.75 } } },
  { id: "opportunist", name: "Opportunist", level: 6, stats: { dexterity: 13 }, description: "25 percent more damage against a foe who is stunned, tripped, or slowed.",
    effect: { kind: "opportunist", params: { mult: 1.25 } } },
  { id: "bloodthirst", name: "Bloodthirst", level: 7, stats: { constitution: 13 }, description: "Heal 5 percent of the damage you deal.",
    effect: { kind: "leech", params: { fraction: 0.05 } } },
  { id: "retaliation", name: "Retaliation", level: 8, stats: { dexterity: 14 }, requires: ["parry"], description: "A tenth of every hit on you goes back to the attacker.",
    effect: { kind: "thorns", params: { fraction: 0.10, verb: "riposte" } } },
  { id: "precise", name: "Precise Strikes", level: 8, stats: { dexterity: 14 }, requires: ["keen"], description: "Critical hits deal two and a half times damage instead of double.",
    effect: { kind: "critMult", params: { amount: 0.5 } } },
  { id: "weapon_master", name: "Weapon Master", level: 9, stats: { strength: 14 }, description: "10 percent more damage with a weapon in hand.",
    effect: { kind: "damage", params: { mult: 1.10, weapon: true } } },
  { id: "iron_fist", name: "Iron Fist", level: 9, stats: { strength: 14 }, description: "Bare hands and kicks hit 25 percent harder.",
    effect: { kind: "unarmedMult", params: { mult: 1.25 } } },
  { id: "executioner", name: "Executioner", level: 10, stats: { strength: 15 }, description: "25 percent more damage to foes below a fifth of their health.",
    effect: { kind: "execute", params: { threshold: 0.20, mult: 1.25 } } },
  { id: "stalwart", name: "Stalwart", level: 11, stats: { constitution: 15 }, requires: ["thick_hide"], description: "Every hit on you is a quarter of a swing smaller, with Thick Hide.",
    effect: { kind: "soak", params: { power: 0.15 } } },
  { id: "lightning_reflexes", name: "Lightning Reflexes", level: 11, stats: { dexterity: 15 }, requires: ["evasion"], description: "Dodge 5 percent more often.",
    effect: { kind: "dodge", params: { amount: 0.05 } } },
  { id: "regeneration", name: "Regeneration", level: 12, stats: { constitution: 15 }, requires: ["toughness"], description: "1 percent of your health back every round, even in a fight.",
    effect: { kind: "regen", params: { fraction: 0.01 } } },
  { id: "bulwark", name: "Bulwark", level: 13, stats: { strength: 16 }, requires: ["shield_focus"], description: "Block 7 percent more again with a shield worn.",
    effect: { kind: "shieldBlock", params: { chance: 0.07 } } },
  { id: "relentless", name: "Relentless", level: 14, stats: { constitution: 16 }, description: "Every skill's cooldown is a round shorter, never below one, and skills cost a fifth less stamina.",
    effect: { kind: "cooldownCut", params: { rounds: 1 } } },
  { id: "unstoppable", name: "Unstoppable", level: 16, stats: { strength: 17 }, requires: ["sure_footing"], description: "Nothing takes your round from you, and slows are halved.",
    effect: { kind: "steady", params: { chance: 1, slow: 0.5 } } },
  { id: "deathblow", name: "Deathblow", level: 18, stats: { dexterity: 17 }, requires: ["deadly"], description: "A 15 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.15, mult: 2 } } },
  { id: "fourth_attack", name: "Fourth Attack", level: 20, stats: { strength: 18 }, requires: ["third_attack"], description: "Yet another extra attack every round.",
    effect: { kind: "attacks", params: { amount: 1 } } },
  // --- fighting styles (RULES 4.4), 2026-10-11 ---
  // Two weapons: the first feat takes away the untrained penalty, the
  // next ones add swings to the off hand.
  { id: "dual_wield", name: "Dual Wield", level: 3, stats: { dexterity: 12 }, description: "Fight with a second weapon without the penalty to either hand.",
    effect: { kind: "dualWield", params: {} } },
  { id: "two_weapon", name: "Two-Weapon Fighting", level: 10, stats: { dexterity: 15 }, requires: ["dual_wield"], description: "One extra attack every round with your second weapon.",
    effect: { kind: "offhandAttacks", params: { amount: 1 } } },
  { id: "greater_two_weapon", name: "Greater Two-Weapon Fighting", level: 18, stats: { dexterity: 17 }, requires: ["two_weapon"], description: "Another extra attack every round with your second weapon.",
    effect: { kind: "offhandAttacks", params: { amount: 1 } } },
  // A two-handed weapon: each feat makes its swings hit harder.
  { id: "great_weapon", name: "Great Weapon Fighting", level: 3, stats: { strength: 12 }, description: "20 percent more damage with a two-handed weapon.",
    effect: { kind: "greatWeapon", params: { mult: 1.20 } } },
  { id: "great_weapon_mastery", name: "Great Weapon Mastery", level: 10, stats: { strength: 15 }, requires: ["great_weapon"], description: "20 percent more damage again with a two-handed weapon.",
    effect: { kind: "greatWeapon", params: { mult: 1.20 } } },
  { id: "devastating_blows", name: "Devastating Blows", level: 18, stats: { strength: 17 }, requires: ["great_weapon_mastery"], description: "20 percent more damage again with a two-handed weapon.",
    effect: { kind: "greatWeapon", params: { mult: 1.20 } } },
  { id: "juggernaut", name: "Juggernaut", level: 22, stats: { constitution: 18 }, requires: ["iron_skin"], description: "Take 10 percent less damage from everything.",
    effect: { kind: "protect", params: { mult: 0.9 } } },

  // --- caster feats (RULES 6.6), 2026-10-11 ---
  // Arcane: specialisation and force.
  { id: "focus_abjuration", name: "Spell Focus (Abjuration)", level: 3, stats: { intelligence: 12 }, school: "abjuration", description: "Abjuration spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "abjuration" } } },
  { id: "focus_conjuration", name: "Spell Focus (Conjuration)", level: 3, stats: { intelligence: 12 }, school: "conjuration", description: "Conjuration spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "conjuration" } } },
  { id: "focus_divination", name: "Spell Focus (Divination)", level: 3, stats: { intelligence: 12 }, school: "divination", description: "Divination spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "divination" } } },
  { id: "focus_enchantment", name: "Spell Focus (Enchantment)", level: 3, stats: { intelligence: 12 }, school: "enchantment", description: "Enchantment spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "enchantment" } } },
  { id: "focus_evocation", name: "Spell Focus (Evocation)", level: 3, stats: { intelligence: 12 }, school: "evocation", description: "Evocation spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "evocation" } } },
  { id: "focus_illusion", name: "Spell Focus (Illusion)", level: 3, stats: { intelligence: 12 }, school: "illusion", description: "Illusion spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "illusion" } } },
  { id: "focus_necromancy", name: "Spell Focus (Necromancy)", level: 3, stats: { intelligence: 12 }, school: "necromancy", description: "Necromancy spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "necromancy" } } },
  { id: "focus_transmutation", name: "Spell Focus (Transmutation)", level: 3, stats: { intelligence: 12 }, school: "transmutation", description: "Transmutation spells are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, school: "transmutation" } } },
  { id: "arcane_reservoir", name: "Arcane Reservoir", level: 4, stats: { intelligence: 12 }, branch: "arcane", description: "A quarter more mana.",
    effect: { kind: "manaMax", params: { amount: 0.25 } } },
  { id: "empower", name: "Empower Spell", level: 6, stats: { intelligence: 13 }, branch: "arcane", description: "Arcane spells that harm are 15 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.15, branch: "arcane", kind: "damage" } } },
  { id: "combat_casting", name: "Combat Casting", level: 6, stats: { intelligence: 13 }, branch: "arcane", description: "Blows no longer break your arcane casting.",
    effect: { kind: "castCost", params: { steady: true, branch: "arcane" } } },
  { id: "penetration", name: "Spell Penetration", level: 8, stats: { intelligence: 14 }, branch: "arcane", description: "Arcane spells are a quarter harder to resist.",
    effect: { kind: "penetration", params: { amount: 0.25, branch: "arcane" } } },
  { id: "arcane_precision", name: "Arcane Precision", level: 10, stats: { intelligence: 15 }, branch: "arcane", description: "A 10 percent chance for an arcane spell to surge for half again.",
    effect: { kind: "spellCrit", params: { chance: 0.10, mult: 1.5, branch: "arcane" } } },
  { id: "quicken", name: "Quicken Spell", level: 12, stats: { intelligence: 16 }, branch: "arcane", requires: ["combat_casting"], description: "Arcane spells of two rounds or more take a round less.",
    effect: { kind: "castCost", params: { rounds: 1, branch: "arcane" } } },
  { id: "arcane_thrift", name: "Arcane Thrift", level: 14, stats: { intelligence: 16 }, branch: "arcane", requires: ["arcane_reservoir"], description: "Arcane spells cost a fifth less mana.",
    effect: { kind: "castCost", params: { mana: 0.8, branch: "arcane" } } },
  { id: "greater_penetration", name: "Greater Penetration", level: 16, stats: { intelligence: 17 }, branch: "arcane", requires: ["penetration"], description: "Arcane spells are a quarter harder again to resist.",
    effect: { kind: "penetration", params: { amount: 0.25, branch: "arcane" } } },
  { id: "archmage", name: "Archmage", level: 20, stats: { intelligence: 18 }, branch: "arcane", requires: ["empower"], description: "Every arcane spell is 15 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.15, branch: "arcane" } } },
  // Divine: devotion and the group.
  { id: "devotion", name: "Devotion", level: 3, stats: { wisdom: 12 }, branch: "divine", description: "Your god's spells are 15 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.15, branch: "divine" } } },
  { id: "healers_grace", name: "Healer's Grace", level: 4, stats: { wisdom: 12 }, branch: "divine", description: "Spells that heal are 25 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.25, kind: "heal" } } },
  { id: "deep_faith", name: "Deep Faith", level: 4, stats: { wisdom: 12 }, branch: "divine", description: "A fifth more mana.",
    effect: { kind: "manaMax", params: { amount: 0.20 } } },
  { id: "shield_of_faith", name: "Shield of Faith", level: 5, stats: { wisdom: 13 }, branch: "divine", description: "A helpful spell leaves you taking 15 percent less damage for three rounds.",
    effect: { kind: "faithShield", params: { mult: 0.85, rounds: 3 } } },
  { id: "martyr", name: "Martyr", level: 6, stats: { wisdom: 13 }, branch: "divine", description: "Healing another heals you for a quarter of it.",
    effect: { kind: "martyr", params: { fraction: 0.25 } } },
  { id: "steadfast_prayer", name: "Steadfast Prayer", level: 7, stats: { wisdom: 13 }, branch: "divine", description: "Blows no longer break your prayers.",
    effect: { kind: "castCost", params: { steady: true, branch: "divine" } } },
  { id: "channel", name: "Channel Divinity", level: 8, stats: { wisdom: 14 }, branch: "divine", description: "Your god's spells are a quarter harder to resist.",
    effect: { kind: "penetration", params: { amount: 0.25, branch: "divine" } } },
  { id: "divine_favor", name: "Divine Favor", level: 10, stats: { wisdom: 15 }, branch: "divine", description: "Two percent of your mana returns each round, even in a fight.",
    effect: { kind: "manaFight", params: { fraction: 0.02 } } },
  { id: "zealous_wrath", name: "Zealous Wrath", level: 12, stats: { wisdom: 16 }, branch: "divine", requires: ["devotion"], description: "Divine spells that harm are 20 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.20, branch: "divine", kind: "damage" } } },
  { id: "miracle", name: "Miracle", level: 16, stats: { wisdom: 17 }, branch: "divine", requires: ["healers_grace"], description: "A 10 percent chance for a divine spell to surge for double.",
    effect: { kind: "spellCrit", params: { chance: 0.10, mult: 2, branch: "divine" } } },
  { id: "saint", name: "Saint", level: 20, stats: { wisdom: 18 }, branch: "divine", requires: ["devotion"], description: "Every divine spell is 15 percent stronger.",
    effect: { kind: "spellPower", params: { mult: 1.15, branch: "divine" } } }
];

function featList() { return FEATS; }

// standardKit is what "a level N fighter" wears in the simulator (RULES
// 4.5): a standard weapon and a medium armor set, all at level N, with
// the effects a character can expect to carry at that level (5.2). The
// weapon is the best of the drops a character has seen, not a typical
// one: one full-quality roll of added damage (5.3), which makes it whole
// against the power unit. A rare averages about 0.8 of such a roll and
// an epic 1.2, so the kit sits between. No affix rolls below item level
// 5, so below it the kit is plain. `simulate plain:N` leaves the
// effects off.
var KIT_ADDED_FROM = 5;

function standardKit(level) {
  var sword = { name: "a standard sword", type: "weapon", slot: "wield", baseline: "standard", weapon: { hands: 1, verb: "slash" } };
  if (level >= KIT_ADDED_FROM) sword.effects = [addedAffix("fire", level, 1)];
  return [
    sword,
    { name: "a standard breastplate", type: "armor", slot: "body", baseline: "medium" },
    { name: "standard greaves", type: "armor", slot: "legs", baseline: "medium" },
    { name: "a standard helm", type: "armor", slot: "head", baseline: "medium" },
    { name: "standard bracers", type: "armor", slot: "arms", baseline: "medium" },
    { name: "standard boots", type: "armor", slot: "feet", baseline: "medium" },
    { name: "standard gauntlets", type: "armor", slot: "hands", baseline: "medium" },
    { name: "a standard mantle", type: "armor", slot: "shoulders", baseline: "medium" },
    { name: "a standard visor", type: "armor", slot: "face", baseline: "medium" },
    { name: "a standard belt", type: "armor", slot: "belt", baseline: "medium" },
    { name: "a standard wristguard", type: "armor", slot: "wrist", baseline: "medium" },
    { name: "a standard wristguard", type: "armor", slot: "wrist", baseline: "medium" }
  ];
}

function onCreate(c) {
  var stats = {};
  for (var i = 0; i < STATS.length; i++) stats[STATS[i]] = P.statBaseline;
  return { stats: stats, statPoints: P.pointsAtCreation,
           message: "You have " + P.pointsAtCreation + " stat points to spend. Type 'train' to see your stats." };
}

function deathRules() {
  return { xpFraction: P.deathXpFraction, xpLevelCap: P.deathXpLevelCap, corpseRounds: P.corpseRounds, respawnHealth: P.respawnHealth };
}

// ---------------------------------------------------------------------
// Magic (RULES 6). spellList defines the spells; mana is each one's cost,
// and materials an extra cost only some spells name. The engine owns cast
// time, targets, materials, and cooldowns; resolveCast decides what each
// target suffers or gains. Damage is a multiple of the power unit at the
// caster's level (6.5, 5.2), times the branch stat and level (6.4).
// ---------------------------------------------------------------------
var SPELLS = [
  { id: "firebolt", kind: "fire", innate: true, level: 1, stats: { intelligence: 11 }, mana: 6, name: "Firebolt", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "single", save: "reflex", saveEffect: "half", damage: 2.0,
    description: "A bolt of fire at one target." },
  { id: "fireball", kind: "fire", innate: true, level: 10, stats: { intelligence: 16 }, mana: 15, name: "Fireball", branch: "arcane", school: "evocation", castRounds: 2, interruptOnDamage: true, cooldown: 8,
    target: "area", save: "reflex", saveEffect: "half", damage: 1.5,
    description: "Fire fills the room." },
  { id: "ward", innate: true, level: 2, stats: { intelligence: 11 }, mana: 8, name: "Ward", branch: "arcane", school: "abjuration", castRounds: 0, interruptOnDamage: false, cooldown: 10,
    target: "ally", save: "none",
    effect: function (caster, target) { return { kind: "defense", params: { amount: target.level + 2 }, rounds: 10 }; },
    description: "A ward as strong as armor, for ten rounds." },
  { id: "acid", kind: "acid", innate: true, level: 3, stats: { intelligence: 12 }, mana: 7, name: "Acid Splash", branch: "arcane", school: "conjuration", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "single", save: "fortitude", saveEffect: "half", damage: 1.5,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.3 * power) }, rounds: 3 }; },
    description: "Acid that keeps burning." },
  { id: "foresight", innate: true, level: 4, stats: { intelligence: 12 }, mana: 8, name: "Foresight", branch: "arcane", school: "divination", castRounds: 0, interruptOnDamage: false, cooldown: 20,
    target: "self", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.10 }, rounds: 10 }; },
    description: "See the blow before it lands." },
  { id: "daze", innate: true, level: 3, stats: { intelligence: 12 }, mana: 6, name: "Daze", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 5,
    target: "single", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.5 }, rounds: 3 }; },
    description: "The target's swings come slow." },
  { id: "blur", innate: true, level: 5, stats: { intelligence: 13 }, mana: 8, name: "Blur", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 15,
    target: "self", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.15 }, rounds: 5 }; },
    description: "Your outline swims." },
  { id: "drain", kind: "drain", innate: true, level: 5, stats: { intelligence: 13 }, mana: 7, name: "Drain", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 3,
    target: "single", save: "fortitude", saveEffect: "half", damage: 1.5, drain: 0.5,
    description: "Take their life for your own." },
  { id: "haste", innate: true, level: 8, stats: { intelligence: 15 }, mana: 15, name: "Haste", branch: "arcane", school: "transmutation", castRounds: 2, interruptOnDamage: true, cooldown: 20,
    target: "self", save: "none",
    effect: function () { return { kind: "attacks", params: { amount: 1 }, rounds: 5 }; },
    description: "One more attack every round." },
  { id: "mend", innate: true, level: 1, stats: { wisdom: 11 }, mana: 8, name: "Mend", branch: "divine", deity: "good", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "ally", save: "none", heal: 0.30,
    description: "Close wounds." },
  { id: "sanctuary", innate: true, level: 8, stats: { wisdom: 15 }, mana: 20, name: "Sanctuary", branch: "divine", deity: "good", castRounds: 2, interruptOnDamage: true, cooldown: 30,
    target: "group", save: "none",
    effect: function () { return { kind: "protect", params: { mult: 0.5 }, rounds: 5 }; },
    description: "The group takes half damage." },
  { id: "stillness", innate: true, level: 4, stats: { wisdom: 12 }, mana: 12, name: "Stillness", branch: "divine", deity: "neutral", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    target: "area", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.5 }, rounds: 2 }; },
    description: "Every enemy slows." },
  { id: "blight", kind: "unholy", innate: true, level: 6, stats: { wisdom: 14 }, mana: 12, name: "Blight", branch: "divine", deity: "evil", castRounds: 1, interruptOnDamage: true, cooldown: 6,
    target: "area", save: "fortitude", saveEffect: "half", dot: true,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.4 * power) }, rounds: 5 }; },
    description: "Rot spreads through every enemy." },
  // Enchantments (RULES 5.6): cast on an item in the caster's inventory,
  // resolved by resolveEnchant, not resolveCast. Every one can fail.
  { id: "enchant_weapon", innate: true, level: 25, stats: { intelligence: 20 }, mana: 150, name: "Enchant Weapon", branch: "arcane", school: "enchantment", castRounds: 2, interruptOnDamage: true, cooldown: 0,
    target: "item", enchant: "weapon", difficulty: 20,
    description: "Raise a weapon's own damage by 5 to 10 percent (at least 1). Each success is another effect on it; a failure may strip it or destroy it." },
  { id: "enchant_armor", innate: true, level: 25, stats: { intelligence: 20 }, mana: 150, name: "Enchant Armor", branch: "arcane", school: "enchantment", castRounds: 2, interruptOnDamage: true, cooldown: 0,
    target: "item", enchant: "armor", difficulty: 20,
    description: "Raise a piece of armor's own defense by 5 to 10 percent (at least 1). Each success is another effect on it; a failure may strip it or destroy it." },
  { id: "restore_item", innate: true, level: 25, stats: { intelligence: 20 }, mana: 150, name: "Restore Item", branch: "arcane", school: "enchantment", castRounds: 2, interruptOnDamage: true, cooldown: 0,
    target: "item", enchant: "restore", difficulty: 15,
    description: "Return an item to what it was made as: every affix, rolled or enchanted, comes off. Not on a permanent item." },
  { id: "permanency", innate: true, level: 30, stats: { intelligence: 22 }, mana: 150, name: "Permanency", branch: "arcane", school: "enchantment", castRounds: 3, interruptOnDamage: true, cooldown: 0,
    target: "item", enchant: "permanency", difficulty: 40,
    description: "Fix an item as it is: nothing can enchant or restore it again. Not an effect of its own for the item's tier." }
];

function spellList() {
  // The engine only needs the definition fields; functions stay here.
  return SPELLS.map(function (sp) {
    return { id: sp.id, name: sp.name, branch: sp.branch, school: sp.school || "", deity: sp.deity || "",
      castRounds: sp.castRounds, interruptOnDamage: !!sp.interruptOnDamage, cooldown: sp.cooldown || 0, mana: sp.mana || 0, level: sp.level || 1, stats: sp.stats || {},
      materials: sp.materials || [], target: sp.target, save: sp.save || "none", saveEffect: sp.saveEffect || "",
      innate: !!sp.innate, price: sp.price || 0, found: !!sp.found,
      description: sp.description || "" };
  });
}

function spellById(id) { for (var i = 0; i < SPELLS.length; i++) if (SPELLS[i].id === id) return SPELLS[i]; return null; }

var SAVE_STAT = { reflex: "dexterity", fortitude: "constitution", will: "wisdom" };

// spellPower is the caster's number for one "unit" of spell: the power
// unit at the caster's level, times the branch stat and level.
function spellPower(caster, sp) {
  var statName = sp.branch === "divine" ? "wisdom" : "intelligence";
  return unit(caster) * mult(caster, statName, true) * levelMult(caster) * (1 + weaponTrait(caster, "spell")) * spellPowerMult(caster, sp);
}

// Caster effects (RULES 6.6) name which spells they help: a school, a
// deity, a branch, or a kind (damage: spells that hurt, heal: spells that
// heal). An effect with none of these helps every spell; with several,
// the spell must fit them all.
function spellMatches(e, sp) {
  var p = e.params || {};
  if (p.school && p.school !== sp.school) return false;
  if (p.deity && p.deity !== sp.deity) return false;
  if (p.branch && p.branch !== sp.branch) return false;
  if (p.kind === "damage" && !(sp.damage || sp.dot)) return false;
  if (p.kind === "heal" && !sp.heal) return false;
  return true;
}

// spellPowerMult multiplies every spellPower effect that fits the spell.
function spellPowerMult(c, sp) {
  var m = 1;
  eachEffect(c, function (e) { if (e.kind === "spellPower" && spellMatches(e, sp)) m *= num(e.params.mult) || 1; });
  return m;
}

// spellSum adds params[field] over the effects of kind that fit the spell.
function spellSum(c, kind, field, sp) {
  var total = 0;
  eachEffect(c, function (e) { if (e.kind === kind && spellMatches(e, sp)) total += num(e.params[field]); });
  return total;
}

// castCost is the engine's hook for what a spell costs this caster (6.6):
// thrift cuts the mana, Quicken shortens a cast of two rounds or more
// (never below one), and Combat Casting keeps damage from breaking it.
function castCost(caster, spell) {
  var sp = spellById(spell.id) || spell;
  var manaMult = 1, cut = 0, steady = false;
  eachEffect(caster, function (e) {
    if (e.kind !== "castCost" || !spellMatches(e, sp)) return;
    if (e.params.mana) manaMult *= num(e.params.mana);
    cut += num(e.params.rounds);
    if (e.params.steady) steady = true;
  });
  var rounds = spell.castRounds;
  if (rounds >= 2) rounds = Math.max(1, rounds - cut);
  return { mana: Math.max(0, Math.round(spell.mana * manaMult)), castRounds: rounds,
    interruptOnDamage: !!spell.interruptOnDamage && !steady };
}

// saves rolls the target's saving throw (6.4): S / (S + 2C).
function saves(caster, target, sp) {
  if (!sp.save || sp.save === "none") return false;
  var S = mult(target, SAVE_STAT[sp.save], true) * levelMult(target);
  var C = mult(caster, sp.branch === "divine" ? "wisdom" : "intelligence", true) * levelMult(caster);
  C *= 1 + spellSum(caster, "penetration", "amount", sp); // Penetration, Channel
  return random.float() < S / (S + 2 * C);
}

// spellKind is the kind of a spell's damage (4.8): its own, or magic.
function spellKind(sp) { return sp.kind || "magic"; }

function resolveCast(caster, targets, spell) {
  var sp = spellById(spell.id);
  if (!sp) return { ok: false, message: "Nobody remembers how that spell goes." };
  var out = { ok: true, message: "", consume: [], targets: [], casterEffects: [] };
  var totalDrain = 0, given = 0;
  for (var i = 0; i < targets.length; i++) {
    var t = targets[i];
    var tr = { index: i, damage: 0, heal: 0, saved: false, negated: false, effects: [], message: "" };
    var power = spellPower(caster, sp);
    var saved = saves(caster, t, sp);
    if (saved && sp.saveEffect === "negate") { tr.saved = true; tr.negated = true; out.targets.push(tr); continue; }
    var scale = saved ? 0.5 : 1;
    tr.saved = saved;
    var kind = spellKind(sp);
    if (sp.damage) {
      // A spell wholly resisted does nothing more when that is all it does;
      // one that also curses or burns still lands its effects (4.8).
      if (immune(t, kind) && !sp.effect && !sp.effects) { tr.negated = true; out.targets.push(tr); continue; }
      var K = kFor(caster);
      var dmg = resisted(t, kind, power * sp.damage * scale * K / (defense(t) + K) * protectMult(t));
      tr.damage = immune(t, kind) ? 0 : Math.max(1, Math.round(dmg));
      if (sp.drain) totalDrain += tr.damage * sp.drain;
    }
    if (sp.heal) tr.heal = Math.max(1, Math.round(t.healthMax * sp.heal * mult(caster, "wisdom") * spellPowerMult(caster, sp)));
    // A spell critical (Arcane Precision, Miracle) strikes or heals harder.
    var crit = Math.min(0.5, spellSum(caster, "spellCrit", "chance", sp));
    if (crit > 0 && (tr.damage || tr.heal) && random.float() < crit) {
      var cm = 1.5;
      eachEffect(caster, function (e) { if (e.kind === "spellCrit" && spellMatches(e, sp) && num(e.params.mult) > cm) cm = num(e.params.mult); });
      tr.damage = Math.round(tr.damage * cm);
      tr.heal = Math.round(tr.heal * cm);
      tr.message = "Your spell surges!";
    }
    if (tr.heal && !t.self) given += tr.heal;
    if (sp.effect) {
      var e = sp.effect(caster, t, power * scale);
      if (e) tr.effects.push(e);
    }
    // A spell may attach several effects at once (a curse that slows and
    // weakens): effects returns a list.
    if (sp.effects) {
      var es = sp.effects(caster, t, power * scale) || [];
      for (var k = 0; k < es.length; k++) if (es[k]) tr.effects.push(es[k]);
    }
    // Damage over time burns with the spell's kind unless it names its own.
    for (var j = 0; j < tr.effects.length; j++) {
      var fx = tr.effects[j];
      if (fx.kind === "dot" && fx.params && !fx.params.kind) fx.params.kind = kind;
    }
    out.targets.push(tr);
  }
  // Drain heals the caster once every target is resolved, and so does
  // Martyr, from the healing given to others (6.6).
  totalDrain += given * Math.min(0.5, sumEffects(caster, "martyr", "fraction"));
  if (totalDrain > 0) out.heal = Math.max(1, Math.round(totalDrain));
  // Shield of Faith: a helpful spell leaves its caster guarded.
  if (sp.target !== "single" && sp.target !== "area") {
    var shield = bestEffect(caster, "faithShield", "rounds");
    if (shield) out.casterEffects.push({ kind: "protect", params: { mult: num(shield.params.mult) || 0.85 }, rounds: num(shield.params.rounds) || 3 });
  }
  return out;
}

// ---------------------------------------------------------------------
// consider: a verdict on a fight, by level gap, worded from the measured
// win rates in RULES 4.5 (+1 a sure win, +3 favourable, +5 hopeless with
// geometric baselines at 1.10).
// ---------------------------------------------------------------------
function consider(me, target) {
  return verdict(me, target) + resistanceNote(target);
}

// resistanceNote names what target is immune or weak to (RULES 4.8),
// on lines of their own after the verdict.
function resistanceNote(target) {
  var immuneTo = [], weakTo = [], all = false;
  resistanceList(target).forEach(function (r) {
    if (r.amount >= 1 && r.kind === "all") all = true;
    else if (r.amount >= 1) immuneTo.push(r.kind);
    else if (r.amount < 0) weakTo.push(r.kind === "all" ? "everything" : r.kind);
  });
  var t = "";
  if (all) t += "\nNothing you have can hurt it.";
  else if (immuneTo.length) t += "\nIt cannot be hurt by " + listWords(immuneTo, "or") + ".";
  if (weakTo.length) t += "\nIt is weak to " + listWords(weakTo, "and") + ".";
  return t;
}

function listWords(xs, conj) {
  if (xs.length <= 1) return xs.join("");
  return xs.slice(0, -1).join(", ") + " " + conj + " " + xs[xs.length - 1];
}

function verdict(me, target) {
  var gap = (target.level || 1) - (me.level || 1);
  if (gap <= -5) return "You could do it with a needle.";
  if (gap <= -3) return "Easy.";
  if (gap <= -1) return "You should win comfortably.";
  if (gap === 0) return "A fair fight, and you should walk away from it.";
  if (gap === 1) return "You should win, but it will cost you.";
  if (gap <= 3) return "Risky. Bring something more than steel.";
  if (gap === 4) return "Death will thank you for your gift.";
  return "You ARE mad!";
}

// ---------------------------------------------------------------------
// combatSheet: what feats, gear, skills, and stats add up to, for the
// score sheet. Each row is {label, value}; the core rows always show, the
// rest only when something grants them. Averages, without the stat band.
// ---------------------------------------------------------------------
function combatSheet(c) {
  var rows = [];
  var add = function (label, value) { rows.push({ label: label, value: value }); };
  var pct = function (x) { return Math.round(x * 100) + "%"; };
  var signed = function (x) { var v = Math.round(x * 100); return (v >= 0 ? "+" : "") + v + "%"; };
  var weapon = c.equipment && c.equipment.wield && c.equipment.wield.weapon ? c.equipment.wield : null;

  add("Attacks", (Math.round(num(c.speed) * 10) / 10) + " a round");
  var dm = damageMult(c, weapon) * (1 + 0.15 * skillRating(c, "enhanced_damage") / 100) * dualMult(c, "wield");
  if (!weapon && !c.mob) dm *= unarmedMult(c);
  add("Damage", signed(dm - 1));
  var off = offhand(c);
  if (off) {
    var odm = damageMult(c, off) * (1 + 0.15 * skillRating(c, "enhanced_damage") / 100) * dualMult(c, "second");
    add("Off hand", (Math.round(num(c.offSpeed) * 10) / 10) + " a round, " + signed(odm - 1));
  }

  var ce = bestEffect(c, "crit", "chance");
  var critChance = (ce ? num(ce.params.chance) : 0) + num(traitsOf(weapon).crit);
  var critX = ((ce && num(ce.params.mult)) || 2) + sumEffects(c, "critMult", "amount");
  add("Critical", critChance > 0 ? pct(critChance) + " for " + critX + "x" : "none");

  add("Dodge", pct(Math.max(0, dodgeChance(c, true))));
  var block = blockChance(c, false, true) + riposteBlock(c) * mult(c, "strength", false);
  add("Block", block > 0 ? pct(block) : "none");

  // Armor: the share an even foe's blow loses to defense, as resolveAttack
  // reduces it, then protection (Juggernaut, Sanctuary).
  var D = 0, worn = false, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.armor && it.armor.defense) { D += armorDefense(it) * attunedScale(it, c); worn = true; }
  }
  if (!worn && c.mob && c.mob.armor) D = num(c.mob.armor.defense);
  D = Math.max(0, (D + sumEffects(c, "defense", "amount")) * mult(c, "constitution", false) * levelMult(c));
  add("Armor", pct(D / (D + kFor(c))) + " off hits");
  var pm = protectMult(c);
  if (pm !== 1) add("Damage taken", signed(pm - 1));

  var soak = soakAmount(c);
  if (soak > 0) add("Soak", Math.max(1, Math.round(soak)) + " off each hit");
  var leech = capped(c, "leech", "fraction", LIMITS.leech);
  if (leech > 0) add("Leech", pct(leech) + " of damage");
  var thorns = capped(c, "thorns", "fraction", LIMITS.thorns);
  if (thorns > 0) add("Thorns", pct(thorns) + " returned");
  var regen = capped(c, "regen", "fraction", LIMITS.regen);
  if (regen > 0) add("Regen", pct(regen) + " a round");
  var ex = bestEffect(c, "execute", "mult");
  if (ex) add("Execute", signed(num(ex.params.mult) - 1) + " under " + pct(num(ex.params.threshold)));
  var ls = null;
  eachEffect(c, function (e) { if (e.kind === "lastStand" && (!ls || num(e.params.mult) < num(ls.params.mult))) ls = e; });
  if (ls) add("Last stand", signed(num(ls.params.mult) - 1) + " under " + pct(num(ls.params.threshold)));
  var opp = opportunistMult(c, { effects: [{ kind: "speedMult", params: { mult: 0.5 } }] });
  if (opp > 1) add("Opportunist", signed(opp - 1) + " vs slowed");
  var st = steadiness(c);
  if (st.chance > 0) add("Footing", st.chance >= 1 ? "never lose a round" : pct(st.chance) + " keep a round");
  var res = resistanceList(c);
  if (res.length) add("Resist", res.map(function (r) {
    return r.kind + " " + (r.amount >= 1 ? "immune" : r.amount < 0 ? "weak " + pct(-r.amount) : pct(r.amount));
  }).join(", "));
  return rows;
}

// resistanceList is c's resistances, one entry per kind, family, or "all"
// named by a resist effect, totalled and limited as resistance() does:
// [{kind, amount}], strongest first.
function resistanceList(c) {
  var seen = {}, out = [];
  eachEffect(c, function (e) {
    if (e.kind !== "resist" || !e.params.kind || seen[e.params.kind]) return;
    seen[e.params.kind] = true;
    var total = 0;
    eachEffect(c, function (f) { if (f.kind === "resist" && f.params.kind === e.params.kind) total += num(f.params.amount); });
    if (!c.mob) total = Math.min(total, LIMITS.resist);
    if (total !== 0) out.push({ kind: e.params.kind, amount: total });
  });
  out.sort(function (a, b) { return b.amount - a.amount; });
  return out;
}

// ---------------------------------------------------------------------
// describeEffect: what an effect means, in words, for look and score.
// ---------------------------------------------------------------------
// spellFilterText words a caster effect's filter: " of evocation",
// " that heal", " of the divine".
function spellFilterText(p) {
  var t = "";
  if (p.kind === "damage") t += " that harm";
  if (p.kind === "heal") t += " that heal";
  if (p.school) t += " of " + p.school;
  if (p.deity) t += " of the " + p.deity + " god";
  if (p.branch && !p.school && !p.deity) t += " of the " + p.branch;
  return t;
}

function describeEffect(e) {
  var p = e.params || {};
  var pct = function (x) { return Math.round(num(x) * 100) + " percent"; };
  // units reads a power as a share of a standard swing at the owner's level.
  var units = function (x) {
    var v = num(x);
    if (v === 1) return "a full swing";
    if (v === 0.5) return "half a swing";
    return Math.round(v * 100) + " percent of a swing";
  };
  var tail = e.rounds > 0 ? " for " + e.rounds + " rounds" : "";
  switch (e.kind) {
    case "stat": return (num(p.amount) >= 0 ? "+" : "") + num(p.amount) + " " + (p.stat || "?") + tail;
    case "crit": return "A " + pct(p.chance) + " chance to critically hit for " + (num(p.mult) || 2) + "x damage" + tail;
    case "block": return "Blocks " + pct(p.chance) + " of swings" + tail;
    case "dodge": return (num(p.amount) >= 0 ? "+" : "") + pct(p.amount) + " dodge" + tail;
    case "offhandAttacks": return (num(p.amount) >= 0 ? "+" : "") + num(p.amount) + " attack" + (Math.abs(num(p.amount)) === 1 ? "" : "s") + " per round with a second weapon, at its speed" + tail;
    case "dualWield": return "Fights with a second weapon without penalty" + tail;
    case "greatWeapon": return Math.round((num(p.mult) - 1) * 100) + " percent more damage with a two-handed weapon" + tail;
    case "attacks": return (num(p.amount) >= 0 ? "+" : "") + num(p.amount) + " attack" + (Math.abs(num(p.amount)) === 1 ? "" : "s") + " per round, at your weapon's speed" + tail;
    case "defense": return "+" + Math.round(num(p.amount)) + " defense" + tail;
    case "protect": return num(p.mult) > 1
      ? "Takes " + Math.round((num(p.mult) - 1) * 100) + " percent more damage" + tail
      : "Takes " + Math.round((1 - num(p.mult)) * 100) + " percent less damage" + tail;
    case "speedMult": return "Swings at " + Math.round(num(p.mult) * 100) + " percent speed" + tail;
    case "dot": return (p.source === "ignite" ? "Burning: takes " : "Takes ") + num(p.damage) + " damage a round" + tail;
    case "attuned": return "Attuned: its numbers grow to match its wearer's level";
    case "proc": return "On hit (" + (p.verb || "magic") + ", " + (p.kind || "magic") + "): " + (p.chance === undefined || num(p.chance) >= 1 ? "always" : "a " + pct(p.chance) + " chance") + ", " + units(p.power) + " past armor" + tail;
    case "ignite": return "On hit (burning): " + units(p.power) + " a round for " + (num(p.rounds) || 3) + " rounds, stacking " + (num(p.maxStacks) || 1) + " deep" + tail;
    case "chill": return "A " + pct(p.chance) + " chance on each hit to slow the target to " + Math.round(num(p.mult) * 100) + " percent speed for " + (num(p.rounds) || 2) + " rounds" + tail;
    case "leech": return "Heals you for " + pct(p.fraction) + " of the damage you deal" + tail;
    case "thorns": return "Thorns (" + (p.verb || "thorns") + ", " + (p.kind || "pierce") + "): " + pct(p.fraction) + " of every hit that lands on you goes back to the attacker" + tail;
    case "soak": return "Every hit on you is " + units(p.power) + " smaller" + tail;
    case "lastStand": return "Below " + pct(p.threshold) + " health, take " + Math.round((1 - num(p.mult)) * 100) + " percent less damage" + tail;
    case "execute": return "Deal " + Math.round((num(p.mult) - 1) * 100) + " percent more damage to foes below " + pct(p.threshold) + " health" + tail;
    case "regen": return "Heals " + pct(p.fraction) + " of your health every round, even in a fight" + tail;
    case "added": return "Every hit adds " + (num(p.level) > 0 ? "about " + Math.round(unitAt(num(p.level)) * num(p.power)) : units(p.power)) + " " + (p.kind || "magic") + " damage" + tail;
    case "aura": return "Aura (" + (p.verb || "aura") + ", " + (p.kind || "magic") + "): " + units(p.power) + " to every foe fighting you, each round" + tail;
    case "blind": return "Blinded: " + pct(p.miss) + " of swings go wide" + tail;
    case "disarmed": return "Disarmed: fights bare-handed" + tail;
    case "damage": return Math.round((num(p.mult) - 1) * 100) + " percent more damage" + (p.weapon ? " with a weapon" : "") + tail;
    case "permanent": return "Permanent: no enchantment can change it again" + tail;
    case "baseDamage": return "The weapon's own damage is " + pct(p.fraction) + " higher (at least 1)" + tail;
    case "baseDefense": return "The armor's own defense is " + pct(p.fraction) + " higher (at least 1)" + tail;
    case "powerAttack": return Math.round((num(p.mult) - 1) * 100) + " percent more damage, " + pct(-num(p.dodge)) + " less dodge" + tail;
    case "critMult": return "Critical hits deal +" + num(p.amount) + "x damage" + tail;
    case "shieldBlock": return "Blocks " + pct(p.chance) + " more with a shield" + tail;
    case "finesse": return "Dexterity drives dagger damage" + tail;
    case "blindFighting": return "Blinded, misses " + Math.round((1 - num(p.mult)) * 100) + " percent less often" + tail;
    case "steady": return (num(p.chance) >= 1 ? "Never loses a round" : "A " + pct(p.chance) + " chance to keep a round that would be lost") +
      (p.slow !== undefined ? "; slows are " + Math.round((1 - num(p.slow)) * 100) + " percent weaker" : "") + tail;
    case "opportunist": return Math.round((num(p.mult) - 1) * 100) + " percent more damage to foes who are stunned or slowed" + tail;
    case "cooldownCut": return "Skill cooldowns " + num(p.rounds) + " round" + (num(p.rounds) === 1 ? "" : "s") + " shorter, and skills cost " +
      Math.min(100, 20 * num(p.rounds)) + " percent less stamina" + tail;
    case "unarmedMult": return "Bare hands and kicks hit " + Math.round((num(p.mult) - 1) * 100) + " percent harder" + tail;
    case "envenom": return "Envenomed: each hit poisons for " + units(p.power) + " a round, " + (num(p.maxStacks) || 1) + " doses deep" + tail;
    case "detectInvisible": return "Reveals the invisible" + tail;
    case "detectHidden": return "Reveals the hidden" + tail;
    case "invisible": return "Makes the wearer invisible" + tail;
    case "hidden": return "Hides the wearer" + tail;
    case "sneak": return "Moves unnoticed by any who cannot see the hidden" + tail;
    case "skill": return "";
    case "spell": return "";
    case "spellPower": return "Spells" + spellFilterText(p) + " are " + Math.round((num(p.mult) - 1) * 100) + " percent stronger" + tail;
    case "spellCrit": return "A " + pct(p.chance) + " chance for spells" + spellFilterText(p) + " to surge for " + (num(p.mult) || 1.5) + "x" + tail;
    case "penetration": return "Spells" + spellFilterText(p) + " are " + pct(p.amount) + " harder to resist" + tail;
    case "manaMax": return "+" + pct(p.amount) + " mana" + tail;
    case "manaFight": return pct(p.fraction) + " of your mana back each round in a fight" + tail;
    case "castCost":
      var parts = [];
      if (p.mana) parts.push("cost " + Math.round((1 - num(p.mana)) * 100) + " percent less mana");
      if (p.rounds) parts.push("long casts take " + num(p.rounds) + " round less");
      if (p.steady) parts.push("are not broken by blows");
      return "Spells" + spellFilterText(p) + " " + parts.join(", ") + tail;
    case "martyr": return "Healing others heals you for " + pct(p.fraction) + " of it" + tail;
    case "faithShield": return "A helpful spell leaves you taking " + Math.round((1 - (num(p.mult) || 0.85)) * 100) + " percent less damage for " + (num(p.rounds) || 3) + " rounds" + tail;
    case "resist": {
      var a = num(p.amount), what = (p.kind === "all" ? "every kind of" : p.kind || "?") + " damage";
      if (a >= 1) return "Immune to " + what + tail;
      if (a < 0) return "Weak to " + what + ": takes " + pct(-a) + " more" + tail;
      return "Resists " + what + " by " + pct(a) + tail;
    }
    default: return "";
  }
}

// ---------------------------------------------------------------------
// Skills (RULES 7.4): the one action per round beyond the auto-attack,
// and passives that improve with use. A rating from start to 100 is how
// much of the skill's potential a use delivers; it is never a chance of
// failure. Every character has every skill its level allows.
// Each active skill costs stamina (RULES 7.7, 2026-10-11), which paces it
// within a fight; a cooldown is kept only where a skill would otherwise
// chain (stuns, slows, disarms, blinding) or is a once-a-fight move.
// A taught skill names the trained stats a trainer wants first (stats);
// innate skills need none (RULES 7.4, 2026-10-11).
// ---------------------------------------------------------------------
// Fields beyond the engine's: damage (units of a standard swing, or a
// function of user and target), strike (a damage skill: see STRIKE),
// verb, stunChance (a lost round, scaled by
// rating), bleed, unavoidable (no dodge or block), unarmed (Iron Fist
// applies), check (a reason the use cannot go ahead, or ""), effects (what
// a landed use leaves, scaled by rating), self (a use on the user alone),
// ally (a use on a friend), rest (a passive that improves resting, not
// fighting).
var SKILLS = [
  { id: "kick", name: "Kick", level: 1, passive: false, target: "single", cooldown: 0, stamina: 20, start: 30, innate: true,
    damage: 1.8, strike: true, verb: "kick", unarmed: true,
    description: "A kick worth nearly two rounds of your weapon at full skill, and it needs no weapon in hand." },
  { id: "bash", stats: { strength: 12 }, name: "Bash", level: 3, passive: false, target: "single", cooldown: 3, stamina: 40, start: 25, innate: false, price: 300,
    damage: 0.5, verb: "bash", stunChance: 1.0,
    description: "Slam into them: some damage, and at full skill they lose their next round of swings." },
  { id: "twin", stats: { dexterity: 13 }, name: "Twin Strike", level: 5, passive: true, start: 20, innate: false, price: 1500,
    description: "A second attack in the same round, as often as your skill allows. Triple Strike follows." },
  { id: "rend", stats: { strength: 12 }, name: "Rend", level: 3, passive: false, target: "single", cooldown: 0, stamina: 30, start: 30, innate: false, price: 100,
    requires: [{ material: "wolf fang", count: 2 }], damage: 1.2, strike: true, verb: "rend", bleed: 0.4,
    description: "Tear at them like a beast: a round's worth and more now, and a wound that bleeds for three rounds. The trainer wants wolf fangs for it." },
  { id: "riposte", stats: { dexterity: 12 }, name: "Riposte", level: 4, passive: true, start: 20, innate: false, price: 500,
    description: "Turn a swing aside with your own blade: at full skill, one swing in five is blocked, weapon in hand." },

  // --- ROM 2.4 and friends (approved 2026-09-28) ---
  { id: "dodge", stats: { dexterity: 11 }, name: "Dodge", level: 1, passive: true, start: 20, innate: false, price: 200,
    description: "Slip aside: up to one more swing in ten dodged." },
  { id: "trip", stats: { dexterity: 11 }, name: "Trip", level: 2, passive: false, target: "single", cooldown: 3, stamina: 30, start: 25, innate: false, price: 200,
    damage: 0.3, verb: "trip", stunChance: 1.0, stunMessage: "They go down.",
    effects: function (u, t, scale) { return [{ on: "target", kind: "dodge", params: { amount: -0.05 }, rounds: 2 }]; },
    description: "Sweep their legs: a little damage, they are slow to dodge for two rounds, and at full skill they lose their next round." },
  { id: "hand_to_hand", stats: { strength: 11 }, name: "Hand to Hand", level: 2, passive: true, start: 20, innate: false, price: 300,
    description: "Fight with your fists: at full skill, bare hands hit twice as hard." },
  { id: "dirt_kicking", stats: { dexterity: 12 }, name: "Dirt Kicking", level: 3, passive: false, target: "single", cooldown: 4, stamina: 40, start: 25, innate: false, price: 300,
    verb: "dirt kick",
    effects: function (u, t, scale, out) {
      out.message = "They are blinded!";
      return [{ on: "target", kind: "blind", params: { miss: 0.4 * scale }, rounds: 2 }];
    },
    description: "Dirt in the eyes: for two rounds, up to four of their swings in ten go wide." },
  { id: "shield_block", stats: { constitution: 12 }, name: "Shield Block", level: 3, passive: true, start: 20, innate: false, price: 400,
    description: "Take blows on your shield: up to 15 percent more blocked, shield worn." },
  { id: "backstab", stats: { dexterity: 13 }, name: "Backstab", level: 4, passive: false, target: "single", cooldown: 0, stamina: 50, start: 25, innate: false, price: 500,
    damage: 4.0, strike: true, verb: "backstab",
    check: function (u, t) {
      if (!isDagger(wielded(u))) return "You need a dagger in hand to backstab.";
      if (u.fighting) return "You are too busy fighting to slip a blade in. Try circle.";
      if (t.fighting) return "They are fighting already, and watching for it.";
      return "";
    },
    description: "Open a fight with a dagger in the back: four rounds' worth at full skill. Only on someone not yet fighting." },
  { id: "fast_healing", stats: { constitution: 12 }, name: "Fast Healing", level: 4, passive: true, start: 20, innate: false, price: 400, rest: true,
    description: "Mend quickly: resting brings health back up to half again as fast. Improves while you rest." },
  { id: "enhanced_damage", stats: { strength: 13 }, name: "Enhanced Damage", level: 5, passive: true, start: 15, innate: false, price: 1000,
    description: "Put your weight behind it: up to 15 percent more damage on every swing." },
  { id: "hamstring", stats: { dexterity: 13 }, name: "Hamstring", level: 6, passive: false, target: "single", cooldown: 4, stamina: 40, start: 25, innate: false, price: 600,
    damage: 0.4, verb: "hamstring",
    effects: function (u, t, scale) { return [{ on: "target", kind: "speedMult", params: { mult: 1 - 0.3 * scale, source: "hamstring" }, rounds: 4 }]; },
    description: "Cut the tendon: some damage, and their swings slow to 70 percent for four rounds at full skill." },
  { id: "disarm", stats: { dexterity: 14 }, name: "Disarm", level: 6, passive: false, target: "single", cooldown: 6, stamina: 50, start: 20, innate: false, price: 700,
    verb: "disarm",
    check: function (u, t) {
      if (!wielded(u)) return "You need a weapon of your own to disarm with.";
      if (!wielded(t) && !armedNatural(t)) return "They have nothing to disarm.";
      return "";
    },
    effects: function (u, t, scale, out) {
      out.message = "Their weapon goes wide, and they fight bare-handed.";
      return [{ on: "target", kind: "disarmed", params: {}, rounds: 1 + Math.round(2 * scale) }];
    },
    description: "Knock their weapon aside: for up to three rounds they fight bare-handed, or at half strength if all they have is what they were born with." },
  { id: "feint", stats: { dexterity: 14 }, name: "Feint", level: 7, passive: false, target: "single", cooldown: 3, stamina: 30, start: 25, innate: false, price: 700,
    verb: "feint", unavoidable: true,
    effects: function (u, t, scale, out) {
      out.message = "They bite on the feint.";
      return [{ on: "target", kind: "dodge", params: { amount: -0.10 * scale }, rounds: 3 },
              { on: "target", kind: "block", params: { chance: -0.10 * scale }, rounds: 3 }];
    },
    description: "Draw their guard: for three rounds they dodge and block up to one swing in ten less." },
  { id: "berserk", stats: { strength: 15 }, name: "Berserk", level: 8, passive: false, target: "none", cooldown: 20, stamina: 60, start: 25, innate: false, price: 900,
    check: function (u) { return hasSource(u, "berserk") ? "You are already raging." : ""; },
    self: function (u, scale, out) {
      out.message = "You fly into a rage!";
      out.effects = [
        { on: "self", kind: "stat", params: { stat: "strength", amount: Math.max(1, Math.round(3 * scale)), source: "berserk" }, rounds: 6 },
        { on: "self", kind: "attacks", params: { amount: 0.5 * scale, source: "berserk" }, rounds: 6 },
        { on: "self", kind: "dodge", params: { amount: -0.05, source: "berserk" }, rounds: 6 }];
    },
    description: "Rage for six rounds: up to +3 Strength and half a swing more each round, but you dodge less." },
  { id: "envenom", stats: { dexterity: 15 }, name: "Envenom", level: 9, passive: false, target: "none", cooldown: 10, stamina: 40, start: 25, innate: false, price: 900,
    check: function (u) { return wielded(u) ? "" : "You need a blade to coat."; },
    self: function (u, scale, out) {
      out.message = "You coat your weapon in venom.";
      out.effects = [{ on: "self", kind: "envenom", params: { power: 0.15 * scale, rounds: 3, maxStacks: 2 }, rounds: 10 }];
    },
    description: "Poison your weapon for ten rounds: each hit leaves venom in the wound, two doses deep." },
  { id: "rescue", stats: { constitution: 14 }, name: "Rescue", level: 10, passive: false, target: "ally", cooldown: 2, stamina: 30, start: 30, innate: false, price: 1000,
    verb: "rescue",
    ally: function (u, t, scale, out) {
      out.taunt = true;
      out.effects = [{ on: "target", kind: "protect", params: { mult: 1 - 0.3 * scale }, rounds: 2 }];
    },
    description: "Step in front of a friend: everyone fighting them turns on you, and they take less damage for two rounds." },
  { id: "circle", stats: { dexterity: 16 }, name: "Circle", level: 10, passive: false, target: "single", cooldown: 2, stamina: 40, start: 20, innate: false, price: 1200,
    damage: 2.4, strike: true, verb: "circle",
    check: function (u, t) {
      if (!isDagger(wielded(u))) return "You need a dagger in hand to circle.";
      if (!u.fighting) return "Circle is for a fight under way; backstab opens one.";
      return "";
    },
    description: "Slip around them mid-fight and stab: nearly two and a half rounds' worth at full skill. Dagger only." },
  { id: "sunder", stats: { strength: 16 }, name: "Sunder", level: 12, passive: false, target: "single", cooldown: 6, stamina: 50, start: 20, innate: false, price: 1500,
    damage: 0.6, verb: "sunder",
    check: function (u) { return isHeavy(wielded(u)) ? "" : "You need a heavy weapon to sunder armor."; },
    effects: function (u, t, scale, out) {
      out.message = "Their armor buckles.";
      return [{ on: "target", kind: "defense", params: { amount: -0.5 * ARMOR_BASELINES.medium(t.level).defense * scale }, rounds: 5 }];
    },
    description: "Break their armor with a heavy weapon: a glancing blow, and for five rounds half a set of armor stops nothing." },
  { id: "second_wind", stats: { constitution: 16 }, name: "Second Wind", level: 12, passive: false, target: "none", cooldown: 30, stamina: 0, start: 25, innate: false, price: 1500,
    check: function (u) { return u.health >= u.healthMax ? "You are not winded." : ""; },
    self: function (u, scale, out) { out.heal = Math.max(1, Math.round(u.healthMax * (0.10 + 0.15 * scale))); },
    verb: "second wind",
    description: "Catch your breath, even mid-fight: up to a quarter of your health back at once." },
  { id: "whirlwind", stats: { strength: 17 }, name: "Whirlwind", level: 15, passive: false, target: "area", cooldown: 3, stamina: 60, start: 20, innate: false, price: 2500,
    damage: 1.2, strike: true, verb: "whirlwind",
    description: "Spin through them: more than a round's worth to every foe fighting you." },
  { id: "coup_de_grace", stats: { strength: 18 }, name: "Coup de Grace", level: 18, passive: false, target: "single", cooldown: 0, stamina: 50, start: 20, innate: false, price: 3000,
    damage: function (u, t) { return t.healthMax > 0 && t.health / t.healthMax < 0.25 ? 5.0 : 0.6; }, strike: true, verb: "coup de grace",
    description: "Finish them: five rounds' worth against a foe below a quarter of their health, little otherwise." },
  // Stealth (RULES 7.8): the engine reads hidden and sneak, and ends a
  // hiding of the character's own when it moves or a fight starts.
  { id: "hide", stats: { dexterity: 12 }, name: "Hide", level: 2, passive: false, target: "none", cooldown: 0, stamina: 20, start: 30, innate: false, price: 200,
    check: function (u) { return u.fighting ? "Not in the middle of a fight." : ""; },
    self: function (u, scale, out) {
      out.message = "You slip into the shadows.";
      out.effects = [{ on: "self", kind: "hidden", params: { source: "hide" }, rounds: 10 + Math.round(50 * scale) }];
    },
    verb: "hide",
    description: "Melt into the shadows where you stand: unseen, except by eyes that find the hidden, until you move or fight, for up to three minutes." },
  { id: "sneak", stats: { dexterity: 13 }, name: "Sneak", level: 4, passive: false, target: "none", cooldown: 0, stamina: 30, start: 30, innate: false, price: 500,
    // A toggle (7.8): on until used again, and turning it off is free.
    check: function (u) { return u.fighting && !sneaking(u) ? "Not in the middle of a fight." : ""; },
    self: function (u, scale, out) {
      if (sneaking(u)) {
        out.message = "You stop moving quietly.";
        out.end = [{ kind: "sneak", source: "sneak" }];
        out.stamina = 0;
        out.skills = {};
        return;
      }
      out.message = "You move quietly.";
      out.effects = [{ on: "self", kind: "sneak", params: { source: "sneak" }, rounds: 0 }];
    },
    verb: "sneak",
    description: "Move quietly until you sneak again: nobody who cannot see the hidden notices you come or go, and aggressive creatures pass you by." },
  { id: "triple", stats: { dexterity: 16 }, name: "Triple Strike", level: 12, passive: true, start: 15, innate: false, price: 4000,
    description: "A third attack in the same round, as often as your skill allows, on top of Twin Strike." },
  // Enchanting (RULES 5.6) is the rating behind every enchantment spell.
  // No trainer teaches it: the first enchantment cast gives it, and it
  // improves with each one, never in a fight.
  { id: "enchanting", stats: { intelligence: 20 }, name: "Enchanting", level: 25, passive: true, craft: true, start: 20, innate: false, price: 0,
    description: "Skill at making an enchantment take. Improves with every enchantment you cast." }
];

// hasSource reports whether c carries an effect tagged with source.
function hasSource(c, source) {
  var found = false;
  eachEffect(c, function (e) { if (e.params && e.params.source === source) found = true; });
  return found;
}

var SKILL_IMPROVE = { chance: 0.5, min: 1, max: 3 };  // per use, scaled by how far from 100

// STRIKE: damage skills (RULES 7.4, 2026-10-11) are measured in rounds of
// the user's own weapon (its damage and the added damage it carries,
// times speed, or a power unit if that is more), so they keep pace with gear; the rating takes them from
// floor to full rather than from nothing; they can crit, and Enhanced
// Damage adds to them. Utility skills (Bash, Trip, Hamstring, Sunder)
// keep a small flat damage on the standard swing: their point is what
// they do, not what they deal.
var STRIKE = { floor: 0.5 };

function strikeUnit(c, weapon) {
  var u = unit(c);
  if (!weapon || !weapon.weapon) return u;
  var w = weapon.weapon, swing = weaponDamage(weapon) * attunedScale(weapon, c);
  eachAdded(c, weapon, function (e, eu) { swing += eu * num(e.params.power); });
  return Math.max(u, swing * (num(w.speed) || 1));
}

function skillList() {
  return SKILLS.map(function (sk) {
    return { id: sk.id, name: sk.name, level: sk.level, passive: !!sk.passive, target: sk.target || "none",
      cooldown: sk.cooldown || 0, stamina: sk.stamina || 0, stats: sk.stats || {}, start: sk.start, innate: !!sk.innate, price: sk.price || 0, requires: sk.requires || [], description: sk.description || "" };
  });
}

function skillById(id) { for (var i = 0; i < SKILLS.length; i++) if (SKILLS[i].id === id) return SKILLS[i]; return null; }

function skillRating(c, id) {
  var r = 0;
  if (c.effects) for (var i = 0; i < c.effects.length; i++) {
    var e = c.effects[i];
    if (e.kind === "skill" && e.params.skill === id) r = num(e.state.effectiveness);
  }
  return r;
}

// improve rolls one step of improvement toward 100 and returns the new
// rating, or the old one.
function improve(rating) {
  if (rating >= 100) return 100;
  var room = (100 - rating) / 100;
  if (random.float() < SKILL_IMPROVE.chance * room) {
    return Math.min(100, rating + SKILL_IMPROVE.min + random.int(SKILL_IMPROVE.max - SKILL_IMPROVE.min + 1));
  }
  return rating;
}

// riposteBlock: Riposte grants up to 20 percent block, scaled by rating,
// only while a weapon is wielded.
function riposteBlock(c) {
  if (!c.equipment || !c.equipment.wield) return 0;
  return 0.2 * skillRating(c, "riposte") / 100;
}

// passiveSwings: Twin Strike grants rating/100 of an extra attack per
// round (a weapon's worth of swings, as feats give), and Triple Strike as
// much again, but only on top of Twin.
function passiveSwings(c) {
  var twin = skillRating(c, "twin") / 100;
  return twin + (twin > 0 ? skillRating(c, "triple") / 100 : 0);
}

// improvePassives: while fighting, every passive the character holds has a
// chance to improve each round.
function improvePassives(c) {
  var out = {};
  if (c.effects) for (var i = 0; i < c.effects.length; i++) {
    var e = c.effects[i];
    if (e.kind !== "skill") continue;
    var sk = skillById(e.params.skill);
    if (!sk || !sk.passive || sk.rest || sk.craft) continue;
    var next = improve(num(e.state.effectiveness));
    if (next !== num(e.state.effectiveness)) out[sk.id] = next;
  }
  return out;
}

// useSkill resolves an active skill through the same pipeline stages as a
// swing: dodge, block, roll, reduce; the rating scales the result.
// skillKind is the kind of a skill's damage (4.8): its own; a weapon
// strike's is the weapon's; anything else (a kick, a bash) is blunt.
function skillKind(sk, weapon) {
  if (sk.kind) return sk.kind;
  if (sk.strike && !sk.unarmed && weapon && weapon.weapon && weapon.weapon.kind) return weapon.weapon.kind;
  return "blunt";
}

function useSkill(user, target, skill, e) {
  var sk = skillById(skill.id);
  if (!sk) return { ok: false, message: "You have forgotten how." };
  if (sk.check) {
    var why = sk.check(user, target);
    if (why) return { ok: false, message: why };
  }
  var rating = num(e.state.effectiveness);
  var scale = rating / 100;
  var out = { ok: true, hit: true, stage: "", damage: 0, verb: sk.verb || sk.id, effects: [], skills: {} };
  var next = improve(rating);
  if (next !== rating) out.skills[sk.id] = next;
  // Relentless: cooldowns a round shorter, never below one, and a fifth
  // off the stamina for each such round.
  var cut = sumEffects(user, "cooldownCut", "rounds");
  if (cut > 0 && sk.cooldown > 0) out.cooldown = Math.max(1, sk.cooldown - cut);
  if (cut > 0 && sk.stamina > 0) out.stamina = Math.max(1, Math.round(sk.stamina * Math.max(0, 1 - 0.2 * cut)));
  if (sk.self) { sk.self(user, scale, out); return out; }
  if (!target) return out;
  if (sk.ally) { sk.ally(user, target, scale, out); return out; }
  if (!sk.unavoidable) {
    var dodge = dodgeChance(target);
    if (dodge > 0 && random.float() < dodge) { out.hit = false; out.stage = "dodge"; return out; }
    var block = blockChance(target, false);
    if (block > 0 && random.float() < block) { out.hit = false; out.stage = "block"; return out; }
  }
  var units = typeof sk.damage === "function" ? sk.damage(user, target) : (sk.damage || 0);
  if (units > 0) {
    var weapon = wielded(user);
    // A strike draws on the user's own weapon and delivers at least
    // STRIKE.floor of itself; a utility skill's damage is a flat aside.
    var potency = sk.strike ? STRIKE.floor + (1 - STRIKE.floor) * scale : scale;
    var dmg = spreadRoll((sk.strike ? strikeUnit(user, weapon) : unit(user)) * units * potency, 0.2) *
      strikeMult(user, weapon) * levelMult(user) * damageMult(user, weapon) * opportunistMult(user, target);
    if (sk.strike) {
      dmg *= 1 + 0.15 * skillRating(user, "enhanced_damage") / 100;
      var cm = critMult(user, weapon);
      if (cm > 1) { dmg *= cm; out.crit = true; out.message = "A telling blow!"; }
    }
    if (sk.unarmed) eachEffect(user, function (fx) { if (fx.kind === "unarmedMult") dmg *= num(fx.params.mult) || 1; });
    var K = kFor(user);
    dmg = dmg * K / (defense(target) + K) * protectMult(target);
    var kind = skillKind(sk, weapon);
    out.damage = Math.max(1, Math.round(resisted(target, kind, dmg)));
    if (immune(target, kind)) { out.damage = 0; out.crit = false; out.resisted = true; out.message = ""; }
  }
  if (sk.stunChance && random.float() < sk.stunChance * scale) {
    // Sure Footing and Unstoppable: a chance to keep the round.
    if (random.float() < steadiness(target).chance) {
      out.message = "They keep their feet.";
    } else {
      out.effects.push({ on: "target", kind: "speedMult", params: { mult: 0 }, rounds: 1 });
      out.message = sk.stunMessage || "They stagger.";
    }
  }
  if (sk.bleed) {
    if (out.damage > 0) out.effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(out.damage * sk.bleed)), kind: skillKind(sk, wielded(user)) }, rounds: 3 });
  }
  if (sk.effects) out.effects = out.effects.concat(sk.effects(user, target, scale, out) || []);
  return out;
}

// ---------------------------------------------------------------------
// Money (RULES 7.5): what a mob carries, in silver. 100 silver is a gold.
// A prototype's silver field overrides this.
// ---------------------------------------------------------------------
var MONEY = { perLevel: 3, spread: 0.5 };  // a level-4 mob carries about 12 silver, give or take half

function moneyFor(victim) {
  var base = MONEY.perLevel * (victim.level || 1);
  return Math.max(0, Math.round(spreadRoll(base, MONEY.spread)));
}

// ---------------------------------------------------------------------
// Loot (RULES 5.3): rarity is a count of effects on the instance. rollItem
// is asked for each plain weapon or armor a reset creates and for each
// extra item a death drops; lootRules says how many a death drops;
// itemTier names the tier for a count, for an item whose count changed or
// was saved under an older table. Item level gates which tiers and which
// affixes can roll. Affixes are ordinary effect kinds, so describeEffect
// words them and the pipeline runs them. Every number here is a starting
// value for playtest.
// ---------------------------------------------------------------------
// A tier rolls between min and max affixes; relic is open-ended above its
// min, but a roll gives at most its max. from is the lowest item level the
// tier rolls at. Weights are per thousand at item level 1 and at
// LOOT.topLevel, linear between and held above, from a reset and from a
// drop.
var RARITY = [
  { id: "common",    color: "",  min: 0, max: 0,  from: 1,  reset: [750, 600], drop: [450, 250], quality: [0, 0] },
  { id: "uncommon",  color: "G", min: 1, max: 2,  from: 1,  reset: [220, 250], drop: [400, 350], quality: [0.5, 1.0] },
  { id: "rare",      color: "B", min: 3, max: 4,  from: 5,  reset: [30, 100],  drop: [150, 230], quality: [0.6, 1.1] },
  { id: "epic",      color: "M", min: 5, max: 6,  from: 15, reset: [0, 40],    drop: [0, 120],   quality: [0.7, 1.2] },
  { id: "legendary", color: "Y", min: 7, max: 8,  from: 30, reset: [0, 9],     drop: [0, 40],    quality: [0.8, 1.3] },
  { id: "relic",     color: "R", min: 9, max: 10, from: 40, reset: [0, 1],     drop: [0, 10],    quality: [0.9, 1.4] },
];

// A named item (a prototype with intrinsic effects) has no tier, but its
// name is shown in a color of its own, apart from the tiers' (5.3). A
// material is shown in the color of the tier its rarity names, and a
// deterministic one, found in one place only, as a named item.
var NAMED_COLOR = "C";

function materialColor(rarity) {
  if (rarity === "deterministic") return NAMED_COLOR;
  for (var i = 0; i < RARITY.length; i++) if (RARITY[i].id === rarity) return RARITY[i].color;
  return "";
}

// dropChance is the chance of an extra drop at level 1 and at topLevel,
// linear between and held above.
var LOOT = { dropChance: [0.15, 0.25], topLevel: 40, bossDrops: 2, levelBand: 1 };

// An affix: the lowest item level it rolls at, the slots it goes on, how
// often it is drawn against the others, the words it lends a name (any
// affix may give either), and its effect at level L and quality q (1 is a
// full-strength roll). Slots are item slots, "wield" for weapons; "any"
// is every slot, and "armor" every armor slot but hold and light.
var ARMOR_ONLY = ["hold", "light"];
var AFFIXES = [
  // Item level 1.
  { id: "str",       from: 1,  on: ["any"], weight: 3, prefix: "mighty",     suffix: "of the bear",       effect: function (L, q) { return { kind: "stat", params: { stat: "strength", amount: statAmount(L, q) } }; } },
  { id: "dex",       from: 1,  on: ["any"], weight: 3, prefix: "nimble",     suffix: "of the cat",        effect: function (L, q) { return { kind: "stat", params: { stat: "dexterity", amount: statAmount(L, q) } }; } },
  { id: "con",       from: 1,  on: ["any"], weight: 3, prefix: "stalwart",   suffix: "of the ox",         effect: function (L, q) { return { kind: "stat", params: { stat: "constitution", amount: statAmount(L, q) } }; } },
  { id: "int",       from: 1,  on: ["any"], weight: 2, prefix: "clever",     suffix: "of the owl",        effect: function (L, q) { return { kind: "stat", params: { stat: "intelligence", amount: statAmount(L, q) } }; } },
  { id: "wis",       from: 1,  on: ["any"], weight: 2, prefix: "wise",       suffix: "of the sage",       effect: function (L, q) { return { kind: "stat", params: { stat: "wisdom", amount: statAmount(L, q) } }; } },
  { id: "damage",    from: 1,  on: ["wield"], weight: 3, prefix: "cruel",    suffix: "of slaying",        effect: function (L, q) { return { kind: "damage", params: { mult: round2(1 + 0.06 * q), weapon: true } }; } },
  { id: "defense",   from: 1,  on: ["armor"], weight: 2, prefix: "sturdy",   suffix: "of the turtle",     effect: function (L, q) { return { kind: "defense", params: { amount: round2(0.45 * growth(L) * q) } }; } },
  // A tenth more of the item's own number, at least 1, whatever the
  // roll's quality; only on slots that carry baseline defense.
  { id: "honed",     from: 1,  on: ["wield"], weight: 2, prefix: "honed",    suffix: "of the whetstone",  effect: function (L, q) { return { kind: "baseDamage", params: { fraction: 0.1 } }; } },
  { id: "reinforced", from: 1, on: Object.keys(SLOT_WEIGHT), weight: 2, prefix: "reinforced", suffix: "of the anvil", effect: function (L, q) { return { kind: "baseDefense", params: { fraction: 0.1 } }; } },
  // Item level 5.
  { id: "crit",      from: 5,  on: ["wield", "hands", "finger"], weight: 2, prefix: "keen", suffix: "of precision", effect: function (L, q) { return { kind: "crit", params: { chance: round2(0.04 * q), mult: 2 } }; } },
  { id: "dodge",     from: 5,  on: ["feet", "legs", "body"], weight: 2, prefix: "elusive", suffix: "of evasion",   effect: function (L, q) { return { kind: "dodge", params: { amount: round2(0.02 * q) } }; } },
  { id: "protect",   from: 5,  on: ["body", "shield", "head", "shoulders", "arms"], weight: 2, prefix: "warded", suffix: "of warding", effect: function (L, q) { return { kind: "protect", params: { mult: round2(1 - 0.04 * q) } }; } },
  { id: "steady",    from: 5,  on: ["any"], weight: 1, prefix: "steadfast",  suffix: "of the mountain",   effect: function (L, q) { return { kind: "steady", params: { chance: round2(0.2 * q) } }; } },
  { id: "fire",      from: 5,  on: ["wield"], weight: 2, prefix: "flaming",  suffix: "of flame",          effect: function (L, q) { return addedAffix("fire", L, q); } },
  { id: "cold",      from: 5,  on: ["wield"], weight: 2, prefix: "icy",      suffix: "of ice",            effect: function (L, q) { return addedAffix("cold", L, q); } },
  { id: "lightning", from: 5,  on: ["wield"], weight: 2, prefix: "sparking", suffix: "of sparks",         effect: function (L, q) { return addedAffix("lightning", L, q); } },
  { id: "poison",    from: 5,  on: ["wield"], weight: 2, prefix: "noxious",  suffix: "of the asp",        effect: function (L, q) { return addedAffix("poison", L, q); } },
  // Item level 10.
  { id: "leech",     from: 10, on: ["wield", "neck", "finger"], weight: 1, prefix: "thirsting", suffix: "of the leech", effect: function (L, q) { return { kind: "leech", params: { fraction: round2(0.04 * q) } }; } },
  { id: "ignite",    from: 10, on: ["wield"], weight: 1, prefix: "smoldering", suffix: "of embers",       effect: function (L, q) { return { kind: "ignite", params: { power: round2(0.1 * q), rounds: 3, maxStacks: 2, level: L } }; } },
  { id: "chill",     from: 10, on: ["wield"], weight: 1, prefix: "frozen",     suffix: "of frost",         effect: function (L, q) { return { kind: "chill", params: { chance: round2(0.1 * q), mult: 0.8, rounds: 2 } }; } },
  { id: "venom",     from: 10, on: ["wield"], weight: 1, prefix: "venomous",   suffix: "of venom",         effect: function (L, q) { return { kind: "envenom", params: { power: round2(0.05 * q), maxStacks: 2, level: L } }; } },
  { id: "thorns",    from: 10, on: ["body", "shield"], weight: 1, prefix: "thorned", suffix: "of thorns",   effect: function (L, q) { return { kind: "thorns", params: { fraction: round2(0.08 * q), verb: "thorns" } }; } },
  { id: "soak",      from: 10, on: ["body", "shield", "belt", "legs"], weight: 1, prefix: "stony", suffix: "of stone", effect: function (L, q) { return { kind: "soak", params: { power: round2(0.05 * q) } }; } },
  // Item level 15.
  { id: "critMult",  from: 15, on: ["wield", "neck", "finger"], weight: 1, prefix: "ruinous", suffix: "of ruin", effect: function (L, q) { return { kind: "critMult", params: { amount: round2(0.25 * q) } }; } },
  { id: "jackal",    from: 15, on: ["wield"], weight: 1, prefix: "cunning",    suffix: "of the jackal",    effect: function (L, q) { return { kind: "opportunist", params: { mult: round2(1 + 0.1 * q) } }; } },
  { id: "bulwark",   from: 15, on: ["shield"], weight: 1, prefix: "guarding",  suffix: "of the bulwark",   effect: function (L, q) { return { kind: "shieldBlock", params: { chance: round2(0.03 * q) } }; } },
  { id: "regen",     from: 15, on: ["neck", "finger", "body", "belt"], weight: 1, prefix: "mending", suffix: "of mending", effect: function (L, q) { return { kind: "regen", params: { fraction: round2(0.005 * q) } }; } },
  // Item level 20.
  { id: "haste",     from: 20, on: ["wield", "hands"], weight: 1, prefix: "swift", suffix: "of haste",       effect: function (L, q) { return { kind: "attacks", params: { amount: round2(0.15 * q) } }; } },
  { id: "thunder",   from: 20, on: ["wield"], weight: 1, prefix: "thundering", suffix: "of thunder",       effect: function (L, q) { return { kind: "proc", params: { kind: "lightning", chance: 0.1, power: round2(0.3 * q), verb: "thunderclap", level: L } }; } },
  { id: "execute",   from: 20, on: ["wield"], weight: 1, prefix: "merciless",  suffix: "of mercy",         effect: function (L, q) { return { kind: "execute", params: { threshold: 0.25, mult: round2(1 + 0.15 * q) } }; } },
  { id: "brambles",  from: 20, on: ["body", "shield"], weight: 1, prefix: "briared", suffix: "of brambles", effect: function (L, q) { return { kind: "aura", params: { kind: "pierce", power: round2(0.05 * q), verb: "brambles", level: L } }; } },
  { id: "last",      from: 20, on: ["body", "neck"], weight: 1, prefix: "defiant", suffix: "of the last stand", effect: function (L, q) { return { kind: "lastStand", params: { threshold: 0.3, mult: round2(1 - 0.15 * q) } }; } },
  // Item level 30.
  { id: "focus",     from: 30, on: ["any"], weight: 0.5, prefix: "focused",    suffix: "of focus",         effect: function (L, q) { return { kind: "cooldownCut", params: { rounds: 1 } }; } },
  { id: "sight",     from: 30, on: ["head", "face"], weight: 0.5, prefix: "clear-eyed", suffix: "of true sight", effect: function (L, q) { return { kind: "detectInvisible", params: {} }; } },
  { id: "hawk",      from: 30, on: ["head", "face"], weight: 0.5, prefix: "watchful", suffix: "of the hawk",   effect: function (L, q) { return { kind: "detectHidden", params: {} }; } },
];

function round2(x) { return Math.round(x * 100) / 100; }

// addedAffix is added damage of kind on every hit (5.3), sized so a
// full-quality roll makes up what the flatter weapon curve (5.2) leaves
// a weapon of item level L short of the power unit. Like every damage
// affix it carries the level it was rolled at, which sizes it.
function addedAffix(kind, L, q) {
  return { kind: "added", params: { kind: kind, power: round2(q * (1 - weaponShare(L))), level: L } };
}

// statAmount: one point at low level, a point more every ten levels, more
// on a good roll.
function statAmount(L, q) { return Math.max(1, Math.round((1 + Math.floor((L || 1) / 10)) * q)); }

// byLevel is a [at level 1, at topLevel] pair read at level L.
function byLevel(pair, L) {
  var t = Math.min(1, Math.max(0, ((L || 1) - 1) / (LOOT.topLevel - 1)));
  return pair[0] + (pair[1] - pair[0]) * t;
}

// itemSlot is the slot an affix's "on" list names for an item.
function itemSlot(it) { return it.type === "weapon" ? "wield" : (it.slot || ""); }

// affixFits reports whether affix a may go on an item in slot at item
// level L.
function affixFits(a, slot, L) {
  if (a.from > L) return false;
  for (var i = 0; i < a.on.length; i++) {
    var on = a.on[i];
    if (on === "any" || on === slot) return true;
    if (on === "armor" && slot !== "wield" && ARMOR_ONLY.indexOf(slot) < 0) return true;
  }
  return false;
}

function weightedPick(list, weightOf) {
  var total = 0;
  for (var i = 0; i < list.length; i++) total += weightOf(list[i]);
  var r = random.float() * total;
  for (var j = 0; j < list.length; j++) {
    r -= weightOf(list[j]);
    if (r < 0) return list[j];
  }
  return list[list.length - 1];
}

function rollItem(it, ctx) {
  var source = ctx.source === "drop" ? "drop" : "reset";
  var L = Math.max(ctx.level || 1, it.level || 1);
  // Tiers above the item level are removed before the draw.
  var tiers = RARITY.filter(function (t) { return t.from <= L; });
  var tier = weightedPick(tiers, function (t) { return byLevel(t[source], L); });
  var count = tier.min + random.int(tier.max - tier.min + 1);
  if (count === 0) return { rarity: "" };
  var slot = itemSlot(it);
  var all = AFFIXES.filter(function (a) { return affixFits(a, slot, L); });
  var pool = all.slice();
  var rolls = [];
  for (var n = 0; n < count; n++) {
    if (pool.length === 0) pool = all.slice(); // past the pool's size, affixes repeat and stack
    var a = weightedPick(pool, function (x) { return x.weight; });
    pool = pool.filter(function (x) { return x !== a; });
    var q = tier.quality[0] + random.float() * (tier.quality[1] - tier.quality[0]);
    rolls.push({ affix: a, q: q, effect: a.effect(L, q) });
  }
  // The strongest roll names the suffix, the next strongest the prefix.
  var named = rolls.slice().sort(function (x, y) { return y.q - x.q; });
  var suffix = named[0].affix.suffix;
  var prefix = "";
  for (var i = 1; i < named.length && !prefix; i++) if (named[i].affix !== named[0].affix) prefix = named[i].affix.prefix;
  return { rarity: tier.id, color: tier.color, prefix: prefix, suffix: suffix,
    effects: rolls.map(function (r) { return r.effect; }) };
}

// itemTier names the tier for the count of effects an item carries; the
// last tier takes every count above its min.
function itemTier(it) {
  var count = effectCount(it.effects);
  var tier = RARITY[0];
  for (var i = 0; i < RARITY.length; i++) if (count >= RARITY[i].min) tier = RARITY[i];
  if (tier.min === 0) return { rarity: "" };
  return { rarity: tier.id, color: tier.color };
}

function lootRules(victim) {
  var flags = (victim.mob && victim.mob.flags) || [];
  if (flags.indexOf("boss") >= 0) return { drops: LOOT.bossDrops, levelBand: LOOT.levelBand };
  return { drops: random.float() < byLevel(LOOT.dropChance, victim.level) ? 1 : 0, levelBand: LOOT.levelBand };
}

// ---------------------------------------------------------------------
// Expeditions: a run of fights on paper in an area near the character's
// level, auto-attack only. Sleeps to full under restBelow, heads home
// after a win under retreatBelow. Starting values for playtest.
// ---------------------------------------------------------------------
var EXPEDITION = { levelBand: 3, fights: 10, maxFights: 30, restBelow: 0.5, retreatBelow: 0.2 };

function expeditionRules(c) { return EXPEDITION; }

// ---------------------------------------------------------------------
// Quests (RULES 7.6). The engine draws a kill or fetch task against a
// live mob near the player's level and runs the clock; these hooks set
// the band, the timers (in rounds), and what finishing pays.
// ---------------------------------------------------------------------
var QUEST = {
  levelBand: 3,        // targets within this many levels of the player
  minutes: 15,         // time to finish
  cooldownMinutes: 5,  // wait after finishing or failing
  quitMinutes: 10,     // wait after giving up, so quitting is not a reroll
  pointsBase: 8,       // quest points for a level-0 task ...
  pointsPerLevel: 2,   // ... plus this per level of the target
  silverPerLevel: 20   // coin on top, in silver
};

function questRules(c) {
  var perMinute = 60 / P.roundSeconds;
  return { levelBand: QUEST.levelBand, rounds: QUEST.minutes * perMinute,
           cooldown: QUEST.cooldownMinutes * perMinute, quitCooldown: QUEST.quitMinutes * perMinute };
}

// questReward is asked once, when the quest is handed out; the numbers
// are shown to the player and paid on completion. q has kind ("kill" or
// "fetch"), target (vnum), name, level, area, room, rounds.
function questReward(c, q) {
  var points = QUEST.pointsBase + QUEST.pointsPerLevel * (q.level || 1);
  return { points: Math.max(1, Math.round(points)), silver: QUEST.silverPerLevel * (q.level || 1), xp: 0, message: "" };
}

// ---------------------------------------------------------------------
// Damage words, after ROM's dam_message, keyed to the share of the
// target's maximum health one hit takes so the ladder reads the same at
// every level. "max" is the fraction the rung covers up to; the last rung
// covers everything above. Edit freely.
// ---------------------------------------------------------------------
function damageWords() {
  return [
    { max: 0.02, word: "scratch" },
    { max: 0.04, word: "graze" },
    { max: 0.07, word: "hit" },
    { max: 0.10, word: "injure" },
    { max: 0.14, word: "wound" },
    { max: 0.18, word: "maul", shout: true },
    { max: 0.22, word: "decimate", shout: true },
    { max: 0.27, word: "devastate", shout: true },
    { max: 0.32, word: "maim", shout: true },
    { max: 0.38, word: "MUTILATE", shout: true },
    { max: 0.45, word: "DISEMBOWEL", shout: true },
    { max: 0.55, word: "DISMEMBER", shout: true },
    { max: 0.65, word: "MASSACRE", shout: true },
    { max: 0.80, word: "MANGLE", shout: true },
    { max: 1.00, word: "*** DEMOLISH ***", shout: true },
    { max: 1.50, word: "=== OBLITERATE ===", shout: true },
    { max: 2.50, word: ">>> ANNIHILATE <<<", shout: true },
    { max: 99,   word: "do UNSPEAKABLE things to", shout: true }
  ];
}


// ---------------------------------------------------------------------
// Locks. pick asks pickChance(picker, door) for the chance, 0 to 1, that
// one try opens a lock. Half at an average dexterity, better with a nimble
// hand, never certain. A pickproof door never opens whatever this says.
// ---------------------------------------------------------------------
function pickChance(c, door) {
  return Math.min(0.9, Math.max(0.1, 0.5 * mult(c, "dexterity")));
}

// ---------------------------------------------------------------------
// Enchanting (RULES 5.6). An enchantment is a spell cast on an item in
// the caster's inventory. enchantBlocker says, before any mana is spent,
// why it cannot go on that item; resolveEnchant decides the attempt. The
// Enchanting rating sets the chance, and each effect already on the item
// makes it harder, gently up to ENCHANT.easy effects and steeply past
// them. A failure does nothing, strips every affix, or, past easy,
// destroys the item. Starting values for playtest.
// ---------------------------------------------------------------------
var ENCHANT = { easy: 12, perEffect: 3, pastEasy: 8, destroyR: 25, stripEasy: 0.05, stripHard: 0.10, min: 0.05, max: 0.10 };

// effectCount is the effects that count toward an item's tier and its
// enchanting difficulty: all of them but Permanency's mark.
function effectCount(effects) {
  var n = 0;
  for (var i = 0; i < (effects || []).length; i++) if (effects[i].kind !== "permanent") n++;
  return n;
}

// itemLevel is an instance's item level: its prototype's, or the level
// a deeper roll recorded on an affix (5.3).
function itemLevel(it) {
  var L = it.level || 1;
  var ap = it.applied || [];
  for (var i = 0; i < ap.length; i++) L = Math.max(L, num(ap[i].params && ap[i].params.level));
  return L;
}

// maxEffects is the most effects enchanting may bring an item of level
// L to: the top of the highest tier L could roll, open from relic's.
function maxEffects(L) {
  var top = 0;
  for (var i = 0; i < RARITY.length; i++) {
    if (RARITY[i].from > L) continue;
    if (i === RARITY.length - 1) return Infinity;
    top = RARITY[i].max;
  }
  return top;
}

function enchantBlocker(c, it, sp) {
  var e = spellById(sp.id);
  if (!e || !e.enchant) return "That is not an enchantment.";
  if (hasEffect(it, "permanent")) return "It is permanent; no enchantment will change it.";
  var n = effectCount(it.effects);
  switch (e.enchant) {
    case "weapon":
      if (it.type !== "weapon" || !it.weapon || num(it.weapon.damage) <= 0) return "That is not a weapon.";
      break;
    case "armor":
      if (it.type !== "armor" || !it.armor || num(it.armor.defense) <= 0) return "That has no defense to strengthen.";
      break;
    case "restore":
      if ((it.applied || []).length === 0) return "It is already as it was made.";
      return "";
    case "permanency":
      return "";
  }
  if (n + 1 > maxEffects(itemLevel(it))) return "It cannot hold another effect at its level.";
  return "";
}

function capitalize(t) { return t ? t.charAt(0).toUpperCase() + t.slice(1) : t; }

// enchantChance is the chance an enchantment of difficulty d takes, at
// rating r, on an item already carrying n effects.
function enchantChance(r, d, n) {
  var e = Math.max(0, n - ENCHANT.easy);
  return r / (r + d + ENCHANT.perEffect * n + ENCHANT.pastEasy * e * e);
}

// tierFor is the tier fields for an item whose effects are now list: a
// named item (intrinsic effects) stays without a tier.
function tierFor(it, list) {
  var intrinsic = (it.effects || []).length - (it.applied || []).length;
  if (intrinsic > 0) return { rarity: "", color: "" };
  var t = itemTier({ effects: list });
  return { rarity: t.rarity || "", color: t.color || "" };
}

function resolveEnchant(c, it, sp) {
  var e = spellById(sp.id);
  var why = enchantBlocker(c, it, sp);
  if (why) return { ok: false, message: why };
  var rating = skillRating(c, "enchanting");
  var skills = {};
  skills.enchanting = improve(rating);
  var n = effectCount(it.effects);
  var applied = (it.applied || []).map(function (x) { return { kind: x.kind, params: x.params }; });
  var name = it.name;
  if (random.float() < enchantChance(rating, e.difficulty, n)) {
    var out = { ok: true, outcome: "success", skills: skills, prefix: it.prefix || "", suffix: it.suffix || "" };
    switch (e.enchant) {
      case "weapon":
      case "armor":
        var frac = round2(ENCHANT.min + random.float() * (ENCHANT.max - ENCHANT.min));
        applied.push({ kind: e.enchant === "weapon" ? "baseDamage" : "baseDefense", params: { fraction: frac, enchanted: true } });
        out.message = capitalize(name) + " glows blue: its " + (e.enchant === "weapon" ? "damage" : "defense") + " rises by " + Math.round(frac * 100) + " percent.";
        break;
      case "restore":
        applied = [];
        out.prefix = ""; out.suffix = "";
        out.message = capitalize(name) + " shimmers and is plain again, as it was made.";
        break;
      case "permanency":
        applied.push({ kind: "permanent", params: {} });
        out.message = capitalize(name) + " hums, and settles: it will not change again.";
        break;
    }
    var t = tierFor(it, applied);
    out.effects = applied; out.rarity = t.rarity; out.color = t.color;
    return out;
  }
  // Failure: destroyed only past easy, then stripped, else nothing.
  var ex = Math.max(0, n - ENCHANT.easy);
  var destroy = ex / (ex + ENCHANT.destroyR);
  var strip = n > ENCHANT.easy ? ENCHANT.stripHard : ENCHANT.stripEasy;
  var roll = random.float();
  if (roll < destroy) {
    return { ok: true, outcome: "destroy", skills: skills, message: capitalize(name) + " shudders, cracks, and falls to dust." };
  }
  if (roll < destroy + strip && applied.length > 0) {
    return { ok: true, outcome: "strip", skills: skills, effects: [], rarity: "", color: "", prefix: "", suffix: "",
      message: capitalize(name) + " flares and goes dull: every enchantment on it is gone." };
  }
  return { ok: true, outcome: "nothing", skills: skills, message: "The magic slides off " + name + "; nothing changes." };
}
