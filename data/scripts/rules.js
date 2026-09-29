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
  levelGrowth: 1.10,    // every baseline grows by this factor per level (RULES 5.2, decided 2026-09-24)
  healthBase: 50,       // health at level 1; grows by levelGrowth
  mobDamage: 0.45,      // a mob's natural attack as a fraction of a standard weapon (RULES 8)
  dodgeBase: 0.03,      // intrinsic dodge before stats and effects (4.3)
  blockBase: 0.0,       // nothing blocks without a shield or a feat
  K: 20,                // reduction = D / (D + K) at level 1; K is a unit, not a cap
  kGrowth: 1.0,         // K grows with the attacker's level as levelGrowth^((L-1)*kGrowth); at 1.0 a tier of armor reduces the same share at every level
  regenRounds: 45,      // rounds from empty to full out of a fight, standing (90 s)
  restRegen: 2,         // resting regenerates twice as fast (45 s)
  sleepRegen: 3,        // sleeping three times as fast (30 s)
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
// growth is the geometric level factor: 1 at level 1, levelGrowth^(L-1)
// after. A gap of N levels is the same ratio at every level, which is
// what lets one win-rate row hold everywhere (RULES 4.5).
function growth(L) { return Math.pow(P.levelGrowth, Math.max(0, (L || 1) - 1)); }

// Every weapon baseline deals about the standard's damage per round
// (damage * speed near 6 at level 1); they differ in rhythm, spread, and
// the trait in BASELINE_TRAITS. hands is the default when an item states
// none.
var WEAPON_BASELINES = {
  standard: function (L) { return { damage: 6 * growth(L), speed: 1.0, spread: 0.2, verb: "hit" }; },
  dagger:   function (L) { return { damage: 3 * growth(L), speed: 2.0, spread: 0.1, verb: "stab" }; },
  heavy:    function (L) { return { damage: 12 * growth(L), speed: 0.5, spread: 0.4, verb: "smash" }; },
  unarmed:  function (L) { return { damage: 1.5 * growth(L), speed: 1.0, spread: 0.2, verb: "punch" }; },
  axe:      function (L) { return { damage: 7.2 * growth(L), speed: 0.8, spread: 0.35, verb: "hack" }; },
  mace:     function (L) { return { damage: 6.6 * growth(L), speed: 0.9, spread: 0.15, verb: "crush" }; },
  spear:    function (L) { return { damage: 5.4 * growth(L), speed: 1.1, spread: 0.15, verb: "thrust" }; },
  flail:    function (L) { return { damage: 8.4 * growth(L), speed: 0.7, spread: 0.5, verb: "flail" }; },
  whip:     function (L) { return { damage: 2.8 * growth(L), speed: 2.1, spread: 0.2, verb: "lash" }; },
  staff:    function (L) { return { damage: 5.4 * growth(L), speed: 1.0, spread: 0.1, verb: "strike", hands: 2 }; }
};

// Armor baselines are a whole set's D; a piece takes its slot's share
// (SLOT_WEIGHT). Shields carry no slot weight, so the shield baselines
// state a piece's own number.
var ARMOR_BASELINES = {
  medium: function (L) { return { defense: 3 * growth(L), spread: 0.2 }; },
  light:  function (L) { return { defense: 0.7 * 3 * growth(L), spread: 0.1 }; },
  heavy:  function (L) { return { defense: 1.3 * 3 * growth(L), spread: 0.3 }; },
  cloth:  function (L) { return { defense: 0.4 * 3 * growth(L), spread: 0.05 }; },
  plate:  function (L) { return { defense: 1.6 * 3 * growth(L), spread: 0.35 }; },
  shield: function (L) { return { defense: 0.08 * 3 * growth(L), spread: 0.2, piece: true }; },
  tower:  function (L) { return { defense: 0.15 * 3 * growth(L), spread: 0.2, piece: true }; }
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
      kind:   w.kind   || "",
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
  base.damage = base.damage * P.mobDamage;
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
// Kinds in play so far: stat, crit, dodge, block, attacks, defense,
// protect, speedMult, dot, the power kinds below, and the combat kinds of
// the skills and feats: blind, disarmed, damage, powerAttack, critMult,
// shieldBlock, finesse, blindFighting, steady, opportunist, cooldownCut,
// unarmedMult, envenom.
// The engine itself reads four kinds for visibility (world/visibility.go):
// invisible and hidden on a character or its gear make it unseen, and
// detectInvisible and detectHidden on the viewer see through them.
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

// hasEffect reports whether an item or character view carries kind.
function hasEffect(v, kind) {
  if (!v || !v.effects) return false;
  for (var i = 0; i < v.effects.length; i++) if (v.effects[i].kind === kind) return true;
  return false;
}

// ---------------------------------------------------------------------
// Power kinds (the outfit set, cmdOutfit). Their sizes are in units: one
// unit is a standard weapon's swing at the owner's level, so a kind means
// the same at level 1 and level 50 and nothing runs away with the curve.
// Stacking, per kind (5.4):
//   attuned    the item's weapon and armor numbers follow its wearer's level
//   proc       each rolls on its own: {chance, power, verb}, on every hit
//   ignite     a burn per hit, stacking up to maxStacks: {power, rounds, maxStacks}
//   chill      slows the target; never stacks: {chance, mult, rounds}
//   leech      adds, capped at LIMITS.leech: {fraction} of damage dealt healed
//   thorns     adds, capped: {fraction} of a landed hit returned, {verb}
//   soak       adds: {power} units off every hit taken, never below LIMITS.soakFloor
//   lastStand  multiplies: take {mult} damage while below {threshold} health
//   execute    takes the larger: deal {mult} damage to foes below {threshold}
//   regen      adds, capped: {fraction} of maximum health back every round
//   aura       each strikes on its own: {power, verb} to every foe, every round
// ---------------------------------------------------------------------
var LIMITS = { leech: 0.5, thorns: 1.0, soakFloor: 0.4, regen: 0.10 };

function unit(c) { return WEAPON_BASELINES.standard(c.level || 1).damage; }

// attunedScale is how much an attuned item's numbers grow on c: the
// baseline ratio between c's level and the item's own.
function attunedScale(it, c) {
  if (!hasEffect(it, "attuned")) return 1;
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
  eachEffect(att, function (e) {
    var p = e.params;
    if (e.kind === "proc" && random.float() < (p.chance === undefined ? 1 : num(p.chance))) {
      var d = Math.round(spreadRoll(unit(att) * num(p.power), 0.2) * protectMult(def) * lastStandMult(def));
      if (d > 0) { procs.push({ verb: p.verb || "magic", damage: d }); dealt += d; }
    }
  });
  var stacks = burningStacks(def);
  eachEffect(att, function (e) {
    var p = e.params;
    if (e.kind === "ignite" && stacks < (num(p.maxStacks) || 1)) {
      effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(unit(att) * num(p.power))), source: "ignite" }, rounds: num(p.rounds) || 3 });
      stacks++;
    }
  });
  // Envenom: poison on the blade, its own stack beside the burns.
  var venom = burningStacks(def, "venom");
  eachEffect(att, function (e) {
    var p = e.params;
    if (e.kind === "envenom" && venom < (num(p.maxStacks) || 1)) {
      effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(unit(att) * num(p.power))), source: "venom" }, rounds: num(p.rounds) || 3 });
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
    var back = Math.round(dmg * thorns);
    if (back > 0) procs.push({ verb: te.params.verb || "thorns", damage: back, back: true });
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
  if (c.mob && c.mob.attack) return c.mob.attack;
  return WEAPON_BASELINES.unarmed(c.level || 1);
}

