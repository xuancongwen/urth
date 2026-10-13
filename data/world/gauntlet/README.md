# gauntlet

The Gauntlet, from the 1985 arcade game: a dungeon of grey stone mazes
built as arcade levels, one under the next, each ending at a painted
EXIT over a shaft down. Ghosts on level 1, grunts on level 2, demons on
the dark level 3, lobbers and sorcerers on level 4, and Death waiting
on the last level. Levels 28 to 34, and Death at 36. Reached down the
stair from the Hall of Distant Stars (room 10 in `start`), which lets
out at the coin slot (2900).

This is a farming area. The rooms are crowded, every common monster is
flagged `assist` so attacking one brings its kind in
the room down on you, the reset interval is eight minutes, and there is
plain drop gear at every level from 27 to 37 so any kill can roll loot.
Generators (bone piles, grunt huts, demon pits) do not fight back but
keep adding their monster to the room (`spawns`) until destroyed, and
the reset puts them back. Grim ghosts, grunts, demons, lobbers, and
sorcerers attack on sight; the faint ghosts of level 1 wait to be hit.

Generators: a small bone pile (28, faint ghosts, 2902), a great bone
pile (30, grim ghosts, 2904), grunt huts (30, grunts, 2906 and 2908),
demon pits (31, demons, 2910 and 2911).

Bosses: Death (36, the last level 2917), who leeches health with every
touch and wears the amulet of invulnerability. Named carriers: Big Grunt
(31, Thor's axe, in the grunt huts), the Demon Mother (32, Thyra's
sword, at the furnace), the Lobber King (33, Questor's bow, in the
lobbers' gallery), the Sorcerer Supreme (34, Merlin's staff, in the
sorcerers' hall). The thief (31) wanders the levels.

Uniques: Thor's axe (a hurled axe that comes back, Strength and
Constitution), Thyra's sword (blocks and crits, Constitution and
Dexterity), Questor's bow (a second arrow, dodge, Dexterity), Merlin's
staff (wizard's lightning, Intelligence and Wisdom, arcane spells
stronger and surging), the amulet of
invulnerability (takes less damage, soaks hits, Constitution). Common
finds: a sorcerer's crooked staff on each sorcerer (2953; 33,
Intelligence, arcane spells stronger, mana), plain drop gear at every level 27 to 37 (swords, maces, axes, a
flail, a spear, a maul, mail, plate, a tower shield), ectoplasm from
ghosts, grunt tusks, demon horns, gold keys, roast chicken, cider, and
magic potions in the treasure rooms.

Sorcerers (33) are invisible; Gorm sells seeing spectacles that reveal
them. Level 3 (2909, 2910, 2911) is dark; Gorm sells pitch torches, and
one lies at the top of the shaft (2908).

Doors: every locked door takes the same gold key, which Gorm sells, a
grunt on the landing carries, and one lies in the bone yard. Locked: the
treasure room (2903), the larder (2907), the exit of the dark level
(2909 north to 2912), and the vault (2913 north to 2916), which is the
way to Death.

The coin slot (2900) is safe. The Voice teaches fast healing and second
wind.

Merchant (`list`, `buy`, `sell`):

- Gorm the provisioner (2901, coin slot 2900): gold key 60 (or two
  wisps of ectoplasm), roast chicken 25, jug of cider 20, magic potion
  90, pitch torch 30, seeing spectacles 300; a warrior's horned helm
  for 450 silver plus three grunt tusks; a valkyrie's winged hauberk for
  550 silver plus two demon horns and two wisps of ectoplasm.

Quest item: 2908, a stone high-score tablet (planted by questmasters
only).

Map (x east, y south; one grid per level):

    level 1 (z=0)                 level 2 (z=-1)
      y=-2            2904          y=-3            2908
                       |                             |
      y=-1    2902 -- 2901 -- 2903  y=-2    2907 -- 2905 -- 2906
                       |
      y= 0            2900 -(up)-> hub 10

    level 3 (z=-2)                        level 4 (z=-3)
      y=-4    2912                          y=-5            2916
               |                                             |
      y=-3    2909 -- 2910 -- 2911          y=-4    2914 -- 2913 -- 2915
             x=0     x=1     x=2                   x=-1    x=0     x=1

Down from 2904 to 2905, from 2908 to 2909, from 2912 to 2913, and from
2916 to 2917 the last level (0, -5, -4).

2900 the coin slot (anchor 0,0,0) · 2901 the stone maze · 2902 the bone
yard · 2903 the treasure room · 2904 level 1 exit · 2905 level 2
landing · 2906 the grunt huts · 2907 the larder · 2908 the club run
2909 the dark level · 2910 the demon pits · 2911 the furnace · 2912
level 3 exit · 2913 the crossfire · 2914 the lobbers' gallery · 2915
the sorcerers' hall · 2916 the treasure vault · 2917 the last level
