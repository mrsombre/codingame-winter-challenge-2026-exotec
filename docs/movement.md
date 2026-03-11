# Movement & Physics

## Turn Order

Each turn:
1. **Move** — all bots move simultaneously (head advances, tail removed unless eating)
2. **Eat** — bots on an apple consume it (body grows by 1, score +1)
3. **Behead** — bots whose head occupies a wall or any other bot's body lose their head segment; if body ≤ 3 segments after beheading, bot dies
4. **Fall** — bots without ground support drop until supported

## Fall Physics

A bot has **support** if any of its segments has a solid cell directly below it (wall, other bot's body, apple). The bot's **own body** does not count as support for itself.

After every move, fall repeats until all bots are supported:
- The entire bot drops one row per iteration
- If the bot exits the grid entirely, it dies

Intercoiled groups (bots touching each other) fall together as a unit.

## Climbing Limit

A bot of body length **L** rests with its tail on the floor and head at `floor_y − (L−1)`, i.e. **L−1 rows above the floor**.

| Body length | Max crossable step height |
|---|---|
| 3 | 2 |
| 4 | 3 |
| L | L−1 |

---

## Situation 1 — Cannot climb wall ≥ own length

Bot L=3, wall height 3. Head sits at wall-top level. Going UP removes the tail
(the only floor anchor) → no support → falls back. Repeats forever.

```
  initial          tries UP         falls back

  . . .            x . .            . . .
  x # .            x # .            x # .   ← head stuck here
  x # .    →       x # .    →       x # .
  x # .            . # .            x # .
  # # #            # # #            # # #
                   ^no anchor:      ^back to start
                   tail gone,
                   falls back
```

Bot needs L=4 to clear: head would already sit at y=0 (above the wall) and
can step RIGHT freely.

---

## Situation 2 — Move sideways, fall, anchor shifts, gain height

Vertical bot moves RIGHT → falls diagonally. The anchor cell (bottom of old
column) stays on the floor while head/body shift. A follow-up UP succeeds
because the new bottom segment now sits over a wall.

```
  vertical         RIGHT + fall     UP (anchor      UP again
                   (diagonal)       shifts)         (gained row)

  x . .          . . .          . . .            . x .
  x . .    →     x x .    →     . x .    →       . x .
  x . .          x . .          x x .            . x .
  # # #          # # #          # # #            # # #
                   ^anchor           ^anchor shifted   ^fully vertical
                   at (0,2)          to (1,2)          one row higher
```

Key: every UP move from a diagonal position shifts the anchor one column
over. When the bot becomes fully vertical again it is one row higher than
it started. This is the only reliable way to gain height.

---

## Situation 3 — Horizontal movement builds body along floor

Vertical bot moves LEFT repeatedly. After the first move+fall the body goes
diagonal; after the second it lies fully horizontal on the floor. Further
LEFT moves slide the whole body with zero fall penalty — the bot "skates".
This is how an enemy can quickly build a blocking wall of body segments.

```
  vertical         LEFT + fall      LEFT again       LEFT (skates)
                   (diagonal)       (horizontal)

  . . x .          . . . .          . . . .          . . . .
  . . x .    →     . x x .    →     . . . .    →     . . . .
  . . x .          . . x .          x x x .          x x x .
  # # # #          # # # #          # # # #          # # # #
```

Once horizontal the bot occupies a full row of the floor — it becomes a
physical wall that blocks vertical passage for any opponent above.

---

## Practical Notes

- **Horizontal traversal of rising terrain** works if each step is ≤ L−1 high.
- **Flat-floor UP moves are useless** for gaining height — fall physics cancel them.
- **Diagonal → UP** is the correct climbing sequence (situation 2).
- **Eating an apple** before a step increases L by 1, raising the climbable limit.
- **Seed-18 map** (18×10, bronze): all terrain steps are 1-row high — crossable by the initial length-3 bot.