function resolveAttack(att, def, weapon, round) {
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
  // wielder's level.
  var base = w.damage * (weapon ? attunedScale(weapon, att) : 1);
  var dmg = spreadRoll(base, w.spread) * strikeMult(att, weapon) * levelMult(att);
  dmg *= swingMult(att, def, weapon);

  // Modify: crits exist only as effects (4.2) and the axe's edge; execute
  // finishes the wounded.
  var crit = false;
  var ce = bestEffect(att, "crit", "chance");
  var critChance = (ce ? num(ce.params.chance) : 0) + num(traitsOf(weapon).crit);
  if (critChance > 0 && random.float() < critChance) {
    dmg *= ((ce && num(ce.params.mult)) || 2) + sumEffects(att, "critMult", "amount");
    crit = true;
  }
  dmg *= executeMult(att, def);

  // Reduce: D / (D + K), with D from armor or natural hide, Constitution,
  // and level, less what a mace ignores; then soak, a last stand, and
  // protection effects.
  var D = defense(def) * (1 - Math.min(1, num(traitsOf(weapon).pierce))), K = kFor(att);
  dmg = dmg * K / (D + K);
  dmg = Math.max(dmg * LIMITS.soakFloor, dmg - soakAmount(def));
  dmg = dmg * lastStandMult(def) * protectMult(def);

  var out = { hit: true, damage: Math.max(0, Math.round(dmg)), crit: crit, verb: verb, stage: "" };
  return out.damage > 0 ? onHit(att, def, out.damage, out) : out;
}

