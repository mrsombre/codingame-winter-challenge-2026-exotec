# Winter Challenge 2026 — Exotec: Rules

## Initialization Input

```
Line 1: myId          integer  your player id (0 or 1)
Line 2: width         integer  grid width
Line 3: height        integer  grid height
Next height lines:             one row of width characters each:
  #  platform
  .  free cell
Next line: snakbotsPerPlayer   integer  snakebots per player
Next snakbotsPerPlayer lines:  integer snakebotId  your snakebots
Next snakbotsPerPlayer lines:  integer snakebotId  opponent's snakebots
```

## Turn Input

```
Line 1: powerSourceCount       integer  remaining power sources
Next powerSourceCount lines:
  x y                          coordinates of each power source

Line N: snakebotCount          integer  remaining snakebots
Next snakebotCount lines:
  snakebotId body
    snakebotId  integer
    body        colon-separated "x,y" coordinates; first is the head
                e.g. "0,1:1,1:2,1" — head at (0,1), two parts to the right
```

## Output

A single line with at least one action, separated by `;`:

```
id UP        move snakebot id up
id DOWN      move snakebot id down
id LEFT      move snakebot id left
id RIGHT     move snakebot id right
MARK x y    mark a coordinate (up to 4 per turn)
WAIT
```

Example:

```
1 LEFT;2 RIGHT;MARK 12 2
```

## Constraints

| Parameter             | Value          |
|-----------------------|----------------|
| width                 | 15 – 45        |
| height                | 10 – 30        |
| snakebotCount         | 1 – 8          |
| Response time / turn  | ≤ 50 ms        |
| Response time / turn 1| ≤ 1000 ms      |
