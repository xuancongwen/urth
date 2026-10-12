// ---------------------------------------------------------------------
// The divine domains (RULES 6.1, 6.5): what each of the three gods gives
// an apostle beyond the innate Mend, Sanctuary, Stillness, and Blight.
// Taught spells carry a price; the strongest of each god is found on a
// scroll (found: true). Damage is in units of a standard swing at the
// caster's level, as in rules.js; heal is a share of the target's maximum.
//
// Good is healing, protection, and holy light. Neutral is balance and
// nature: slowing, binding, bark and storm, and restoration. Evil is rot,
// fear, curses, and the wound that feeds its caster.
// ---------------------------------------------------------------------
SPELLS.push(
  // --- the Good god ---
  { id: "bless", level: 3, stats: { wisdom: 12 }, mana: 8, price: 240, name: "Bless", branch: "divine", deity: "good", castRounds: 1, interruptOnDamage: false, cooldown: 10,
    target: "ally", save: "none",
    effects: function (caster, target) {
      return [{ kind: "defense", params: { amount: Math.round((target.level || 1) / 2) + 2 }, rounds: 15 },
              { kind: "protect", params: { mult: 0.9 }, rounds: 15 }];
    },
    description: "A hand on the brow, and blows land softer." },
  { id: "smite", level: 12, stats: { wisdom: 16 }, mana: 12, price: 1080, name: "Smite", branch: "divine", deity: "good", castRounds: 1, interruptOnDamage: true, cooldown: 2,
    target: "single", save: "will", saveEffect: "half", damage: 2.2,
    description: "Light falls on one foe like a hammer." },
  { id: "prayer_of_healing", level: 28, stats: { wisdom: 19 }, mana: 28, price: 2800, name: "Prayer of Healing", branch: "divine", deity: "good", castRounds: 2, interruptOnDamage: true, cooldown: 12,
    target: "group", save: "none", heal: 0.25,
    effect: function () { return { kind: "regen", params: { fraction: 0.02 }, rounds: 6 }; },
    description: "The whole group is healed, and goes on healing." },
  { id: "holy_word", found: true, level: 42, stats: { wisdom: 22 }, mana: 38, name: "Holy Word", branch: "divine", deity: "good", castRounds: 2, interruptOnDamage: true, cooldown: 25,
    target: "area", save: "will", saveEffect: "half", damage: 2.5,
    effect: function () { return { kind: "blind", params: { miss: 0.3 }, rounds: 2 }; },
    description: "One word of the god, spoken aloud. Every enemy burns and is dazzled." },

  // --- the Neutral god ---
  { id: "entangle", level: 2, stats: { wisdom: 11 }, mana: 7, price: 160, name: "Entangle", branch: "divine", deity: "neutral", castRounds: 1, interruptOnDamage: true, cooldown: 4,
    target: "single", save: "reflex", saveEffect: "negate",
    effects: function () {
      return [{ kind: "speedMult", params: { mult: 0.6 }, rounds: 3 },
              { kind: "dodge", params: { amount: -0.05 }, rounds: 3 }];
    },
    description: "Roots and creepers take hold of one foe's legs." },
  { id: "barkskin", level: 7, stats: { wisdom: 14 }, mana: 10, price: 560, name: "Barkskin", branch: "divine", deity: "neutral", castRounds: 1, interruptOnDamage: false, cooldown: 15,
    target: "ally", save: "none",
    effects: function (caster, target) {
      return [{ kind: "soak", params: { power: 0.2 }, rounds: 12 },
              { kind: "defense", params: { amount: Math.round((target.level || 1) / 2) }, rounds: 12 }];
    },
    description: "Skin as rough and thick as oak bark." },
  { id: "call_lightning", level: 14, stats: { wisdom: 17 }, mana: 16, price: 1260, name: "Call Lightning", branch: "divine", deity: "neutral", castRounds: 2, interruptOnDamage: true, cooldown: 6,
    target: "single", save: "reflex", saveEffect: "half", damage: 2.8,
    description: "Call the sky down on one foe. It takes a moment to answer." },
  { id: "restoration", level: 26, stats: { wisdom: 19 }, mana: 22, price: 2600, name: "Restoration", branch: "divine", deity: "neutral", castRounds: 2, interruptOnDamage: true, cooldown: 10,
    target: "ally", save: "none", heal: 0.35,
    effect: function () { return { kind: "regen", params: { fraction: 0.03 }, rounds: 5 }; },
    description: "The body set back in balance, and kept there a while." },
  { id: "storm_of_vengeance", found: true, level: 44, stats: { wisdom: 22 }, mana: 40, name: "Storm of Vengeance", branch: "divine", deity: "neutral", castRounds: 3, interruptOnDamage: true, cooldown: 25,
    target: "area", save: "reflex", saveEffect: "half", damage: 2.2,
    effect: function () { return { kind: "speedMult", params: { mult: 0.6 }, rounds: 3 }; },
    description: "Hail, wind, and lightning over every enemy, and they bow under it." },

  // --- the Evil god ---
  { id: "rotting_touch", level: 1, stats: { wisdom: 11 }, mana: 6, price: 60, name: "Rotting Touch", branch: "divine", deity: "evil", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "single", save: "fortitude", saveEffect: "half", dot: true,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.5 * power) }, rounds: 4 }; },
    description: "One touch, and the flesh begins to spoil." },
  { id: "dread", level: 4, stats: { wisdom: 12 }, mana: 8, price: 320, name: "Dread", branch: "divine", deity: "evil", castRounds: 1, interruptOnDamage: true, cooldown: 6,
    target: "single", save: "will", saveEffect: "negate",
    effects: function () {
      return [{ kind: "speedMult", params: { mult: 0.6 }, rounds: 4 },
              { kind: "damage", params: { mult: 0.85 }, rounds: 4 }];
    },
    description: "A terror out of the pit. The target cowers, slow and half-hearted." },
  { id: "malediction", level: 12, stats: { wisdom: 16 }, mana: 12, price: 1080, name: "Malediction", branch: "divine", deity: "evil", castRounds: 1, interruptOnDamage: true, cooldown: 8,
    target: "single", save: "will", saveEffect: "negate",
    effects: function (caster) {
      var n = -(2 + Math.floor((caster.level || 1) / 10));
      return [{ kind: "stat", params: { stat: "strength", amount: n }, rounds: 8 },
              { kind: "stat", params: { stat: "dexterity", amount: n }, rounds: 8 },
              { kind: "stat", params: { stat: "wisdom", amount: n }, rounds: 8 }];
    },
    description: "A curse that eats strength, quickness, and will." },
  { id: "harm", level: 28, stats: { wisdom: 19 }, mana: 26, price: 2800, name: "Harm", branch: "divine", deity: "evil", castRounds: 2, interruptOnDamage: true, cooldown: 8,
    target: "single", save: "fortitude", saveEffect: "half", damage: 2.8, drain: 0.33,
    description: "Open every wound at once, and drink a little of what runs out." },
  { id: "unholy_word", found: true, level: 44, stats: { wisdom: 22 }, mana: 40, name: "Unholy Word", branch: "divine", deity: "evil", castRounds: 2, interruptOnDamage: true, cooldown: 25,
    target: "area", save: "will", saveEffect: "half", damage: 2.2,
    effects: function () {
      return [{ kind: "speedMult", params: { mult: 0.7 }, rounds: 3 },
              { kind: "damage", params: { mult: 0.8 }, rounds: 3 }];
    },
    description: "A word that should not be said. Every enemy is torn by it, and shrinks." }
);
