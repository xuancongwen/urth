# data/

Root for everything the server reads and writes at runtime.

Planned layout (filled in by later milestones):

- `world/`    area files: rooms, exits, items, mobs, resets (milestones 2 and 5)
- `players/`  saved accounts and characters, one YAML file each (milestone 4, gitignored)
- `scripts/`  goja JavaScript rule hooks: damage, hit resolution, cast, tick, level (milestone 6)
