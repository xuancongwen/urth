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
// statPoints, health, healthMax, speed, equipment{slot: item}, effects[],
// isPlayer, fighting, and for mobs mob{vnum, flags, health, xp, attack,
// armor, effects}. Items carry vnum, name, type, slot, level, baseline,
// weapon{damage, spread, speed, hands, kind, verb}, armor{defense, spread},
// effects[]. Effects are {kind, params, state, rounds}.

// ---------------------------------------------------------------------
// Parameters (RULES 3.3, 4.3, 4.5, 4.6, 7.1)
// ---------------------------------------------------------------------
var P = {
  statBaseline: 10,     // a stat of 10 multiplies by exactly 1
  statK: 0.05,          // multiplier per point above or below 10
  statBand: 0.10,       // ±10% jitter on every stat multiplier (4.2)
  levelK: 0.02,         // slight per-level multiplier (7.2)
  healthBase: 40, healthPerLevel: 10,  // 9 to 11 rounds for an even fight at every level
  mobDamage: 0.45,      // a mob's natural attack as a fraction of a standard weapon (RULES 8)
  dodgeBase: 0.03,      // intrinsic dodge before stats and effects (4.3)
  blockBase: 0.0,       // nothing blocks without a shield or a feat
  K: 20,                // reduction = D / (D + K); K is a unit, not a cap
  regenRounds: 15,      // rounds from empty to full when resting (30 s)
  xpBase: 50, xpR: 1.15, // cost(N) = xpBase * N * xpR^(N-1)
  xpPerLevelKill: 10,   // an even kill is worth 10 * victim level
  deathXpFraction: 0.20, deathXpLevelCap: 0.50,
  corpseRounds: 600,    // 20 minutes at 2 s rounds
  respawnHealth: 0.25,
  pointsAtCreation: 6, pointsPerLevel: 2,
  featsPerLevel: 1,     // feat picks per level (7.2: one or two)
  manaEnabled: false    // D18: mana stays in the engine, off in the rules
};

var STATS = ["strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"];

// ---------------------------------------------------------------------
// Baselines by level (RULES 5.2, 8). An item or mob prototype names its
// level and baseline; anything the builder stated explicitly wins.
// ---------------------------------------------------------------------
var WEAPON_BASELINES = {
  standard: function (L) { return { damage: 4 + 2 * L, speed: 1.0, spread: 0.2, verb: "hit" }; },
  dagger:   function (L) { return { damage: Math.max(1, Math.round((4 + 2 * L) / 2)), speed: 2.0, spread: 0.1, verb: "stab" }; },
  heavy:    function (L) { return { damage: (4 + 2 * L) * 2, speed: 0.5, spread: 0.4, verb: "smash" }; },
  unarmed:  function (L) { return { damage: 1 + Math.floor(L / 2), speed: 1.0, spread: 0.2, verb: "punch" }; }
};

var ARMOR_BASELINES = {
  medium: function (L) { return { defense: L + 2, spread: 0.2 }; },
  light:  function (L) { return { defense: 0.7 * (L + 2), spread: 0.1 }; },
  heavy:  function (L) { return { defense: 1.3 * (L + 2), spread: 0.3 }; }
};

// A set's total D is split across slots by these weights (5.2). Slots not
// listed (shield, hands, wrist, finger, neck, waist, light, hold) carry no
// baseline defense; they are for effects, or for stated numbers.
var SLOT_WEIGHT = { body: 0.40, legs: 0.20, head: 0.15, arms: 0.15, feet: 0.10 };

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
      hands:  w.hands  || 1,
      kind:   w.kind   || "",
      verb:   w.verb   || base.verb
    };
  }
  if (p.type === "armor") {
    var a = p.armor || {};
    var ab = (ARMOR_BASELINES[p.baseline] || ARMOR_BASELINES.medium)(L);
    var weight = SLOT_WEIGHT[p.slot] || 0;
    out.armor = {
      defense: a.defense || Math.round(ab.defense * weight),
      spread:  a.spread  || ab.spread
    };
  }
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
  var base = WEAPON_BASELINES.standard(L);
  base.damage = Math.max(1, Math.round(base.damage * P.mobDamage));
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
      hands:  0, kind: atk.kind || "",
      verb:   atk.verb || "hit"
    },
    armor: { defense: arm.defense || ab.defense, spread: arm.spread || ab.spread }
  };
}

// ---------------------------------------------------------------------
// Effects (RULES 5.4). A kind is a function of the pipeline context; the
// engine stores {kind, params, state, rounds} and never looks inside.
// Kinds in play so far: stat, crit, dodge, block, attacks.
// ---------------------------------------------------------------------
function eachEffect(c, fn) {
  var i, s;
  if (c.effects) for (i = 0; i < c.effects.length; i++) fn(c.effects[i]);
  if (c.mob && c.mob.effects) for (i = 0; i < c.mob.effects.length; i++) fn(c.mob.effects[i]);
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.effects) for (i = 0; i < it.effects.length; i++) fn(it.effects[i]);
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
// The attack pipeline (RULES 4.1 to 4.4, 5.4): prepare is the engine's
// swing meter; here roll, modify, dodge, block, reduce.
// ---------------------------------------------------------------------
function naturalAttack(c) {
  if (c.mob && c.mob.attack) return c.mob.attack;
  return WEAPON_BASELINES.unarmed(c.level || 1);
}