// defense is the defender's D for this hit: the sum of worn armor's draws,
// or the mob's natural armor if nothing is worn, plus flat "defense"
// effects (wards), times Constitution and level.
function defense(c) {
  var D = 0, worn = false, s;
  if (c.equipment) for (s in c.equipment) {
    var it = c.equipment[s];
    if (it && it.armor && it.armor.defense) { D += spreadRoll(it.armor.defense * attunedScale(it, c), it.armor.spread); worn = true; }
  }
  if (!worn && c.mob && c.mob.armor) D = spreadRoll(c.mob.armor.defense, c.mob.armor.spread);
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

// dodgeChance is c's chance to dodge a swing or a skill.
function dodgeChance(c) {
  return P.dodgeBase * mult(c, "dexterity", true) + sumEffects(c, "dodge", "amount") + sumEffects(c, "powerAttack", "dodge") +
    armorTrait(c, "dodge") + weaponTrait(c, "dodge") + 0.10 * skillRating(c, "dodge") / 100;
}

// blockChance is c's chance to block, Riposte aside: block effects that do
// not come from the shield, then the shield's own (shieldBlock), unless
// the blow goes over it. Strength scales it.
function blockChance(c, overShield) {
  var own = 0, sh = shield(c);
  if (sh && sh.effects) for (var i = 0; i < sh.effects.length; i++) if (sh.effects[i].kind === "block") own += num(sh.effects[i].params.chance);
  var b = sumEffects(c, "block", "chance") - own + (overShield ? 0 : shieldBlock(c));
  return P.blockBase + b * mult(c, "strength", true);
}

// strikeMult is the stat multiplier on c's damage: Strength, or Dexterity
// with Weapon Finesse and a dagger in hand.
function strikeMult(c, weapon) {
  var finesse = false;
  eachEffect(c, function (e) { if (e.kind === "finesse") finesse = true; });
  return mult(c, finesse && isDagger(weapon) ? "dexterity" : "strength", true);
}

// damageMult is the sum of c's damage bonuses as one multiplier: "damage"
// effects (a weapon-only one needs a weapon) and Power Attack.
function damageMult(c, weapon) {
  var add = 0;
  eachEffect(c, function (e) {
    if (e.kind === "damage" && (!e.params.weapon || weapon)) add += (num(e.params.mult) || 1) - 1;
    if (e.kind === "powerAttack") add += (num(e.params.mult) || 1) - 1;
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
  var speed = ((w.speed || 1) * mult(c, "dexterity") + sumEffects(c, "attacks", "amount") + passiveSwings(c)) * speedMult(c);
  var stats = {};
  for (var i = 0; i < STATS.length; i++) stats[STATS[i]] = stat(c, STATS[i]);
  return {
    healthMax: Math.max(1, healthMax),
    manaMax: P.manaEnabled ? 10 + 2 * c.level : 0,
    speed: Math.max(0.1, speed),
    stats: stats
  };
}

function onTick(c) {
  // Damage over time ("dot" effects: Acid Splash, Blight, ignite) ticks
  // whether or not the character is fighting; so does regen.
  var dot = 0;
  eachEffect(c, function (e) { if (e.kind === "dot") dot += num(e.params.damage); });
  var regen = Math.round(c.healthMax * capped(c, "regen", "fraction", LIMITS.regen));
  if (c.fighting) {
    var aura = [];
    eachEffect(c, function (e) {
      if (e.kind === "aura") aura.push({ verb: e.params.verb || "aura", damage: Math.max(1, Math.round(spreadRoll(unit(c) * num(e.params.power), 0.2))) });
    });
    return { healthDelta: regen - Math.round(dot), manaDelta: 0, skills: improvePassives(c), aura: aura };
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
  return { healthDelta: rest + regen - Math.round(dot), manaDelta: 0, skills: skills };
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
    effect: { kind: "attacks", params: { amount: 1 } } },

  // --- approved 2026-09-28 ---
  { id: "iron_will", name: "Iron Will", level: 1, description: "+2 Wisdom.",
    effect: { kind: "stat", params: { stat: "wisdom", amount: 2 } } },
  { id: "keen_mind", name: "Keen Mind", level: 1, description: "+2 Intelligence.",
    effect: { kind: "stat", params: { stat: "intelligence", amount: 2 } } },
  { id: "thick_hide", name: "Thick Hide", level: 2, description: "Every hit on you is a tenth of a swing smaller.",
    effect: { kind: "soak", params: { power: 0.10 } } },
  { id: "shield_focus", name: "Shield Focus", level: 3, description: "Block 8 percent more with a shield worn.",
    effect: { kind: "shieldBlock", params: { chance: 0.08 } } },
  { id: "finesse", name: "Weapon Finesse", level: 3, description: "With a dagger, Dexterity drives your damage instead of Strength.",
    effect: { kind: "finesse", params: {} } },
  { id: "blind_fighting", name: "Blind-Fighting", level: 4, description: "Blinded, you miss half as often.",
    effect: { kind: "blindFighting", params: { mult: 0.5 } } },
  { id: "sure_footing", name: "Sure Footing", level: 4, description: "An even chance to keep your feet when tripped or bashed.",
    effect: { kind: "steady", params: { chance: 0.5 } } },
  { id: "power_attack", name: "Power Attack", level: 5, description: "15 percent more damage, 5 percent less dodge.",
    effect: { kind: "powerAttack", params: { mult: 1.15, dodge: -0.05 } } },
  { id: "die_hard", name: "Die Hard", level: 6, requires: ["toughness"], description: "Below a quarter of your health, take a quarter less damage.",
    effect: { kind: "lastStand", params: { threshold: 0.25, mult: 0.75 } } },
  { id: "opportunist", name: "Opportunist", level: 6, description: "25 percent more damage against a foe who is stunned, tripped, or slowed.",
    effect: { kind: "opportunist", params: { mult: 1.25 } } },
  { id: "bloodthirst", name: "Bloodthirst", level: 7, description: "Heal 5 percent of the damage you deal.",
    effect: { kind: "leech", params: { fraction: 0.05 } } },
  { id: "retaliation", name: "Retaliation", level: 8, requires: ["parry"], description: "A tenth of every hit on you goes back to the attacker.",
    effect: { kind: "thorns", params: { fraction: 0.10, verb: "riposte" } } },
  { id: "precise", name: "Precise Strikes", level: 8, requires: ["keen"], description: "Critical hits deal two and a half times damage instead of double.",
    effect: { kind: "critMult", params: { amount: 0.5 } } },
  { id: "weapon_master", name: "Weapon Master", level: 9, description: "10 percent more damage with a weapon in hand.",
    effect: { kind: "damage", params: { mult: 1.10, weapon: true } } },
  { id: "iron_fist", name: "Iron Fist", level: 9, description: "Bare hands and kicks hit 25 percent harder.",
    effect: { kind: "unarmedMult", params: { mult: 1.25 } } },
  { id: "executioner", name: "Executioner", level: 10, description: "25 percent more damage to foes below a fifth of their health.",
    effect: { kind: "execute", params: { threshold: 0.20, mult: 1.25 } } },
  { id: "stalwart", name: "Stalwart", level: 11, requires: ["thick_hide"], description: "Every hit on you is a quarter of a swing smaller, with Thick Hide.",
    effect: { kind: "soak", params: { power: 0.15 } } },
  { id: "lightning_reflexes", name: "Lightning Reflexes", level: 11, requires: ["evasion"], description: "Dodge 5 percent more often.",
    effect: { kind: "dodge", params: { amount: 0.05 } } },
  { id: "regeneration", name: "Regeneration", level: 12, requires: ["toughness"], description: "1 percent of your health back every round, even in a fight.",
    effect: { kind: "regen", params: { fraction: 0.01 } } },
  { id: "bulwark", name: "Bulwark", level: 13, requires: ["shield_focus"], description: "Block 7 percent more again with a shield worn.",
    effect: { kind: "shieldBlock", params: { chance: 0.07 } } },
  { id: "relentless", name: "Relentless", level: 14, description: "Every skill's cooldown is a round shorter, never below one.",
    effect: { kind: "cooldownCut", params: { rounds: 1 } } },
  { id: "unstoppable", name: "Unstoppable", level: 16, requires: ["sure_footing"], description: "Nothing takes your round from you, and slows are halved.",
    effect: { kind: "steady", params: { chance: 1, slow: 0.5 } } },
  { id: "deathblow", name: "Deathblow", level: 18, requires: ["deadly"], description: "A 15 percent chance to critically hit for double damage.",
    effect: { kind: "crit", params: { chance: 0.15, mult: 2 } } },
  { id: "fourth_attack", name: "Fourth Attack", level: 20, requires: ["third_attack"], description: "Yet another extra swing every round.",
    effect: { kind: "attacks", params: { amount: 1 } } },
  { id: "juggernaut", name: "Juggernaut", level: 22, requires: ["iron_skin"], description: "Take 10 percent less damage from everything.",
    effect: { kind: "protect", params: { mult: 0.9 } } }
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
// Magic (RULES 6). spellList defines the spells; the engine owns cast
// time, targets, materials, and cooldowns; resolveCast decides what each
// target suffers or gains. Damage is a multiple of the standard weapon's
// per-swing damage at the caster's level (6.5), times the branch stat and
// level (6.4).
// ---------------------------------------------------------------------
var SPELLS = [
  { id: "firebolt", name: "Firebolt", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    materials: [{ material: "ash", count: 1 }], target: "single", save: "reflex", saveEffect: "half", damage: 2.0,
    description: "A bolt of fire at one target." },
  { id: "fireball", name: "Fireball", branch: "arcane", school: "evocation", castRounds: 2, interruptOnDamage: true, cooldown: 8,
    materials: [{ material: "ash", count: 2 }, { material: "star iron", count: 1 }], target: "area", save: "reflex", saveEffect: "half", damage: 1.5,
    description: "Fire fills the room." },
  { id: "ward", name: "Ward", branch: "arcane", school: "abjuration", castRounds: 0, interruptOnDamage: false, cooldown: 10,
    materials: [{ material: "salt", count: 1 }], target: "ally", save: "none",
    effect: function (caster, target) { return { kind: "defense", params: { amount: target.level + 2 }, rounds: 10 }; },
    description: "A ward as strong as armor, for ten rounds." },
  { id: "acid", name: "Acid Splash", branch: "arcane", school: "conjuration", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    materials: [{ material: "salt", count: 1 }, { material: "ash", count: 1 }], target: "single", save: "fortitude", saveEffect: "half", damage: 1.5,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.3 * power) }, rounds: 3 }; },
    description: "Acid that keeps burning." },
  { id: "foresight", name: "Foresight", branch: "arcane", school: "divination", castRounds: 0, interruptOnDamage: false, cooldown: 20,
    materials: [{ material: "nightshade", count: 1 }], target: "self", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.10 }, rounds: 10 }; },
    description: "See the blow before it lands." },
  { id: "daze", name: "Daze", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 5,
    materials: [{ material: "bone dust", count: 1 }], target: "single", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.5 }, rounds: 3 }; },
    description: "The target's swings come slow." },
  { id: "blur", name: "Blur", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 15,
    materials: [{ material: "quicksilver", count: 1 }], target: "self", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.15 }, rounds: 5 }; },
    description: "Your outline swims." },
  { id: "drain", name: "Drain", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 3,
    materials: [{ material: "bone dust", count: 1 }], target: "single", save: "fortitude", saveEffect: "half", damage: 1.5, drain: 0.5,
    description: "Take their life for your own." },
  { id: "haste", name: "Haste", branch: "arcane", school: "transmutation", castRounds: 2, interruptOnDamage: true, cooldown: 20,
    materials: [{ material: "quicksilver", count: 2 }], target: "self", save: "none",
    effect: function () { return { kind: "attacks", params: { amount: 1 }, rounds: 5 }; },
    description: "One more swing every round." },
  { id: "mend", name: "Mend", branch: "divine", deity: "good", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    materials: [{ material: "tallow", count: 1 }], target: "ally", save: "none", heal: 0.30,
    description: "Close wounds." },
  { id: "sanctuary", name: "Sanctuary", branch: "divine", deity: "good", castRounds: 2, interruptOnDamage: true, cooldown: 30,
    materials: [{ material: "salt", count: 1 }, { material: "heartwood", count: 1 }], target: "group", save: "none",
    effect: function () { return { kind: "protect", params: { mult: 0.5 }, rounds: 5 }; },
    description: "The group takes half damage." },
  { id: "stillness", name: "Stillness", branch: "divine", deity: "neutral", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    materials: [{ material: "salt", count: 1 }, { material: "tallow", count: 1 }], target: "area", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.5 }, rounds: 2 }; },
    description: "Every enemy slows." },
  { id: "blight", name: "Blight", branch: "divine", deity: "evil", castRounds: 1, interruptOnDamage: true, cooldown: 6,
    materials: [{ material: "bone dust", count: 1 }, { material: "nightshade", count: 1 }], target: "area", save: "fortitude", saveEffect: "half",
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.4 * power) }, rounds: 5 }; },
    description: "Rot spreads through every enemy." }
];

