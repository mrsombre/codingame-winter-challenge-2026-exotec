from __future__ import annotations

import base64
import gzip
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Dict, List, Tuple

MAX_W = 45
MAX_H = 30
MAX_AP = 128
MAX_SEG = 256
MAX_TURNS = 200
MAX_PS = 4

DU = 0
DR = 1
DD = 2
DL = 3
DIR_NAMES = ["UP", "RIGHT", "DOWN", "LEFT"]
DXY = [(0, -1), (1, 0), (0, 1), (-1, 0)]
OPPOSITE = [DD, DL, DU, DR]
FEATURE_COUNT = 96
CROP_RADIUS = 2
FEATURE_SCHEMA_VERSION = 2

APPLE_FEATURE_COUNT = 16
APPLE_FEATURE_SCHEMA_VERSION = 3


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def train_root() -> Path:
    return Path(__file__).resolve().parent


def traces_dir() -> Path:
    return train_root() / "traces"


def artifacts_dir() -> Path:
    return train_root() / "artifacts"


def fixtures_dir() -> Path:
    return train_root() / "fixtures"


def weights_out_path() -> Path:
    return repo_root() / "agent" / "nn" / "src" / "model_weights_generated.go"


def load_traces(path: Path) -> List[dict]:
    rows = []
    static_by_match = {}
    with gzip.open(path, "rt", encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if not line:
                continue
            row = json.loads(line)
            match_id = row["match_id"]
            if "width" in row:
                static_by_match[match_id] = {
                    "width": row["width"],
                    "height": row["height"],
                    "walls": row["walls"],
                }
            else:
                row.update(static_by_match[match_id])
            rows.append(row)
    return rows


def parse_command_line(line: str) -> Dict[int, int]:
    result: Dict[int, int] = {}
    for part in line.split(";"):
        fields = part.strip().split()
        if len(fields) < 2 or fields[0] in {"WAIT", "MARK"}:
            continue
        try:
            snake_id = int(fields[0])
        except ValueError:
            continue
        if fields[1] not in DIR_NAMES:
            continue
        result[snake_id] = DIR_NAMES.index(fields[1])
    return result


def idx(x: int, y: int, width: int) -> int:
    return y * width + x


def xy(cell: int, width: int) -> Tuple[int, int]:
    return cell % width, cell // width


def body_dir(body: List[int], width: int) -> int:
    if len(body) < 2 or body[0] < 0 or body[1] < 0:
        return DU
    ax, ay = xy(body[1], width)
    bx, by = xy(body[0], width)
    dx = bx - ax
    dy = by - ay
    if dx == 1:
        return DR
    if dx == -1:
        return DL
    if dy == 1:
        return DD
    return DU


@dataclass
class Candidate:
    legal: bool
    dir: int
    head: int = -1
    body: List[int] | None = None
    eating: bool = False
    supported: bool = False
    fall_distance: int = 0
    flood: int = 0
    safe_moves: int = 0
    wall_adj: int = 0
    blocked_adj: int = 0
    head_on_risk: bool = False
    head_on_win: bool = False
    race_delta: int = 0
    features: List[float] | None = None


def build_state(row: dict, teacher_owner: int = 0) -> dict:
    width = int(row["width"])
    height = int(row["height"])
    wall = [False] * (width * height)
    for y, line in enumerate(row["walls"]):
        for x, ch in enumerate(line):
            wall[idx(x, y, width)] = ch == "#"

    apples = [idx(a["x"], a["y"], width) for a in row["apples"]]
    occupied = [-1] * (width * height)
    snakes = []
    for sn_index, sn in enumerate(row["snakes"]):
        body = []
        for p in sn["body"]:
            x = int(p["x"])
            y = int(p["y"])
            if in_bounds(x, y, width, height):
                body.append(idx(x, y, width))
            else:
                body.append(-1)
        owner = int(sn["owner"])
        snakes.append(
            {
                "id": int(sn["id"]),
                "owner": 0 if owner == teacher_owner else 1,
                "body": body,
                "dir": body_dir(body, width),
            }
        )
        for cell in body:
            if cell >= 0:
                occupied[cell] = sn_index

    return {
        "width": width,
        "height": height,
        "wall": wall,
        "walls": row["walls"],
        "apples": apples,
        "apple_set": set(apples),
        "occupied": occupied,
        "snakes": snakes,
        "turn": int(row["turn"]),
    }


def in_bounds(x: int, y: int, width: int, height: int) -> bool:
    return 0 <= x < width and 0 <= y < height


def is_supported(body: List[int], sn_index: int, eaten_apple: int, state: dict) -> bool:
    width = state["width"]
    height = state["height"]
    for cell in body:
        x, y = xy(cell, width)
        ny = y + 1
        if not in_bounds(x, ny, width, height):
            continue
        below = idx(x, ny, width)
        if state["wall"][below]:
            return True
        if below in state["apple_set"] and below != eaten_apple:
            return True
        if state["occupied"][below] >= 0 and state["occupied"][below] != sn_index:
            return True
    return False


def rotate_offset(x: int, y: int, dir_idx: int) -> Tuple[int, int]:
    if dir_idx == DU:
        return x, y
    if dir_idx == DR:
        return -y, x
    if dir_idx == DD:
        return -x, -y
    if dir_idx == DL:
        return y, -x
    return x, y


def nearest_enemy_dist(head: int, sn_index: int, state: dict) -> float:
    width = state["width"]
    hx, hy = xy(head, width)
    best = None
    for i, snake in enumerate(state["snakes"]):
        if i == sn_index or snake["owner"] == 0 or not snake["body"]:
            continue
        if snake["body"][0] < 0:
            continue
        ex, ey = xy(snake["body"][0], width)
        dx = ex - hx
        dy = ey - hy
        dist = abs(dx) + abs(dy)
        if best is None or dist < best:
            best = dist
    if best is None:
        return 1.0
    return best / 75.0


def build_blocked(state: dict, sn_index: int, body: List[int]) -> List[bool]:
    blocked = list(state["wall"])
    for cell, owner in enumerate(state["occupied"]):
        if owner >= 0 and owner != sn_index:
            blocked[cell] = True
    for cell in body[1:]:
        if cell >= 0:
            blocked[cell] = True
    return blocked


def flood_dist(start: int, blocked: List[bool], width: int, height: int) -> Tuple[int, List[int]]:
    dists = [-1] * (width * height)
    if start < 0 or start >= width * height or blocked[start]:
        return 0, dists
    queue = [start]
    dists[start] = 0
    count = 0
    qi = 0
    while qi < len(queue):
        cell = queue[qi]
        qi += 1
        count += 1
        cx, cy = xy(cell, width)
        for dx, dy in DXY:
            nx = cx + dx
            ny = cy + dy
            if not in_bounds(nx, ny, width, height):
                continue
            ncell = idx(nx, ny, width)
            if blocked[ncell] or dists[ncell] >= 0:
                continue
            dists[ncell] = dists[cell] + 1
            queue.append(ncell)
    return count, dists


def future_move_legal(state: dict, sn_index: int, body: List[int], facing: int, dir_idx: int, apple_set: set[int]) -> bool:
    if not body or body[0] < 0:
        return False
    if len(body) > 1 and dir_idx == OPPOSITE[facing]:
        return False

    width = state["width"]
    height = state["height"]
    hx, hy = xy(body[0], width)
    dx, dy = DXY[dir_idx]
    nx, ny = hx + dx, hy + dy
    if not in_bounds(nx, ny, width, height):
        return False
    next_cell = idx(nx, ny, width)
    if state["wall"][next_cell]:
        return False
    occ = state["occupied"][next_cell]
    if occ >= 0 and occ != sn_index:
        return False

    eating = next_cell in apple_set
    moved = [next_cell]
    if eating:
        moved.extend(body)
    else:
        moved.extend(body[:-1])
    if len(set(moved)) != len(moved):
        return False

    supported = is_supported(moved, sn_index, next_cell if eating else -1, state)
    while not supported:
        fallen = []
        for cell in moved:
            x, y = xy(cell, width)
            y += 1
            if not in_bounds(x, y, width, height):
                return False
            ncell = idx(x, y, width)
            if state["wall"][ncell]:
                return False
            occ = state["occupied"][ncell]
            if occ >= 0 and occ != sn_index:
                return False
            fallen.append(ncell)
        if len(set(fallen)) != len(fallen):
            return False
        moved = fallen
        supported = is_supported(moved, sn_index, next_cell if eating else -1, state)

    return True


def count_future_safe_moves(state: dict, sn_index: int, cand: Candidate) -> int:
    apple_set = set(state["apple_set"])
    if cand.eating and cand.head >= 0:
        apple_set.discard(cand.head)
    safe = 0
    for dir_idx in range(4):
        if cand.body and len(cand.body) > 1 and dir_idx == OPPOSITE[cand.dir]:
            continue
        if future_move_legal(state, sn_index, cand.body or [], cand.dir, dir_idx, apple_set):
            safe += 1
    return safe


def adjacent_counts(state: dict, sn_index: int, cand: Candidate) -> Tuple[int, int]:
    width = state["width"]
    hx, hy = xy(cand.head, width)
    local_occ = {cell for cell in (cand.body or [])[1:] if cell >= 0}
    walls = 0
    blocked = 0
    for dx, dy in DXY:
        nx = hx + dx
        ny = hy + dy
        if not in_bounds(nx, ny, width, state["height"]):
            walls += 1
            blocked += 1
            continue
        cell = idx(nx, ny, width)
        if state["wall"][cell]:
            walls += 1
            blocked += 1
            continue
        if cell in local_occ or (state["occupied"][cell] >= 0 and state["occupied"][cell] != sn_index):
            blocked += 1
    return walls, blocked


def head_on_signals(state: dict, sn_index: int, cand: Candidate) -> Tuple[bool, bool]:
    width = state["width"]
    my_len = len(cand.body or [])
    risk = False
    win = False
    for i, snake in enumerate(state["snakes"]):
        body = snake["body"]
        if i == sn_index or snake["owner"] == 0 or not body or body[0] < 0:
            continue
        hx, hy = xy(body[0], width)
        for dir_idx in range(4):
            if len(body) > 1 and dir_idx == OPPOSITE[snake["dir"]]:
                continue
            dx, dy = DXY[dir_idx]
            nx = hx + dx
            ny = hy + dy
            if not in_bounds(nx, ny, width, state["height"]):
                continue
            ncell = idx(nx, ny, width)
            if state["wall"][ncell] or ncell != cand.head:
                continue
            risk = True
            if my_len > 3 and len(body) <= 3:
                win = True
            break
    return risk, win


def enemy_apple_dist(state: dict, sn_index: int, apple: int) -> int:
    width = state["width"]
    ax, ay = xy(apple, width)
    best = None
    for i, snake in enumerate(state["snakes"]):
        body = snake["body"]
        if i == sn_index or snake["owner"] == 0 or not body or body[0] < 0:
            continue
        ex, ey = xy(body[0], width)
        dist = abs(ax - ex) + abs(ay - ey)
        if best is None or dist < best:
            best = dist
    return best if best is not None else state["width"] + state["height"]


def best_apple_target(state: dict, sn_index: int, cand: Candidate, dists: List[int]) -> Tuple[float, float, float]:
    width = state["width"]
    hx, hy = xy(cand.head, width)
    best_cell = None
    best_own = None
    best_race = None
    for apple in state["apples"]:
        if cand.eating and apple == cand.head:
            continue
        own_dist = dists[apple]
        if own_dist < 0:
            ax, ay = xy(apple, width)
            own_dist = abs(ax - hx) + abs(ay - hy) + state["width"] + state["height"]
        race = own_dist - enemy_apple_dist(state, sn_index, apple)
        if best_cell is None or own_dist < best_own or (own_dist == best_own and race < best_race):
            best_cell = apple
            best_own = own_dist
            best_race = race
    if best_cell is None:
        return 0.0, 0.0, 1.0
    ax, ay = xy(best_cell, width)
    return (ax - hx) / MAX_W, (ay - hy) / MAX_H, min(1.0, max(0.0, best_own / 75.0))


def best_race_delta(state: dict, sn_index: int, cand: Candidate, dists: List[int]) -> int:
    width = state["width"]
    hx, hy = xy(cand.head, width)
    best_own = None
    best_race = None
    for apple in state["apples"]:
        if cand.eating and apple == cand.head:
            continue
        own_dist = dists[apple]
        if own_dist < 0:
            ax, ay = xy(apple, width)
            own_dist = abs(ax - hx) + abs(ay - hy) + state["width"] + state["height"]
        race = own_dist - enemy_apple_dist(state, sn_index, apple)
        if best_own is None or own_dist < best_own or (own_dist == best_own and race < best_race):
            best_own = own_dist
            best_race = race
    return 0 if best_race is None else best_race


def fill_features(state: dict, sn_index: int, cand: Candidate) -> List[float]:
    width = state["width"]
    local_occ = set(cand.body or [])
    features = [0.0] * FEATURE_COUNT
    hx, hy = xy(cand.head, width)
    used = 0
    for ry in range(-CROP_RADIUS, CROP_RADIUS + 1):
        for rx in range(-CROP_RADIUS, CROP_RADIUS + 1):
            ox, oy = rotate_offset(rx, ry, cand.dir)
            tx = hx + ox
            ty = hy + oy
            wall = 0.0
            apple = 0.0
            occ = 0.0
            if not in_bounds(tx, ty, width, state["height"]):
                wall = 1.0
            else:
                cell = idx(tx, ty, width)
                if state["wall"][cell]:
                    wall = 1.0
                if cell in state["apple_set"] and not (cand.eating and cell == cand.head):
                    apple = 1.0
                if cell in local_occ or (state["occupied"][cell] >= 0 and state["occupied"][cell] != sn_index):
                    occ = 1.0
            features[used : used + 3] = [wall, apple, occ]
            used += 3

    my_total = sum(len(sn["body"]) for sn in state["snakes"] if sn["owner"] == 0)
    op_total = sum(len(sn["body"]) for sn in state["snakes"] if sn["owner"] == 1)
    blocked = build_blocked(state, sn_index, cand.body or [])
    cand.flood, dists = flood_dist(cand.head, blocked, width, state["height"])
    cand.safe_moves = count_future_safe_moves(state, sn_index, cand)
    cand.wall_adj, cand.blocked_adj = adjacent_counts(state, sn_index, cand)
    cand.head_on_risk, cand.head_on_win = head_on_signals(state, sn_index, cand)
    cand.race_delta = best_race_delta(state, sn_index, cand, dists)
    target_dx, target_dy, target_dist = best_apple_target(state, sn_index, cand, dists)
    enemy_dist = nearest_enemy_dist(cand.head, sn_index, state)
    features[used:] = [
        state["width"] / MAX_W,
        state["height"] / MAX_H,
        state["turn"] / MAX_TURNS,
        len(state["apples"]) / MAX_AP,
        my_total / 128.0,
        op_total / 128.0,
        len(cand.body or []) / MAX_SEG,
        target_dx,
        target_dy,
        target_dist,
        enemy_dist,
        1.0 if cand.supported else 0.0,
        cand.fall_distance / MAX_H,
        1.0 if cand.eating else 0.0,
        cand.flood / (MAX_W * MAX_H),
        cand.safe_moves / 3.0,
        cand.wall_adj / 4.0,
        1.0 if cand.head_on_risk else 0.0,
        1.0 if cand.head_on_win else 0.0,
        max(-1.0, min(1.0, cand.race_delta / 75.0)),
        cand.blocked_adj / 4.0,
    ]
    return features


def simulate_candidate(state: dict, sn_index: int, dir_idx: int) -> Candidate:
    snake = state["snakes"][sn_index]
    body = snake["body"]
    if not body or body[0] < 0:
        return Candidate(False, dir_idx)
    if len(body) > 1 and dir_idx == OPPOSITE[snake["dir"]]:
        return Candidate(False, dir_idx)

    width = state["width"]
    height = state["height"]
    hx, hy = xy(body[0], width)
    dx, dy = DXY[dir_idx]
    nx, ny = hx + dx, hy + dy
    if not in_bounds(nx, ny, width, height):
        return Candidate(False, dir_idx)
    next_cell = idx(nx, ny, width)
    if state["wall"][next_cell]:
        return Candidate(False, dir_idx)
    occ = state["occupied"][next_cell]
    if occ >= 0 and occ != sn_index:
        return Candidate(False, dir_idx)

    eating = next_cell in state["apple_set"]
    moved = [next_cell]
    if eating:
        moved.extend(body)
    else:
        moved.extend(body[:-1])
    if len(set(moved)) != len(moved):
        return Candidate(False, dir_idx)

    supported = is_supported(moved, sn_index, next_cell if eating else -1, state)
    fall_distance = 0
    while not supported:
        fallen = []
        for cell in moved:
            x, y = xy(cell, width)
            y += 1
            if not in_bounds(x, y, width, height):
                return Candidate(False, dir_idx)
            ncell = idx(x, y, width)
            if state["wall"][ncell]:
                return Candidate(False, dir_idx)
            occ = state["occupied"][ncell]
            if occ >= 0 and occ != sn_index:
                return Candidate(False, dir_idx)
            fallen.append(ncell)
        moved = fallen
        fall_distance += 1
        supported = is_supported(moved, sn_index, next_cell if eating else -1, state)

    cand = Candidate(
        True,
        dir_idx,
        head=moved[0],
        body=moved,
        eating=eating,
        supported=supported,
        fall_distance=fall_distance,
    )
    cand.features = fill_features(state, sn_index, cand)
    return cand


def row_samples(row: dict, command_key: str = "p0_command", teacher_owner: int = 0) -> List[Tuple[List[List[float]], List[bool], int, int]]:
    state = build_state(row, teacher_owner=teacher_owner)
    commands = parse_command_line(row.get(command_key, ""))
    samples = []
    for sn_index, snake in enumerate(state["snakes"]):
        if snake["owner"] != 0 or snake["id"] not in commands:
            continue
        label = commands[snake["id"]]
        cands = [simulate_candidate(state, sn_index, d) for d in range(4)]
        mask = [cand.legal for cand in cands]
        if not mask[label]:
            continue
        feats = [cand.features if cand.features is not None else [0.0] * FEATURE_COUNT for cand in cands]
        samples.append((feats, mask, label, snake["id"]))
    return samples


def teacher_perspective_samples(row: dict) -> List[Tuple[List[List[float]], List[bool], int, int, str]]:
    samples = []
    for feats, mask, label, snake_id in row_samples(row, command_key="p0_command", teacher_owner=0):
        samples.append((feats, mask, label, snake_id, "p0"))
    for feats, mask, label, snake_id in row_samples(row, command_key="p1_command", teacher_owner=1):
        samples.append((feats, mask, label, snake_id, "p1"))
    return samples


def quantize_int4(values: List[float]) -> Tuple[List[int], float]:
    max_abs = max((abs(v) for v in values), default=0.0)
    if max_abs == 0.0:
        return [0] * len(values), 0.0
    scale = max_abs / 7.0
    quantized = []
    for value in values:
        q = int(round(value / scale))
        q = max(-8, min(7, q))
        quantized.append(q)
    return quantized, scale


def pack_int4(values: List[int]) -> bytes:
    padded = list(values)
    if len(padded) % 2 == 1:
        padded.append(0)
    data = bytearray()
    for i in range(0, len(padded), 2):
        lo = padded[i] & 0x0F
        hi = padded[i + 1] & 0x0F
        data.append(lo | (hi << 4))
    return bytes(data)


def nearest_friendly_dist(state: dict, sn_index: int, apple_cell: int) -> float:
    """Manhattan distance from nearest friendly snake (other than sn_index) to apple."""
    width = state["width"]
    ax, ay = xy(apple_cell, width)
    best = 75.0
    for i, snake in enumerate(state["snakes"]):
        if i == sn_index or snake["owner"] != 0 or not snake["body"]:
            continue
        if snake["body"][0] < 0:
            continue
        fx, fy = xy(snake["body"][0], width)
        dist = abs(ax - fx) + abs(ay - fy)
        if dist < best:
            best = dist
    return best


def apple_features_for_snake(
    state: dict, sn_index: int, apple_cell: int, head: int, flood_count: int, dists: List[int]
) -> List[float]:
    """16 features for a (snake, apple) pair."""
    width = state["width"]
    height = state["height"]
    hx, hy = xy(head, width)
    ax, ay = xy(apple_cell, width)

    bfs_d = dists[apple_cell]
    if bfs_d < 0:
        bfs_d = abs(ax - hx) + abs(ay - hy) + width + height
    enemy_d = enemy_apple_dist(state, sn_index, apple_cell)
    race = bfs_d - enemy_d

    my_total = sum(len(sn["body"]) for sn in state["snakes"] if sn["owner"] == 0)
    op_total = sum(len(sn["body"]) for sn in state["snakes"] if sn["owner"] == 1)
    snake_len = len(state["snakes"][sn_index]["body"])
    friendly_d = nearest_friendly_dist(state, sn_index, apple_cell)

    return [
        min(1.0, bfs_d / 75.0),
        min(1.0, enemy_d / 75.0),
        max(-1.0, min(1.0, race / 75.0)),
        (ax - hx) / MAX_W,
        (ay - hy) / MAX_H,
        snake_len / MAX_SEG,
        state["turn"] / MAX_TURNS,
        len(state["apples"]) / MAX_AP,
        my_total / 128.0,
        op_total / 128.0,
        width / MAX_W,
        height / MAX_H,
        1.0 if dists[apple_cell] >= 0 else 0.0,
        flood_count / (MAX_W * MAX_H),
        ay / MAX_H,
        min(1.0, friendly_d / 75.0),
    ]


def emit_go_weights(params: List[List[float]], out_path: Path, schema_version: int = FEATURE_SCHEMA_VERSION) -> dict:
    quantized = []
    scales = []
    for tensor in params:
        q, scale = quantize_int4(tensor)
        quantized.extend(q)
        scales.append(scale)
    blob = base64.b64encode(pack_int4(quantized)).decode("ascii")
    out_path.parent.mkdir(parents=True, exist_ok=True)
    content = [
        "package main",
        "",
        f"const featureSchemaVersion = {schema_version}",
        "",
        "var modelTensorScales = [...]float32{",
    ]
    for scale in scales:
        content.append(f"\t{scale:.9g},")
    content.extend(
        [
            "}",
            "",
            f'const modelBlobBase64 = "{blob}"',
            "",
        ]
    )
    out_path.write_text("\n".join(content), encoding="utf-8")
    return {"scales": scales, "blob_chars": len(blob)}
