// ---------------------------------------------------------------------
// Arcane spells beyond the first list in rules.js (RULES 6.1, 6.5). Six
// schools since 2026-10-12, divination and transmutation folded into
// the rest. Every spell has a tier: common spells are open to anyone with
// the level and Intelligence, taught for a price; school spells need the
// school, the first of each known at once (innate); rare ones are found
// only on a scroll (found). Damage is a multiple of the standard weapon's
// swing at the caster's level, as in rules.js: about 2.0x for a one-round
// single target, 1.5x for the room, 0.3x to 0.5x a round for damage over
// time; more needs a longer cast, a cooldown, and more mana.
// ---------------------------------------------------------------------
SPELLS.push(
  // --- Evocation: raw force, the school that hurts most ---
  { id: "magic_missile", tier: "common", kind: "magic", level: 2, stats: { intelligence: 11 }, mana: 6, price: 150, name: "Magic Missile", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: false, cooldown: 0,
    target: "single", save: "none", damage: 1.4,
    description: "Darts of force that never miss." },
  { id: "fire_shield", tier: "school", level: 12, stats: { intelligence: 16 }, mana: 12, price: 1200, name: "Fire Shield", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: false, cooldown: 20,
    target: "self", save: "none",
    effect: function () { return { kind: "thorns", params: { fraction: 0.30, verb: "flames" }, rounds: 10 }; },
    description: "Flames wrap you and bite whoever strikes you." },
  { id: "lightning_bolt", tier: "school", kind: "lightning", level: 14, stats: { intelligence: 17 }, mana: 14, price: 1400, name: "Lightning Bolt", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: true, cooldown: 4,
    target: "single", save: "reflex", saveEffect: "half", damage: 2.5,
    description: "A stroke of lightning from your hand." },
  { id: "cone_of_cold", tier: "school", kind: "cold", level: 20, stats: { intelligence: 18 }, mana: 20, price: 2000, name: "Cone of Cold", branch: "arcane", school: "evocation", castRounds: 2, interruptOnDamage: true, cooldown: 10,
    target: "area", save: "reflex", saveEffect: "half", damage: 1.8,
    effect: function () { return { kind: "speedMult", params: { mult: 0.75, source: "chill" }, rounds: 2 }; },
    description: "A blast of killing frost that leaves the room stiff." },
  { id: "chain_lightning", tier: "school", kind: "lightning", level: 30, stats: { intelligence: 20 }, mana: 28, price: 3600, name: "Chain Lightning", branch: "arcane", school: "evocation", castRounds: 2, interruptOnDamage: true, cooldown: 12,
    target: "area", save: "reflex", saveEffect: "half", damage: 2.2,
    description: "Lightning leaps from enemy to enemy." },
  { id: "meteor_swarm", tier: "rare", kind: "fire", found: true, level: 42, stats: { intelligence: 22 }, mana: 40, name: "Meteor Swarm", branch: "arcane", school: "evocation", castRounds: 3, interruptOnDamage: true, cooldown: 20,
    target: "area", save: "reflex", saveEffect: "half", damage: 3.5,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.4 * power) }, rounds: 3 }; },
    description: "Burning stones fall from the sky and the ground keeps burning." },

  // --- Abjuration: wards, soaks, and the walls between ---
  { id: "arcane_shield", tier: "common", level: 1, stats: { intelligence: 11 }, mana: 6, price: 100, name: "Shield", branch: "arcane", school: "abjuration", castRounds: 0, interruptOnDamage: false, cooldown: 10,
    target: "self", save: "none",
    effect: function () { return { kind: "block", params: { chance: 0.20 }, rounds: 5 }; },
    description: "An unseen disc turns blows aside." },
  { id: "stoneskin", tier: "school", innate: true, level: 12, stats: { intelligence: 16 }, mana: 14, price: 1200, name: "Stoneskin", branch: "arcane", school: "abjuration", castRounds: 1, interruptOnDamage: true, cooldown: 20,
    target: "ally", save: "none",
    effect: function () { return { kind: "soak", params: { power: 0.25 }, rounds: 10 }; },
    description: "Skin hard as granite blunts every hit." },
  { id: "repulsion", tier: "school", level: 18, stats: { intelligence: 18 }, mana: 16, price: 1800, name: "Repulsion", branch: "arcane", school: "abjuration", castRounds: 1, interruptOnDamage: true, cooldown: 12,
    target: "area", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.6 }, rounds: 3 }; },
    description: "A pressure holds every enemy back from you." },
  { id: "globe_of_invulnerability", tier: "school", level: 26, stats: { intelligence: 19 }, mana: 26, price: 3000, name: "Globe of Invulnerability", branch: "arcane", school: "abjuration", castRounds: 2, interruptOnDamage: true, cooldown: 30,
    target: "group", save: "none",
    effect: function () { return { kind: "protect", params: { mult: 0.6 }, rounds: 5 }; },
    description: "A shimmering globe closes over you and yours." },
  { id: "prismatic_ward", tier: "rare", found: true, level: 38, stats: { intelligence: 21 }, mana: 35, name: "Prismatic Ward", branch: "arcane", school: "abjuration", castRounds: 2, interruptOnDamage: true, cooldown: 40,
    target: "group", save: "none",
    effects: function () {
      return [{ kind: "protect", params: { mult: 0.5 }, rounds: 6 },
              { kind: "thorns", params: { fraction: 0.25, verb: "prismatic light" }, rounds: 6 }];
    },
    description: "Seven walls of colour, and each one bites back." },

  // --- Conjuration: acid, ice, and poison brought from elsewhere ---
  { id: "mage_armor", tier: "common", level: 5, stats: { intelligence: 13 }, mana: 8, price: 500, name: "Mage Armor", branch: "arcane", school: "conjuration", castRounds: 1, interruptOnDamage: false, cooldown: 0,
    target: "ally", save: "none",
    effect: function (caster, target) { return { kind: "defense", params: { amount: Math.round((target.level || 1) / 2) + 2 }, rounds: 40 }; },
    description: "A lasting skin of force, lighter than a ward." },
  { id: "acid_arrow", tier: "common", kind: "acid", level: 7, stats: { intelligence: 14 }, mana: 10, price: 700, name: "Acid Arrow", branch: "arcane", school: "conjuration", castRounds: 1, interruptOnDamage: true, cooldown: 4,
    target: "single", save: "reflex", saveEffect: "half", damage: 1.0,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.5 * power) }, rounds: 4 }; },
    description: "An arrow of acid that eats deeper every round." },
  { id: "web", tier: "school", innate: true, level: 9, stats: { intelligence: 15 }, mana: 10, price: 900, name: "Web", branch: "arcane", school: "conjuration", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    target: "area", save: "reflex", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.6 }, rounds: 3 }; },
    description: "Sticky strands fill the room." },
  { id: "ice_storm", tier: "school", kind: "cold", level: 16, stats: { intelligence: 17 }, mana: 18, price: 1600, name: "Ice Storm", branch: "arcane", school: "conjuration", castRounds: 2, interruptOnDamage: true, cooldown: 8,
    target: "area", save: "reflex", saveEffect: "half", damage: 1.7,
    description: "Hail the size of fists hammers the room." },
  { id: "cloudkill", tier: "school", kind: "poison", level: 24, stats: { intelligence: 19 }, mana: 22, price: 2400, name: "Cloudkill", branch: "arcane", school: "conjuration", castRounds: 2, interruptOnDamage: true, cooldown: 12,
    target: "area", save: "fortitude", saveEffect: "half", dot: true,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.5 * power) }, rounds: 6 }; },
    description: "A yellow-green fog that chokes everyone in it." },
  { id: "incendiary_cloud", tier: "rare", kind: "fire", found: true, level: 40, stats: { intelligence: 22 }, mana: 38, name: "Incendiary Cloud", branch: "arcane", school: "conjuration", castRounds: 3, interruptOnDamage: true, cooldown: 25,
    target: "area", save: "reflex", saveEffect: "half", damage: 1.5,
    effect: function (caster, target, power) { return { kind: "dot", params: { damage: Math.round(0.6 * power) }, rounds: 6 }; },
    description: "A rolling cloud of smoke and live embers." },

  // --- Once divination (cut 2026-10-12): now common, abjuration, illusion, enchantment, evocation ---
  { id: "true_strike", tier: "common", level: 1, stats: { intelligence: 11 }, mana: 6, price: 100, name: "True Strike", branch: "arcane", school: "enchantment", castRounds: 0, interruptOnDamage: false, cooldown: 10,
    target: "self", save: "none",
    effect: function () { return { kind: "crit", params: { chance: 0.40, mult: 2 }, rounds: 3 }; },
    description: "You see exactly where to strike." },
  { id: "see_invisible", tier: "common", level: 6, stats: { intelligence: 13 }, mana: 8, price: 500, name: "See Invisible", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 0,
    target: "self", save: "none",
    effect: function () { return { kind: "detectInvisible", params: {}, rounds: 60 }; },
    description: "The unseen show as shapes of light." },
  { id: "find_weakness", tier: "school", level: 10, stats: { intelligence: 16 }, mana: 10, price: 1000, name: "Find Weakness", branch: "arcane", school: "evocation", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    target: "single", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "protect", params: { mult: 1.25 }, rounds: 4 }; },
    description: "You read the flaw in a foe, and every blow finds it." },
  { id: "keen_insight", tier: "school", level: 16, stats: { intelligence: 17 }, mana: 16, price: 1600, name: "Insight", branch: "arcane", school: "abjuration", castRounds: 1, interruptOnDamage: false, cooldown: 20,
    target: "group", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.08 }, rounds: 8 }; },
    description: "Your companions hear your warnings a moment early." },
  { id: "true_seeing", tier: "school", level: 28, stats: { intelligence: 20 }, mana: 20, price: 3000, name: "True Seeing", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 30,
    target: "self", save: "none",
    effects: function () {
      return [{ kind: "detectInvisible", params: {}, rounds: 10 },
              { kind: "detectHidden", params: {}, rounds: 10 },
              { kind: "crit", params: { chance: 0.20, mult: 2 }, rounds: 10 },
              { kind: "dodge", params: { amount: 0.10 }, rounds: 10 }];
    },
    description: "Nothing is hidden from you, not even the next blow." },
  { id: "greater_foresight", tier: "rare", found: true, level: 36, stats: { intelligence: 21 }, mana: 30, name: "Greater Foresight", branch: "arcane", school: "abjuration", castRounds: 1, interruptOnDamage: false, cooldown: 40,
    target: "group", save: "none",
    effects: function () {
      return [{ kind: "dodge", params: { amount: 0.20 }, rounds: 6 },
              { kind: "crit", params: { chance: 0.25, mult: 2 }, rounds: 6 }];
    },
    description: "The whole fight unrolls before you, and you share it." },

  // --- Enchantment: the mind bent, and since 2026-10-12 the body too ---
  { id: "mind_spike", tier: "common", kind: "psionic", level: 5, stats: { intelligence: 13 }, mana: 8, price: 500, name: "Mind Spike", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "single", save: "will", saveEffect: "half", damage: 1.8,
    description: "A thought driven like a nail; armor does not stop it, will does." },
  { id: "sleep", tier: "school", innate: true, level: 7, stats: { intelligence: 14 }, mana: 10, price: 700, name: "Sleep", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 8,
    target: "single", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.25 }, rounds: 3 }; },
    description: "The target nods and barely lifts its arms." },
  { id: "arcane_heroism", tier: "school", level: 10, stats: { intelligence: 16 }, mana: 12, price: 1000, name: "Heroism", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 20,
    target: "ally", save: "none",
    effects: function () {
      return [{ kind: "damage", params: { mult: 1.15 }, rounds: 10 },
              { kind: "steady", params: { chance: 0.5 }, rounds: 10 }];
    },
    description: "Courage put into a mind like a hot coal." },
  { id: "confusion", tier: "school", level: 18, stats: { intelligence: 18 }, mana: 18, price: 1800, name: "Confusion", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 12,
    target: "area", save: "will", saveEffect: "negate",
    effects: function () {
      return [{ kind: "blind", params: { miss: 0.30 }, rounds: 3 },
              { kind: "speedMult", params: { mult: 0.8 }, rounds: 3 }];
    },
    description: "Every enemy forgets which way the fight is." },
  { id: "hold_monster", tier: "school", level: 28, stats: { intelligence: 20 }, mana: 22, price: 3000, name: "Hold Monster", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 15,
    target: "single", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.2 }, rounds: 4 }; },
    description: "Any creature, held rigid in its tracks." },
  { id: "power_word_stun", tier: "rare", found: true, level: 38, stats: { intelligence: 21 }, mana: 30, name: "Power Word Stun", branch: "arcane", school: "enchantment", castRounds: 0, interruptOnDamage: false, cooldown: 30,
    target: "single", save: "none",
    effects: function () {
      return [{ kind: "speedMult", params: { mult: 0.1 }, rounds: 3 },
              { kind: "protect", params: { mult: 1.2 }, rounds: 3 }];
    },
    description: "One word, and no mind can refuse it." },

  // --- Illusion: what is seen, and what is not ---
  { id: "color_spray", tier: "common", level: 2, stats: { intelligence: 11 }, mana: 7, price: 150, name: "Color Spray", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: true, cooldown: 6,
    target: "area", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "blind", params: { miss: 0.40 }, rounds: 2 }; },
    description: "A fan of clashing colours dazzles the room." },
  { id: "invisibility", tier: "school", innate: true, level: 6, stats: { intelligence: 13 }, mana: 10, price: 600, name: "Invisibility", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    target: "ally", save: "none",
    effect: function () { return { kind: "invisible", params: {}, rounds: 30 }; },
    description: "Light bends around the target." },
  { id: "mirror_image", tier: "school", level: 9, stats: { intelligence: 15 }, mana: 12, price: 900, name: "Mirror Image", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 20,
    target: "self", save: "none",
    effect: function () { return { kind: "dodge", params: { amount: 0.25 }, rounds: 4 }; },
    description: "Copies of you crowd the room, and blows strike them." },
  { id: "phantasmal_killer", tier: "school", kind: "psionic", level: 20, stats: { intelligence: 18 }, mana: 18, price: 2000, name: "Phantasmal Killer", branch: "arcane", school: "illusion", castRounds: 2, interruptOnDamage: true, cooldown: 8,
    target: "single", save: "will", saveEffect: "half", damage: 2.6,
    description: "The target's worst fear, made real only to it." },
  { id: "mislead", tier: "school", level: 30, stats: { intelligence: 20 }, mana: 22, price: 3600, name: "Mislead", branch: "arcane", school: "illusion", castRounds: 1, interruptOnDamage: false, cooldown: 30,
    target: "self", save: "none",
    effects: function () {
      return [{ kind: "dodge", params: { amount: 0.30 }, rounds: 6 },
              { kind: "crit", params: { chance: 0.20, mult: 2 }, rounds: 6 }];
    },
    description: "Your double fights in plain sight while you strike from beside it." },
  { id: "weird", tier: "rare", kind: "psionic", found: true, level: 38, stats: { intelligence: 21 }, mana: 36, name: "Weird", branch: "arcane", school: "illusion", castRounds: 2, interruptOnDamage: true, cooldown: 25,
    target: "area", save: "will", saveEffect: "half", damage: 2.5,
    effect: function () { return { kind: "speedMult", params: { mult: 0.5 }, rounds: 2 }; },
    description: "Every enemy meets its own nightmare at once." },

  // --- Necromancy: cold, rot, and stolen life ---
  { id: "chill_touch", tier: "common", kind: "drain", level: 1, stats: { intelligence: 11 }, mana: 6, price: 100, name: "Chill Touch", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 0,
    target: "single", save: "fortitude", saveEffect: "half", damage: 1.6,
    effect: function () { return { kind: "stat", params: { stat: "strength", amount: -2 }, rounds: 3 }; },
    description: "A grave-cold hand that saps the strength it touches." },
  { id: "ray_of_enfeeblement", tier: "school", innate: true, level: 8, stats: { intelligence: 15 }, mana: 10, price: 800, name: "Ray of Enfeeblement", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 8,
    target: "single", save: "fortitude", saveEffect: "negate",
    effect: function () { return { kind: "stat", params: { stat: "strength", amount: -5 }, rounds: 8 }; },
    description: "A black ray that leaves the target's arms like water." },
  { id: "false_life", tier: "school", level: 12, stats: { intelligence: 16 }, mana: 12, price: 1200, name: "False Life", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 20,
    target: "self", save: "none",
    effect: function () { return { kind: "regen", params: { fraction: 0.03 }, rounds: 10 }; },
    description: "Borrowed vigour knits your wounds as you fight." },
  { id: "vampiric_touch", tier: "school", kind: "drain", level: 16, stats: { intelligence: 17 }, mana: 14, price: 1600, name: "Vampiric Touch", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 6,
    target: "single", save: "fortitude", saveEffect: "half", damage: 2.2, drain: 0.75,
    description: "Your touch drinks the life out of them and into you." },
  { id: "circle_of_death", tier: "school", kind: "drain", level: 32, stats: { intelligence: 20 }, mana: 30, price: 3800, name: "Circle of Death", branch: "arcane", school: "necromancy", castRounds: 2, interruptOnDamage: true, cooldown: 15,
    target: "area", save: "fortitude", saveEffect: "half", damage: 2.0,
    description: "A ring of negative energy withers everyone in it." },
  { id: "finger_of_death", tier: "rare", kind: "drain", found: true, level: 44, stats: { intelligence: 22 }, mana: 38, name: "Finger of Death", branch: "arcane", school: "necromancy", castRounds: 1, interruptOnDamage: true, cooldown: 30,
    target: "single", save: "fortitude", saveEffect: "half", damage: 4.5, drain: 0.25,
    description: "You point, and the target begins to die." },

  // --- Once transmutation (folded into enchantment 2026-10-12): flesh, stone, and time ---
  { id: "stone_fist", tier: "common", level: 2, stats: { intelligence: 11 }, mana: 6, price: 150, name: "Stone Fist", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: false, cooldown: 10,
    target: "self", save: "none",
    effects: function () {
      return [{ kind: "damage", params: { mult: 1.15 }, rounds: 6 },
              { kind: "unarmedMult", params: { mult: 1.5 }, rounds: 6 }];
    },
    description: "Your hands turn to stone, and so does whatever they hold." },
  { id: "slow", tier: "school", level: 12, stats: { intelligence: 16 }, mana: 14, price: 1200, name: "Slow", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 10,
    target: "area", save: "will", saveEffect: "negate",
    effect: function () { return { kind: "speedMult", params: { mult: 0.6 }, rounds: 3 }; },
    description: "Every enemy moves as if through deep water." },
  { id: "iron_body", tier: "school", level: 22, stats: { intelligence: 18 }, mana: 20, price: 2400, name: "Iron Body", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: true, cooldown: 30,
    target: "self", save: "none",
    effects: function (caster) {
      return [{ kind: "soak", params: { power: 0.30 }, rounds: 8 },
              { kind: "defense", params: { amount: (caster.level || 1) + 2 }, rounds: 8 },
              { kind: "speedMult", params: { mult: 0.85 }, rounds: 8 }];
    },
    description: "Your flesh becomes iron: heavy, slow, and very hard to hurt." },
  { id: "tensers_transformation", tier: "school", level: 24, stats: { intelligence: 19 }, mana: 22, price: 2600, name: "Tenser's Transformation", branch: "arcane", school: "enchantment", castRounds: 2, interruptOnDamage: true, cooldown: 30,
    target: "self", save: "none",
    effects: function (caster) {
      return [{ kind: "attacks", params: { amount: 1 }, rounds: 8 },
              { kind: "damage", params: { mult: 1.25 }, rounds: 8 },
              { kind: "defense", params: { amount: Math.round((caster.level || 1) / 2) }, rounds: 8 }];
    },
    description: "The scholar becomes a warrior, for a while." },
  { id: "disintegrate", tier: "school", kind: "magic", level: 34, stats: { intelligence: 21 }, mana: 28, price: 4000, name: "Disintegrate", branch: "arcane", school: "enchantment", castRounds: 2, interruptOnDamage: true, cooldown: 15,
    target: "single", save: "fortitude", saveEffect: "half", damage: 3.5,
    description: "A thin green ray, and what it touches turns to dust." },
  { id: "time_stop", tier: "rare", found: true, level: 45, stats: { intelligence: 22 }, mana: 40, name: "Time Stop", branch: "arcane", school: "enchantment", castRounds: 1, interruptOnDamage: false, cooldown: 50,
    target: "area", save: "none",
    effect: function () { return { kind: "speedMult", params: { mult: 0.1 }, rounds: 5 }; },
    description: "The world stops for everyone but you." }
);
