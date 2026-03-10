# Winter Challenge 2026 — Exotec: Rules

## Goal

Collect power sources to grow your snake-like robots. When the game ends, the player with the most total body parts across all remaining snakebots wins.

## Rules

The game is played on a grid.

Each player controls a team of snakebots. On each turn, all snakebots move simultaneously according to the submitted commands.

### Map

- `#` is a platform. Platforms are impassable.
- `.` is a free cell.
- The grid may also contain snakebot body parts and power sources.

### Snakebots

- A snakebot is made of adjacent body parts.
- The first body part is the head.
- Snakebots are affected by gravity.
- At least one body part must be above something solid, otherwise the snakebot falls.
- Platforms, power sources, and other snakebots count as solid.

### Movement

- Snakebots move every turn, even if you do not issue a new direction.
- A snakebot keeps moving in the last direction it was facing unless you change it.
- The starting direction is `UP`.
- When a snakebot moves, its head advances one cell in its current direction and the rest of its body follows.

#### Collision with a platform or body part

If a snakebot head enters a cell containing a platform or any body part:

- Its head is destroyed.
- The next body part becomes the new head if at least three parts remain.
- Otherwise the entire snakebot is removed.

#### Collision with a power source

If a snakebot head enters a cell containing a power source:

- The snakebot eats that power source.
- The snakebot grows by one body part at its tail.
- That cell is no longer considered solid after being eaten.

All head movements and collisions are resolved simultaneously for all snakebots.

Special case:

- If multiple snakebot heads move onto the same cell containing a power source, that power source is considered eaten by each of those snakebots.

### Falling

- After movement and removals are resolved, snakebots fall downward until one of their body parts lands on something solid.
- Snakebots may temporarily extend beyond the borders of the grid.
- If a snakebot falls out of the playing area, it is removed.

## Actions

Each turn, your program must print a single line containing at least one action. Actions are separated by semicolons (`;`).

Movement commands for a snakebot you control:

- `id UP` sets its direction to `UP` with delta `(0, -1)`.
- `id DOWN` sets its direction to `DOWN` with delta `(0, 1)`.
- `id LEFT` sets its direction to `LEFT` with delta `(-1, 0)`.
- `id RIGHT` sets its direction to `RIGHT` with delta `(1, 0)`.

Any movement action may be followed by extra text. That text is displayed above the corresponding snakebot for debugging purposes.

Special commands:

- `MARK x y` places a marker at the given coordinates for debugging in the viewer.
- `WAIT` does nothing.

You may place up to 4 markers per turn.

Example:

```text
1 LEFT;2 RIGHT;MARK 12 2
```

## Game End

The game ends at the end of a turn if any of the following is true:

- All snakebots of one player have been removed.
- There are no power sources left.
- 200 turns have elapsed.

## Victory Conditions

Have more total body parts across all your snakebots than your opponent at the end of the game.

## Defeat Conditions

You lose if:

- Your program does not output a command within the allotted time.
- One of your commands is invalid.

## Initialization Input

```text
Line 1: myId                 integer  your player id (0 or 1)
Line 2: width                integer  grid width
Line 3: height               integer  grid height
Next height lines:                    one row of width characters each:
  #                                   platform
  .                                   free cell
Next line: snakbotsPerPlayer  integer  number of snakebots per player
Next snakbotsPerPlayer lines: integer  snakebotId for your snakebots
Next snakbotsPerPlayer lines: integer  snakebotId for the opponent snakebots
```

## Turn Input

```text
Line 1: powerSourceCount      integer  number of remaining power sources
Next powerSourceCount lines:
  x y                                  coordinates of one power source

Next line: snakebotCount      integer  number of remaining snakebots
Next snakebotCount lines:
  snakebotId body
    snakebotId                         integer identifier
    body                               colon-separated "x,y" coordinates
                                       the first coordinate is the head
                                       example: "0,1:1,1:2,1"
```

## Output

Print a single line with at least one action.

Valid actions:

```text
id UP
id DOWN
id LEFT
id RIGHT
MARK x y
WAIT
```

Actions must be separated by `;`.

## Constraints

| Parameter | Value |
| --- | --- |
| `width` | `15` to `45` |
| `height` | `10` to `30` |
| `snakebotCount` | `1` to `8` |
| Response time per turn | `<= 50 ms` |
| Response time for first turn | `<= 1000 ms` |

## Debugging Tips

- Use `MARK x y` to highlight up to 4 cells per turn.
- Hover over the grid in the viewer to inspect a cell.
- Use the viewer gear icon for extra display options.
- Keyboard controls: space to play or pause, arrow keys to step frame by frame.

## Technical Details

- The source code of the game is published at <https://github.com/CodinGame/WinterChallenge2026-Exotec>.
