# balance

The test lab. Detached: nothing leads here on foot, the content check
leaves it alone, and quests never send anyone here. An admin arrives
with `goto 900`. Everything the regular world does has a place here, so
a mechanic can be tried without walking to where it lives.

| Room | What it tests |
|---|---|
| 900 The Balance Range | the hub; safe; a closed door south (open, close, doors reset shut) |
| 945 A Sunlit Shrine | the Good temple, behind the door |
| 940 The Test Armory | one of nearly everything: weapons one- and two-handed, every armor slot, the level-2 iron set, lights, a backpack and a purse (containers), bread, bucket, candle, the common materials, wolf fangs (one loose, one in each container) |
| 941 The Muster Room | a drillmaster (bash, twin) and a quartermaster (quests, quest points, and the herald's wares) |
| 903 The Arena Gate | into the arena, and west to the colleges |
| 910-925 the arena | fights by row, levels 1 to 3 at the gate up to the troll: dogs, crows, a beggar, assisting wolves carrying fangs, an armed bandit and guard, a bog wight, a blacksmith, a wolf pack, and dummies 905, 906, 910, 915, 920, 925; the hauberk, ring, boots, and Sundering Blade lie with their guards |
| 926 A Dark Cell | darkness (bring a light), the salt totem, the chalice |
| 930-936 the colleges | all eight totems (931), rare materials and the other gods' sacrifices (932), the Neutral (934) and Evil (935) temples, the beast-master (rend) and fencing master (riposte) in the yards (936) |
| 947, 948 | two unfinished rooms off the arena's southeast corner |

Mobs 901 to 925 are one plain dummy per level for the simulator. Use
them to fill the win-rate row of docs/RULES.md 4.5:

    simulate fighter:5 906 1000 1     # a level-5 fighter in standard kit against the level-6 dummy
    simulate fighter:5 908 1000 1     # +3
    simulate fighter:5 910 1000 1     # +5

Dummy vnum is 900 + level. Every dummy is its level's baseline: six
stats at 10, natural attack and armor from mobBaseline, no equipment.

The lab's own items are 926 to 990 and its mobs 930 to 943. Some of what
it places belongs to the start area (starter gear, the common
materials, the herald's wares), because the regular world uses those
too.
