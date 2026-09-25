# data/

Root for everything the server reads and writes at runtime.

Planned layout (filled in by later milestones):

- `world/<area>/`  `area.yaml`, `rooms/*.yaml`, `items/*.yaml`, `mobs/*.yaml`, `resets.yaml`
- `players/`  saved accounts and characters, one YAML file each (milestone 4, gitignored)
- `scripts/`  goja JavaScript rule hooks; `rules.js` is the live rule set (docs/RULES.md section 1 lists the hooks)