function resolveAttack(att, def, weapon, round) {
  var w = weapon && weapon.weapon ? weapon.weapon : naturalAttack(att);
  var verb = w.verb || "punch";

  // Dodge: intrinsic, scaled by Dexterity, shifted by effects (light or
  // heavy armor, feats).
  var dodge = P.dodgeBase * mult(def, "dexterity", true) + sumEffects(def, "dodge", "amount");
  if (dodge > 0 && random.float() < dodge) return { hit: false, damage: 0, crit: false, verb: verb, stage: "dodge" };

  // Block: nothing intrinsic; a shield or a feat grants it, Strength
  // scales it.
  var block = P.blockBase + sumEffects(def, "block", "chance") * mult(def, "strength", true);
  if (block > 0 && random.float() < block) return { hit: false, damage: 0, crit: false, verb: verb, stage: "block" };

  // Roll: the weapon's spread, then Strength and level.
  var dmg = spreadRoll(w.damage, w.spread) * mult(att, "strength", true) * levelMult(att);

  // Modify: crits exist only as effects (4.2).
  var crit = false;
  var ce = bestEffect(att, "crit", "chance");
  if (ce && random.float() < num(ce.params.chance)) { dmg *= (num(ce.params.mult) || 2); crit = true; }

  // Reduce: D / (D + K), with D from armor or natural hide, Constitution,
  // and level.
  var D = defense(def);
  dmg = dmg * P.K / (D + P.K);

  return { hit: true, damage: Math.max(0, Math.round(dmg)), crit: crit, verb: verb, stage: "" };
}

// defense is the defender's D for this hit: the sum of worn armor's draws,
// or the mob's natural armor if nothing is worn, times Constitution and
// level.
function defense(c) {
  var D = 0, worn = false, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.armor && it.armor.defense) { D += spreadRoll(it.armor.defense, it.armor.spread); worn = true; }
  }
  if (!worn && c.mob && c.mob.armor) D = spreadRoll(c.mob.armor.defense, c.mob.armor.spread);
  return Math.max(0, D * mult(c, "constitution", true) * levelMult(c));
}

// ---------------------------------------------------------------------
// Derived values, regeneration, progression (RULES 4.4, 4.5, 7.1, 7.2)
// ---------------------------------------------------------------------
function derivedStats(c) {
  var healthMax = (c.mob && c.mob.health > 0)
    ? c.mob.health
    : Math.round((P.healthBase + P.healthPerLevel * (c.level || 1)) * mult(c, "constitution") * levelMult(c));
  var w = c.equipment && c.equipment.wield && c.equipment.wield.weapon ? c.equipment.wield.weapon : naturalAttack(c);
  var speed = (w.speed || 1) * mult(c, "dexterity") + sumEffects(c, "attacks", "amount");
  return {
    healthMax: Math.max(1, healthMax),
    manaMax: P.manaEnabled ? 10 + 2 * c.level : 0,
    speed: Math.max(0.1, speed)
  };
}

function onTick(c) {
  if (c.fighting) return { healthDelta: 0, manaDelta: 0 };
  return { healthDelta: Math.max(1, Math.round(c.healthMax / P.regenRounds)), manaDelta: 0 };
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
  { id: "toughness", name: "Toughness", level: 1, description: "+2 Constitution.",
    effect: { kind: "stat", params: { stat: "constitution", amount: 2 } } },
  { id: "brawn", name: "Brawn", level: 1, description: "+2 Strength.",
    effect: { kind: "stat", params: { stat: "strength", amount: 2 } } },
  { id: "nimble", name: "Nimble", level: 1, description: "+2 Dexterity.",
    effect: { kind: "stat", params: { stat: "dexterity", amount: 2 } } },
  { id: "keen", name: "Keen Edge", level: 3, description: "A 5 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.05, mult: 2 } } },
  { id: "parry", name: "Parry", level: 3, description: "Block one swing in ten with your weapon.",
    effect: { kind: "block", params: { chance: 0.10 } } },
  { id: "evasion", name: "Evasion", level: 5, description: "Dodge 5 percent more often.",
    effect: { kind: "dodge", params: { amount: 0.05 } } },
  { id: "second_attack", name: "Second Attack", level: 5, description: "One extra swing every round.",
    effect: { kind: "attacks", params: { amount: 1 } } },
  { id: "deadly", name: "Deadly Edge", level: 8, requires: ["keen"], description: "A 10 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.10, mult: 2 } } },
  { id: "iron_skin", name: "Iron Skin", level: 10, requires: ["toughness"], description: "+4 Constitution.",
    effect: { kind: "stat", params: { stat: "constitution", amount: 4 } } },
  { id: "third_attack", name: "Third Attack", level: 12, requires: ["second_attack"], description: "Another extra swing every round.",
    effect: { kind: "attacks", params: { amount: 1 } } }
];

function featList() { return FEATS; }

// standardKit is what "a level N fighter" wears in the simulator (RULES
// 4.5): a standard weapon and a medium armor set, all at level N.
function standardKit(level) {
  return [
    { name: "a standard sword", type: "weapon", slot: "wield", baseline: "standard", weapon: { hands: 1, verb: "slash" } },
    { name: "a standard breastplate", type: "armor", slot: "body", baseline: "medium" },
    { name: "standard greaves", type: "armor", slot: "legs", baseline: "medium" },
    { name: "a standard helm", type: "armor", slot: "head", baseline: "medium" },
    { name: "standard bracers", type: "armor", slot: "arms", baseline: "medium" },
    { name: "standard boots", type: "armor", slot: "feet", baseline: "medium" }
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
