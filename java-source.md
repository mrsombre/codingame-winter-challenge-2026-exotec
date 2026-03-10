# Java Source → Go Migration Guide

## Goal

The repository is split into three layers:

- `cmd/match` — tiny CLI wrapper
- `internal/engine` — Java engine parity port
- `internal/match` — local binary-vs-binary execution and stats

## Mapping

Map the Java referee package directly into `internal/engine`.

| Java source | Go target |
|---|---|
| `source/src/main/java/com/codingame/game/Bird.java` | `internal/engine/bird.go` |
| `source/src/main/java/com/codingame/game/CommandManager.java` | `internal/engine/command_manager.go` |
| `source/src/main/java/com/codingame/game/Game.java` | `internal/engine/game.go` |
| `source/src/main/java/com/codingame/game/Player.java` | `internal/engine/player.go` |
| `source/src/main/java/com/codingame/game/Referee.java` | `internal/engine/referee.go` |
| `source/src/main/java/com/codingame/game/Serializer.java` | `internal/engine/serializer.go` |
| `source/src/main/java/com/codingame/game/action/*.java` | `internal/engine/*` |
| `source/src/main/java/com/codingame/game/grid/*.java` | `internal/engine/*` |

## Rules

- Keep `internal/engine` focused on the Java-port engine types and rules.
- Keep `internal/match` focused on external bot execution, timeouts, batch runs, and summary output.
- Keep `cmd/match` minimal; it should delegate to `internal/match`.
- Keep Java file parity naming: `CamelCase.java` becomes `snake_case.go`.
