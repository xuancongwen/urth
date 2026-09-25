# balance

A builder-only area holding one plain mob per level, 1 to 20, for the
simulator. Nothing here spawns (there is no resets.yaml) and the one
room has no exits. Use it to fill the win-rate row of docs/RULES.md 4.5:

    simulate fighter:5 906 1000 1     # a level-5 fighter in standard kit against the level-6 dummy
    simulate fighter:5 908 1000 1     # +3
    simulate fighter:5 910 1000 1     # +5

Dummy vnum is 900 + level. Every dummy is its level's baseline: six
stats at 10, natural attack and armor from mobBaseline, no equipment.