function spellList() {
  // The engine only needs the definition fields; functions stay here.
  return SPELLS.map(function (sp) {
    return { id: sp.id, name: sp.name, branch: sp.branch, school: sp.school || "", deity: sp.deity || "",
      castRounds: sp.castRounds, interruptOnDamage: !!sp.interruptOnDamage, cooldown: sp.cooldown || 0,
      materials: sp.materials || [], target: sp.target, save: sp.save || "none", saveEffect: sp.saveEffect || "",
      description: sp.description || "" };
  });
}

function spellById(id) { for (var i = 0; i < SPELLS.length; i++) if (SPELLS[i].id === id) return SPELLS[i]; return null; }

var SAVE_STAT = { reflex: "dexterity", fortitude: "constitution", will: "wisdom" };

// spellPower is the caster's number for one "unit" of spell: the standard
// weapon's damage at the caster's level, times the branch stat and level.
function spellPower(caster, sp) {
  var statName = sp.branch === "divine" ? "wisdom" : "intelligence";
  return WEAPON_BASELINES.standard(caster.level || 1).damage * mult(caster, statName, true) * levelMult(caster) * (1 + weaponTrait(caster, "spell"));
}

// saves rolls the target's saving throw (6.4): S / (S + 2C).
function saves(caster, target, sp) {
  if (!sp.save || sp.save === "none") return false;
  var S = mult(target, SAVE_STAT[sp.save], true) * levelMult(target);
  var C = mult(caster, sp.branch === "divine" ? "wisdom" : "intelligence", true) * levelMult(caster);
  return random.float() < S / (S + 2 * C);
}

