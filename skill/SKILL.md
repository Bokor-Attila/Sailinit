---
name: sailinit
description: Set up, inspect and manage Laravel Sail projects with the sailinit CLI — collision-free port allocation per project, .env port blocks, container status, diagnostics. Use when working in a Laravel Sail project, when ports clash between Sail projects, when asked which port a project runs on, or when creating a new Laravel project with Sail.
---

# sailinit

`sailinit` gives every Laravel Sail project on this machine a unique port
suffix, writes the matching port block into `.env`, installs Composer
dependencies via Docker and runs `sail up -d`. A registry maps each project
directory to its suffix, so ports never collide.

## Rules for driving it

- **Always pass `-y`** (`--yes`) when a command may prompt. Without a terminal
  and without `-y`, sailinit refuses to guess and exits `3`.
- **Add `-j`** (`--json`) for `--list`, `--status`, `--ports` and `--doctor`
  and parse stdout. stdout carries data only; progress, warnings and errors go
  to stderr, so JSON on stdout is never mixed with messages.
- **Branch on the exit code**, not on stderr text:

  | Code | Meaning |
  |---|---|
  | 0 | Done |
  | 1 | Runtime failure (Docker down, command failed, unreadable registry) |
  | 2 | Bad invocation (unknown flag, bad argument) |
  | 3 | Aborted: a prompt was declined, or no TTY and no `-y` |
  | 4 | `--doctor` ran and at least one check FAILed |

- **Read ports from sailinit, never compute them.** Base ports are configurable
  (`config.json`, `SAILINIT_BASE_*_PORT`), so `8000 + suffix` is wrong on some
  machines. Use `--port` for `APP_PORT` or `--ports -j` for all of them.
- **Preview with `--dry-run`** (`-d`) before anything that edits the registry,
  `.env` or containers.
- **Sandbox with `SAILINIT_HOME=<dir>`** to experiment without touching the
  user's real registry and config.

## Ask the user before running

These change state the user may care about. Confirm first, and show the
`--dry-run` output where one exists:

- `--down` — `sail down`, removes the project's containers.
- `--stop` — stops the project's containers.
- `--remove` — drops the current project from the registry; it gets a new
  suffix, and therefore new ports, on the next setup.
- `--clean` — drops every registry entry whose directory no longer exists.
- `--reset-db` — overwrites the `DB_*` settings in `.env` with Sail defaults.
- `--fresh` — re-runs `composer install` even though `vendor/bin/sail` exists.
- `--new <name>` — creates a new Laravel project by running the
  `laravel.build` installer script; `--with` picks the Sail services
  (default `mysql`).
- `--upgrade` — replaces the sailinit binary with the latest release.
- `--uninstall-skill` — removes this skill.

## Read-only commands

| Command | Output |
|---|---|
| `sailinit --version` | Version string |
| `sailinit --port` | `APP_PORT` of the current project |
| `sailinit --ports -j` | `{"path", "suffix", "ports": {"APP_PORT": ..., ...}}` |
| `sailinit --list -j` | Every registered project with ports and container state |
| `sailinit --status -j` | Every project with `app_port` and container state |
| `sailinit --doctor -j` | `{"healthy": bool, "diagnostics": [{"name", "status", "detail", "fix"}]}` |
| `sailinit --upgrade --dry-run` | Whether a newer release exists |

`containers` in `--list -j` and `--status -j` is `{"state", "running"}`, where
`state` is one of `running`, `stopped`, `no sail`, `unknown`, `missing`.

For an unregistered project, `--port` and `--ports` report the suffix the next
setup *would* assign — a projection, not an allocation.

`--open` opens the project URL in the browser; with `--dry-run` it only prints
the URL.

## Common tasks

**Set up the current project** (from its root):

```bash
sailinit -y          # PHP version detected from compose.yaml, default 84
sailinit -y 83       # force PHP 8.3
```

**Find what's wrong** — run `sailinit --doctor -j`, then apply each failing
check's `fix` (after confirming with the user if it is in the list above).

**Port already in use** — `sailinit --doctor -j` names the busy ports. Either
stop whatever holds them, or reassign the project: `sailinit --remove`, then
`sailinit -y`.

**`.env` disagrees with the registry** — `sailinit -y` in the project rewrites
the port block from the registry. Other `.env` values are kept; `DB_*` is only
touched on first creation or with `--reset-db`.

## Maintaining this skill

`sailinit --install-skill` wrote this file, and `sailinit --upgrade` refreshes
it to match the new version. `sailinit --refresh-skill` does the same by hand.
Local edits are detected and never overwritten. `sailinit --doctor` reports
whether the skill is current.
