# midgaard

Midgaard, the old city of every Diku mud, after ROM 2.4: the Temple and
its altar, Temple Square, Main Street and the Market Square with its
shops, the Common Square and the Town Hall, the guilds, the Grunting
Boar, both city gates, Poor Alley and the thieves, the river, the dump,
the graveyard, and the sewers under the city. Levels 1 to 15. Reached
through the west door of the Hall of Far Doors (room 7 in `start`), which
opens into the Temple of Midgaard (3001).

The temple, the altar, the shops, the bank, the inn, and the guild halls
are safe. The sewers and the crypt are dark. Vnums 3000 to 3099 for rooms,
items, and mobs.

Map (x east, y south; the temple's east door leads to the hub, room 7):

    z=0, the city
     y=-1                                            3054
                                                      |
     y= 0                              3003 - 3002 - 3001                        3028
                                                      |                   |
     y= 1                3020   3019   3009   3004 - 3005 - 3006 - 3007   3026   3027
                          |      |      |  (up 3008)  |                    |      |
     y= 2  3052 - 3040 - 3010 - 3011 - 3012 - 3013 - 3014 - 3015 - 3016 - 3017 - 3018 - 3041 - 3053
                          |      |      |             |
     y= 3                3023   3022   3021   3031 - 3025 - 3036 - 3037
                                               |      |
     y= 4                3051 - 3050 - 3034 - 3033   3042 - 3048 - 3049
                                         (down 3060)  |
     y= 5                                            3043
                                                      |
     y= 6                                     3046 - 3044 - 3045
                                        (down 3047)
          x=-6   -5     -4     -3     -2     -1      0      1      2      3      4      5      6

    z=-1, the sewers and the crypt
     y= 4  3060 - 3061 - 3062        3060 up to 3033
            |             |
     y= 5  3065          3063 - 3064
            |
     y= 6  3047                      3047 up to 3046
          x=-1    0      1      2

3008 (the Mages' Tower Study) is at z=1 above 3004.

3001 Temple of Midgaard (anchor 0,0,0) · 3054 By the Temple Altar
3002 Clerics' Guild · 3003 Clerics' Inner Sanctum · 3005 Temple Square
3004 Mages' Guild · 3008 Mages' Tower Study · 3006 Grunting Boar Inn
3007 Bar of the Grunting Boar · 3010-3013, 3015-3018 Main Street
3014 Market Square · 3040 West Gate · 3052 outside the West Gate
3041 East Gate · 3053 outside the East Gate · 3009 Bakery · 3019 Grocer
3020 General Store · 3021 Armoury · 3022 Weaponsmith · 3023 Magic Shop
3026 Bank · 3027 Warriors' Guild · 3028 Warriors' Training Hall
3025 Common Square · 3036 Town Hall · 3037 Mayor's Office
3031, 3033 Poor Alley · 3034 Dark Alley · 3050 Thieves' Guild
3051 Thieves' Den · 3042 the Bridge · 3048 Riverbank · 3049 City Dump
3043 Graveyard Gate · 3044 Graveyard · 3045 Old Graves · 3046 Mausoleum
3060-3063 the Sewers (dark) · 3064 Rats' Nest (dark)
3065 Collapsed Tunnel (dark) · 3047 the Crypt (dark)

## Merchants

`list`, `buy`, and `sell` in the shop's room. Three trades ask for
something found in Midgaard as well as coin:

| Merchant | Room | Stock |
|---|---|---|
| the baker | 3009 | bread, firebreather |
| the grocer | 3019 | waterskin, trail rations |
| the shopkeeper | 3020 | torch, brass lantern, backpack, wooden shield |
| the armourer | 3021 | jerkin, boots, iron helm; **bronze-bound shield: 150 silver + two sewer rat hides** (rats in the sewers) |
| the weaponsmith | 3022 | dagger, short sword, war hammer; **fine steel broadsword: 180 silver + a lump of scrap iron** (the dump) |
| the wizard | 3023 | ring of the owl; **amulet of warding: 120 silver + two pinches of grave dust** (graveyard zombies); fido-bone charm: two gnawed bones (beastly fidos) |
| the bartender | 3007 | dark ale, firebreather |

## Mobs

Fido (1), the cat (1), a beggar (1), the janitor (2), the drunk (2),
the beastly fido (3), sewer rats (3), the gravedigger (4), a pickpocket
(5), restless zombies (6), the rat king (7), the crypt ghoul (9),
cityguards (10), the mayor (13), the Peacekeeper (14), and Hassan (15)
in the Grunting Boar. Peaceful: the shopkeepers, the banker, the high
priest, the sorcerer, the master thief (teaches riposte), and the
warrior guildmaster (teaches bash and twin).

The Mayor's lost ledger (3035) is a quest item, planted only by a
questmaster.