function resolveCast(caster, targets, spell) {
  var sp = spellById(spell.id);
  if (!sp) return { ok: false, message: "Nobody remembers how that spell goes." };
  var out = { ok: true, message: "", consume: [], targets: [], casterEffects: [] };
  var totalDrain = 0;
  for (var i = 0; i < targets.length; i++) {
    var t = targets[i];
    var tr = { index: i, damage: 0, heal: 0, saved: false, negated: false, effects: [], message: "" };
    var power = spellPower(caster, sp);
    var saved = saves(caster, t, sp);
    if (saved && sp.saveEffect === "negate") { tr.saved = true; tr.negated = true; out.targets.push(tr); continue; }
    var scale = saved ? 0.5 : 1;
    tr.saved = saved;
    if (sp.damage) {
      var K = kFor(caster);
      var dmg = power * sp.damage * scale * K / (defense(t) + K) * protectMult(t);
      tr.damage = Math.max(1, Math.round(dmg));
      if (sp.drain) totalDrain += tr.damage * sp.drain;
    }
    if (sp.heal) tr.heal = Math.max(1, Math.round(t.healthMax * sp.heal * mult(caster, "wisdom") ));
    if (sp.effect) {
      var e = sp.effect(caster, t, power * scale);
      if (e) tr.effects.push(e);
    }
    out.targets.push(tr);
  }
  // Drain heals the caster once every target is resolved.
  if (totalDrain > 0) out.heal = Math.max(1, Math.round(totalDrain));
  return out;
}

