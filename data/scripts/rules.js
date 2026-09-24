// Urth rules. PLACEHOLDER NUMBERS. Every formula here is a stand-in so the
// engine has something to call; the real rules are designed in
// docs/RULES.md and will replace these one hook at a time.
//
// Edits to this file take effect on the next round without a restart.
// A syntax error keeps the previous version running and warns admins.
//
// Available globals:
//   random.int(n)        integer in [0, n)
//   random.float()       float in [0, 1)
//   random.roll(c, s)    sum of c dice with s sides
//   log(...)             write to the server log
//
// Character views: { name, level, xp, stats:{}, health, healthMax, mana,
//   manaMax, equipment:{slot: item}, isPlayer, fighting, room:{vnum,name,area},
//   vnum (mobs), flags (mobs) }
// Item views: { vnum, name, type, slot, weight, value, flags,
//   weapon:{damage,hands,kind}, armor:{defense}, mods:{} }

// resolveAttack: one swing. Return { hit, damage, crit, verb }.
// Placeholder: every swing lands for the weapon's listed damage, or 1
// unarmed. Stats do nothing yet.
function resolveAttack(attacker, defender, weapon, round) {
  var damage = weapon && weapon.weapon ? weapon.weapon.damage : 1;
  return { hit: true, damage: damage, crit: false, verb: "hit" };
}

// derivedStats: maxima and attack count from level, stats, and gear.
// Placeholder: flat pools that grow with level.
function derivedStats(c) {
  return {
    healthMax: 20 + c.level * 5,
    manaMax: 10 + c.level * 2,
    attacksPerRound: 1
  };
}

// onTick: per-round regeneration. Return { healthDelta, manaDelta }.
function onTick(c) {
  return { healthDelta: c.fighting ? 0 : 1, manaDelta: 1 };
}

// xpForKill: experience for a kill.
function xpForKill(killer, victim) {
  return Math.max(1, victim.level * 10 - (killer.level - victim.level) * 5);
}

// xpToLevel: total experience needed to reach a level.
function xpToLevel(level) {
  return (level - 1) * 100;
}

// onLevel: what a new level grants. Return { statDeltas:{}, message }.
function onLevel(c, newLevel) {
  return { statDeltas: {}, message: "You raise a level!" };
}