// ---------------------------------------------------------------------
// consider: a verdict on a fight, by level gap, worded from the measured
// win rates in RULES 4.5 (+1 a sure win, +3 favourable, +5 hopeless with
// geometric baselines at 1.10).
// ---------------------------------------------------------------------
function consider(me, target) {
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
// describeEffect: what an effect means, in words, for look and score.
// ---------------------------------------------------------------------
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
    case "attacks": return (num(p.amount) >= 0 ? "+" : "") + num(p.amount) + " swing" + (Math.abs(num(p.amount)) === 1 ? "" : "s") + " per round" + tail;
    case "defense": return "+" + Math.round(num(p.amount)) + " defense" + tail;
    case "protect": return num(p.mult) > 1
      ? "Takes " + Math.round((num(p.mult) - 1) * 100) + " percent more damage" + tail
      : "Takes " + Math.round((1 - num(p.mult)) * 100) + " percent less damage" + tail;
    case "speedMult": return "Swings at " + Math.round(num(p.mult) * 100) + " percent speed" + tail;
    case "dot": return (p.source === "ignite" ? "Burning: takes " : "Takes ") + num(p.damage) + " damage a round" + tail;
    case "attuned": return "Attuned: its numbers grow to match its wearer's level";
    case "proc": return "On hit (" + (p.verb || "magic") + "): " + (p.chance === undefined || num(p.chance) >= 1 ? "always" : "a " + pct(p.chance) + " chance") + ", " + units(p.power) + " past armor" + tail;
    case "ignite": return "On hit (burning): " + units(p.power) + " a round for " + (num(p.rounds) || 3) + " rounds, stacking " + (num(p.maxStacks) || 1) + " deep" + tail;
    case "chill": return "A " + pct(p.chance) + " chance on each hit to slow the target to " + Math.round(num(p.mult) * 100) + " percent speed for " + (num(p.rounds) || 2) + " rounds" + tail;
    case "leech": return "Heals you for " + pct(p.fraction) + " of the damage you deal" + tail;
    case "thorns": return "Thorns (" + (p.verb || "thorns") + "): " + pct(p.fraction) + " of every hit that lands on you goes back to the attacker" + tail;
    case "soak": return "Every hit on you is " + units(p.power) + " smaller" + tail;
    case "lastStand": return "Below " + pct(p.threshold) + " health, take " + Math.round((1 - num(p.mult)) * 100) + " percent less damage" + tail;
    case "execute": return "Deal " + Math.round((num(p.mult) - 1) * 100) + " percent more damage to foes below " + pct(p.threshold) + " health" + tail;
    case "regen": return "Heals " + pct(p.fraction) + " of your health every round, even in a fight" + tail;
    case "aura": return "Aura (" + (p.verb || "aura") + "): " + units(p.power) + " to every foe fighting you, each round" + tail;
    case "blind": return "Blinded: " + pct(p.miss) + " of swings go wide" + tail;
    case "disarmed": return "Disarmed: fights bare-handed" + tail;
    case "damage": return Math.round((num(p.mult) - 1) * 100) + " percent more damage" + (p.weapon ? " with a weapon" : "") + tail;
    case "powerAttack": return Math.round((num(p.mult) - 1) * 100) + " percent more damage, " + pct(-num(p.dodge)) + " less dodge" + tail;
    case "critMult": return "Critical hits deal +" + num(p.amount) + "x damage" + tail;
    case "shieldBlock": return "Blocks " + pct(p.chance) + " more with a shield" + tail;
    case "finesse": return "Dexterity drives dagger damage" + tail;
    case "blindFighting": return "Blinded, misses " + Math.round((1 - num(p.mult)) * 100) + " percent less often" + tail;
    case "steady": return (num(p.chance) >= 1 ? "Never loses a round" : "A " + pct(p.chance) + " chance to keep a round that would be lost") +
      (p.slow !== undefined ? "; slows are " + Math.round((1 - num(p.slow)) * 100) + " percent weaker" : "") + tail;
    case "opportunist": return Math.round((num(p.mult) - 1) * 100) + " percent more damage to foes who are stunned or slowed" + tail;
    case "cooldownCut": return "Skill cooldowns " + num(p.rounds) + " round" + (num(p.rounds) === 1 ? "" : "s") + " shorter" + tail;
    case "unarmedMult": return "Bare hands and kicks hit " + Math.round((num(p.mult) - 1) * 100) + " percent harder" + tail;
    case "envenom": return "Envenomed: each hit poisons for " + units(p.power) + " a round, " + (num(p.maxStacks) || 1) + " doses deep" + tail;
    case "detectInvisible": return "Reveals the invisible" + tail;
    case "detectHidden": return "Reveals the hidden" + tail;
    case "invisible": return "Makes the wearer invisible" + tail;
    case "hidden": return "Hides the wearer" + tail;
    case "skill": return "";
    default: return "";
  }
}

// ---------------------------------------------------------------------
// Skills (RULES 7.4): the one action per round beyond the auto-attack,
// and passives that improve with use. A rating from start to 100 is how
// much of the skill's potential a use delivers; it is never a chance of
// failure. Every character has every skill its level allows.
// ---------------------------------------------------------------------
// Fields beyond the engine's: damage (units of a standard swing, or a
// function of user and target), verb, stunChance (a lost round, scaled by
// rating), bleed, unavoidable (no dodge or block), unarmed (Iron Fist
// applies), check (a reason the use cannot go ahead, or ""), effects (what
// a landed use leaves, scaled by rating), self (a use on the user alone),
// ally (a use on a friend), rest (a passive that improves resting, not
// fighting).
var SKILLS = [
  { id: "kick", name: "Kick", level: 1, passive: false, target: "single", cooldown: 2, start: 30, innate: true,
    damage: 1.2, verb: "kick", unarmed: true,
    description: "A kick worth more than a swing at full skill, and it needs no weapon." },
  { id: "bash", name: "Bash", level: 3, passive: false, target: "single", cooldown: 4, start: 25, innate: false, price: 300,
    damage: 0.6, verb: "bash", stunChance: 1.0,
    description: "Slam into them: some damage, and at full skill they lose their next round of swings." },
  { id: "twin", name: "Twin Strike", level: 5, passive: true, start: 20, innate: false, price: 1500,
    description: "A second swing in the same round, as often as your skill allows. Triple Strike follows." },
  { id: "rend", name: "Rend", level: 3, passive: false, target: "single", cooldown: 3, start: 30, innate: false, price: 100,
    requires: [{ material: "wolf fang", count: 2 }], damage: 0.8, verb: "rend", bleed: 0.25,
    description: "Tear at them like a beast: some damage now, and a wound that bleeds for three rounds. The beast-master wants wolf fangs for it." },
  { id: "riposte", name: "Riposte", level: 4, passive: true, start: 20, innate: false, price: 500,
    description: "Turn a swing aside with your own blade: at full skill, one swing in five is blocked, weapon in hand." },

  // --- ROM 2.4 and friends (approved 2026-09-28) ---
  { id: "dodge", name: "Dodge", level: 1, passive: true, start: 20, innate: false, price: 200,
    description: "Slip aside: up to one more swing in ten dodged." },
  { id: "trip", name: "Trip", level: 2, passive: false, target: "single", cooldown: 4, start: 25, innate: false, price: 200,
    damage: 0.3, verb: "trip", stunChance: 1.0, stunMessage: "They go down.",
    effects: function (u, t, scale) { return [{ on: "target", kind: "dodge", params: { amount: -0.05 }, rounds: 2 }]; },
    description: "Sweep their legs: a little damage, they are slow to dodge for two rounds, and at full skill they lose their next round." },
  { id: "hand_to_hand", name: "Hand to Hand", level: 2, passive: true, start: 20, innate: false, price: 300,
    description: "Fight with your fists: at full skill, bare hands hit twice as hard." },
  { id: "dirt_kicking", name: "Dirt Kicking", level: 3, passive: false, target: "single", cooldown: 5, start: 25, innate: false, price: 300,
    verb: "dirt kick",
    effects: function (u, t, scale, out) {
      out.message = "They are blinded!";
      return [{ on: "target", kind: "blind", params: { miss: 0.4 * scale }, rounds: 2 }];
    },
    description: "Dirt in the eyes: for two rounds, up to four of their swings in ten go wide." },
  { id: "shield_block", name: "Shield Block", level: 3, passive: true, start: 20, innate: false, price: 400,
    description: "Take blows on your shield: up to 15 percent more blocked, shield worn." },
  { id: "backstab", name: "Backstab", level: 4, passive: false, target: "single", cooldown: 0, start: 25, innate: false, price: 500,
    damage: 3.0, verb: "backstab",
    check: function (u, t) {
      if (!isDagger(wielded(u))) return "You need a dagger in hand to backstab.";
      if (u.fighting) return "You are too busy fighting to slip a blade in. Try circle.";
      if (t.fighting) return "They are fighting already, and watching for it.";
      return "";
    },
    description: "Open a fight with a dagger in the back: three swings' worth at full skill. Only on someone not yet fighting." },
  { id: "fast_healing", name: "Fast Healing", level: 4, passive: true, start: 20, innate: false, price: 400, rest: true,
    description: "Mend quickly: resting brings health back up to half again as fast. Improves while you rest." },
  { id: "enhanced_damage", name: "Enhanced Damage", level: 5, passive: true, start: 15, innate: false, price: 1000,
    description: "Put your weight behind it: up to 15 percent more damage on every swing." },
  { id: "hamstring", name: "Hamstring", level: 6, passive: false, target: "single", cooldown: 6, start: 25, innate: false, price: 600,
    damage: 0.5, verb: "hamstring",
    effects: function (u, t, scale) { return [{ on: "target", kind: "speedMult", params: { mult: 1 - 0.3 * scale, source: "hamstring" }, rounds: 4 }]; },
    description: "Cut the tendon: some damage, and their swings slow to 70 percent for four rounds at full skill." },
  { id: "disarm", name: "Disarm", level: 6, passive: false, target: "single", cooldown: 8, start: 20, innate: false, price: 700,
    verb: "disarm",
    check: function (u, t) {
      if (!wielded(u)) return "You need a weapon of your own to disarm with.";
      if (!wielded(t)) return "They have nothing to disarm.";
      return "";
    },
    effects: function (u, t, scale, out) {
      out.message = "Their weapon goes wide, and they fight bare-handed.";
      return [{ on: "target", kind: "disarmed", params: {}, rounds: 1 + Math.round(2 * scale) }];
    },
    description: "Knock their weapon aside: they fight with what nature gave them for up to three rounds." },
  { id: "feint", name: "Feint", level: 7, passive: false, target: "single", cooldown: 5, start: 25, innate: false, price: 700,
    verb: "feint", unavoidable: true,
    effects: function (u, t, scale, out) {
      out.message = "They bite on the feint.";
      return [{ on: "target", kind: "dodge", params: { amount: -0.10 * scale }, rounds: 3 },
              { on: "target", kind: "block", params: { chance: -0.10 * scale }, rounds: 3 }];
    },
    description: "Draw their guard: for three rounds they dodge and block up to one swing in ten less." },
  { id: "berserk", name: "Berserk", level: 8, passive: false, target: "none", cooldown: 20, start: 25, innate: false, price: 900,
    check: function (u) { return hasSource(u, "berserk") ? "You are already raging." : ""; },
    self: function (u, scale, out) {
      out.message = "You fly into a rage!";
      out.effects = [
        { on: "self", kind: "stat", params: { stat: "strength", amount: Math.max(1, Math.round(3 * scale)), source: "berserk" }, rounds: 6 },
        { on: "self", kind: "attacks", params: { amount: 0.5 * scale, source: "berserk" }, rounds: 6 },
        { on: "self", kind: "dodge", params: { amount: -0.05, source: "berserk" }, rounds: 6 }];
    },
    description: "Rage for six rounds: up to +3 Strength and half a swing more each round, but you dodge less." },
  { id: "envenom", name: "Envenom", level: 9, passive: false, target: "none", cooldown: 15, start: 25, innate: false, price: 900,
    check: function (u) { return wielded(u) ? "" : "You need a blade to coat."; },
    self: function (u, scale, out) {
      out.message = "You coat your weapon in venom.";
      out.effects = [{ on: "self", kind: "envenom", params: { power: 0.15 * scale, rounds: 3, maxStacks: 2 }, rounds: 10 }];
    },
    description: "Poison your weapon for ten rounds: each hit leaves venom in the wound, two doses deep." },
  { id: "rescue", name: "Rescue", level: 10, passive: false, target: "ally", cooldown: 6, start: 30, innate: false, price: 1000,
    verb: "rescue",
    ally: function (u, t, scale, out) {
      out.taunt = true;
      out.effects = [{ on: "target", kind: "protect", params: { mult: 1 - 0.3 * scale }, rounds: 2 }];
    },
    description: "Step in front of a friend: everyone fighting them turns on you, and they take less damage for two rounds." },
  { id: "circle", name: "Circle", level: 10, passive: false, target: "single", cooldown: 6, start: 20, innate: false, price: 1200,
    damage: 1.8, verb: "circle",
    check: function (u, t) {
      if (!isDagger(wielded(u))) return "You need a dagger in hand to circle.";
      if (!u.fighting) return "Circle is for a fight under way; backstab opens one.";
      return "";
    },
    description: "Slip around them mid-fight and stab: nearly two swings' worth at full skill. Dagger only." },
  { id: "sunder", name: "Sunder", level: 12, passive: false, target: "single", cooldown: 8, start: 20, innate: false, price: 1500,
    damage: 1.0, verb: "sunder",
    check: function (u) { return isHeavy(wielded(u)) ? "" : "You need a heavy weapon to sunder armor."; },
    effects: function (u, t, scale, out) {
      out.message = "Their armor buckles.";
      return [{ on: "target", kind: "defense", params: { amount: -0.5 * ARMOR_BASELINES.medium(t.level).defense * scale }, rounds: 5 }];
    },
    description: "Break their armor with a heavy weapon: a swing's damage, and for five rounds half a set of armor stops nothing." },
  { id: "second_wind", name: "Second Wind", level: 12, passive: false, target: "none", cooldown: 30, start: 25, innate: false, price: 1500,
    check: function (u) { return u.health >= u.healthMax ? "You are not winded." : ""; },
    self: function (u, scale, out) { out.heal = Math.max(1, Math.round(u.healthMax * (0.10 + 0.15 * scale))); },
    verb: "second wind",
    description: "Catch your breath, even mid-fight: up to a quarter of your health back at once." },
  { id: "whirlwind", name: "Whirlwind", level: 15, passive: false, target: "area", cooldown: 10, start: 20, innate: false, price: 2500,
    damage: 0.8, verb: "whirlwind",
    description: "Spin through them: most of a swing to every foe fighting you." },
  { id: "coup_de_grace", name: "Coup de Grace", level: 18, passive: false, target: "single", cooldown: 8, start: 20, innate: false, price: 3000,
    damage: function (u, t) { return t.healthMax > 0 && t.health / t.healthMax < 0.25 ? 3.5 : 0.5; }, verb: "coup de grace",
    description: "Finish them: three and a half swings' worth against a foe below a quarter of their health, little otherwise." },
  { id: "triple", name: "Triple Strike", level: 12, passive: true, start: 15, innate: false, price: 4000,
    description: "A third swing in the same round, as often as your skill allows, on top of Twin Strike." }
];

// hasSource reports whether c carries an effect tagged with source.
function hasSource(c, source) {
  var found = false;
  eachEffect(c, function (e) { if (e.params && e.params.source === source) found = true; });
  return found;
}

var SKILL_IMPROVE = { chance: 0.5, min: 1, max: 3 };  // per use, scaled by how far from 100

function skillList() {
  return SKILLS.map(function (sk) {
    return { id: sk.id, name: sk.name, level: sk.level, passive: !!sk.passive, target: sk.target || "none",
      cooldown: sk.cooldown || 0, start: sk.start, innate: !!sk.innate, price: sk.price || 0, requires: sk.requires || [], description: sk.description || "" };
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

// passiveSwings: Twin Strike grants rating/100 of an extra swing per
// round, and Triple Strike as much again, but only on top of Twin.
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
    if (!sk || !sk.passive || sk.rest) continue;
    var next = improve(num(e.state.effectiveness));
    if (next !== num(e.state.effectiveness)) out[sk.id] = next;
  }
  return out;
}

// useSkill resolves an active skill through the same pipeline stages as a
// swing: dodge, block, roll, reduce; the rating scales the result.
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
  // Relentless: cooldowns a round shorter, never below one.
  var cut = sumEffects(user, "cooldownCut", "rounds");
  if (cut > 0 && sk.cooldown > 0) out.cooldown = Math.max(1, sk.cooldown - cut);
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
    var dmg = spreadRoll(unit(user) * units * scale, 0.2) * strikeMult(user, weapon) * levelMult(user) *
      damageMult(user, weapon) * opportunistMult(user, target);
    if (sk.unarmed) eachEffect(user, function (fx) { if (fx.kind === "unarmedMult") dmg *= num(fx.params.mult) || 1; });
    var K = kFor(user);
    dmg = dmg * K / (defense(target) + K) * protectMult(target);
    out.damage = Math.max(1, Math.round(dmg));
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
    out.effects.push({ on: "target", kind: "dot", params: { damage: Math.max(1, Math.round(out.damage * sk.bleed)) }, rounds: 3 });
  }
  if (sk.effects) out.effects = out.effects.concat(sk.effects(user, target, scale, out) || []);
  return out;
}

// ---------------------------------------------------------------------
// Money (RULES 7.5): what a mob carries, in silver. 100 silver is a gold.
// A prototype's silver field overrides this.
// ---------------------------------------------------------------------
var MONEY = { perLevel: 5, spread: 0.5 };  // a level-4 mob carries about 20 silver, give or take half

function moneyFor(victim) {
  var base = MONEY.perLevel * (victim.level || 1);
  return Math.max(0, Math.round(spreadRoll(base, MONEY.spread)));
}

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
  silverPerLevel: 20,  // coin on top, in silver
  roundSeconds: 2      // timing.round_ms in config; keep in step
};

function questRules(c) {
  var perMinute = 60 / QUEST.roundSeconds;
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
