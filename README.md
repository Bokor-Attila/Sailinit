# SailInit - Laravel Sail Setup Automator

A Go-based tool to automate the initialization of Laravel Sail projects with intelligent port management and environment configuration.

## Features

- **New Project Creation**: Create a new Laravel project from scratch with `--new`.
- **Automated Dependency Install**: Runs Composer via Docker (no local PHP needed).
- **Collision-Free Ports**: Automatically allocates unique ports for each project.
- **Interactive Suffix Selection**: Suggests the next available port suffix and allows manual overrides.
- **Port Conflict Detection**: Prevents assigning the same port suffix to multiple projects.
- **Port Availability Check**: Warns if OS-level ports are already in use before starting.
- **Port Suffix Validation**: Ensures suffixes stay within valid TCP port range (0-47435).
- **Clean .env Formatting**: Groups all port settings at the end of the file with proper spacing.
- **One-Step Startup**: Automatically runs `sail up -d` after configuration.
- **Colored Output**: ANSI-colored terminal output with `NO_COLOR` support.
- **Dry-Run Mode**: Preview what would happen without making any changes, including registry edits.
- **Sail Lifecycle**: Stop, bring down, and check status of Sail containers.
- **Self-Update**: Upgrade to the latest release in place with `--upgrade`, with checksum verification.
- **Diagnostics**: `--doctor` checks the registry, duplicate suffixes, `.env` drift, and port availability.
- **Quick Open**: `--open` launches the current project's URL in your browser.
- **Scriptable Output**: data on stdout, messages on stderr, and distinct exit codes per outcome.
- **Isolated State**: point `SAILINIT_HOME` at any directory to sandbox the registry and config.

## Breaking Changes

Three behaviours changed to make `sailinit` safe to drive from scripts and
agents. Each is a deliberate break:

1. **`--doctor` exits `4`** (was `1`) when a check FAILs. Gates written as
   `sailinit --doctor || handle_failure` still work; anything comparing the code
   to `1` needs updating.
2. **No terminal and no `-y` is now an error**, exit `3`. Previously a piped or
   detached run silently took the defaults, or exited `0` having done nothing.
   Add `-y` to unattended invocations.
3. **Messages moved to stderr.** Progress, warnings and errors no longer appear
   on stdout; stdout carries data only.
4. **`containers` in `--list -j` is now an object**, `{"state": ..., "running": N}`,
   where `state` is one of `running`, `stopped`, `no sail`, `unknown`, `missing`.
   It used to be a rendered string such as `"3 running"` — and carried ANSI
   colour codes when stdout was a terminal.

## Installation

### Quick Install (macOS / Linux)
```bash
curl -fsSL https://raw.githubusercontent.com/Bokor-Attila/Sailinit/main/install.sh | sh
```

Downloads the latest release for your platform, verifies it against
`sha256sums.txt`, and installs it to `/usr/local/bin` (prompting for sudo only if
that directory isn't writable). Set `SAILINIT_INSTALL_DIR` to install elsewhere:
```bash
curl -fsSL https://raw.githubusercontent.com/Bokor-Attila/Sailinit/main/install.sh | SAILINIT_INSTALL_DIR="$HOME/.local/bin" sh
```

### From Source
1. Clone the repository.
2. Build the binary:
   ```bash
   go build -o sailinit .
   ```
3. Move to your bin directory:
   ```bash
   sudo mv sailinit /usr/local/bin/sailinit
   ```

### From Binary (GitHub Release)
Download the `sailinit` binary from your GitHub project's **Releases** page. Binaries are automatically built for:
- Linux x86_64 (`sailinit-linux-amd64`)
- Linux ARM64 (`sailinit-linux-arm64`)
- macOS Intel (`sailinit-macos-amd64`)
- macOS Apple Silicon (`sailinit-macos-arm64`)

Each release also publishes `sha256sums.txt` for verification.

After downloading, move it to your path:
```bash
chmod +x sailinit-macos-arm64
# Remove quarantine attribute (macOS only)
xattr -d com.apple.quarantine sailinit-macos-arm64
sudo mv sailinit-macos-arm64 /usr/local/bin/sailinit
```

The `xattr` step is only needed for this first manual install. Later upgrades via
`sailinit --upgrade` download the binary directly and are never quarantined.

## Usage

Run the following command in your Laravel project root:

```bash
sailinit [flags] [php_version]
```

### Flags

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--version` | `-v` | Print version and exit |
| `--list` | `-l` | List all registered projects with port details and status |
| `--status` | `-s` | Show all projects with container running status |
| `--clean` | `-c` | Remove entries for project directories that no longer exist |
| `--remove` | `-r` | Remove the current project from the port registry |
| `--stop` | | Run `sail stop` in the current project |
| `--down` | | Run `sail down` in the current project |
| `--fresh` | `-f` | Force re-run composer install even if `vendor/bin/sail` exists |
| `--reset-db` | | Reset database settings to Sail defaults (mysql, laravel, sail/password) |
| `--new <name>`| `-n <name>` | Create a new Laravel project and set it up with Sail |
| `--with <svcs>`| `-w <svcs>` | Services to include for new project (default: `mysql`, e.g. `mysql,redis,mailpit`) |
| `--json` | `-j` | Output registered projects in structured JSON format |
| `--port` | `-p` | Print calculated `APP_PORT` for current project and exit |
| `--ports` | | Print every calculated port for the current project and exit |
| `--yes` | `-y` | Automatic yes to prompts; assume non-interactive mode |
| `--dry-run` | `-d` | Show what would happen without making changes |
| `--open` | `-o` | Open the current project's URL in the default browser |
| `--doctor` | | Run diagnostics on the port registry and the current project |
| `--upgrade` | `-u` | Download and install the latest release over the running binary |
| `--completion <shell>` | | Generate shell completion script (`bash`, `zsh`, `fish`) |

### Arguments

- **php_version**: Optional (e.g., `81`, `82`, `83`, `84`).
    - If omitted, the tool will scan `compose.yaml` or `docker-compose.yaml` to detect the version.
    - If detection fails, it defaults to `84`.
    - If you provide a version that differs from the detected one, the tool will warn you.

### Examples

```bash
# Create a brand new Laravel project with Sail + MySQL
sailinit -n my-blog

# Create a new project with custom Sail services (MySQL + Redis + Mailpit)
sailinit -n my-app -w mysql,redis,mailpit

# Run headlessly / non-interactively
sailinit -y

# Print just the assigned APP_PORT for current directory (useful in scripts)
sailinit -p

# Print every port for the current project
sailinit --ports

# ...as JSON
sailinit --ports -j

# Export all registered projects as JSON
sailinit -l -j

# Auto-detects PHP version (run inside an existing project)
sailinit

# Print version
sailinit -v

# List all registered projects with detailed port info
sailinit -l

# Show all projects with container status
sailinit -s

# Clean up orphaned projects (directories that no longer exist)
sailinit -c

# Remove the current project from port registry
sailinit -r

# Stop containers in the current project
sailinit --stop

# Bring down containers in the current project
sailinit --down

# Force reinstall dependencies even if sail already exists
sailinit -f

# Preview what would happen without making any changes
sailinit -d

# Preview registry edits without writing them
sailinit -c -d
sailinit -r -d

# Container status as JSON
sailinit -s -j

# Open the current project in your browser
sailinit -o

# Diagnose registry and project problems
sailinit --doctor

# Machine-readable diagnostics
sailinit --doctor -j

# Upgrade to the latest release
sailinit -u

# Check for a newer release without installing it
sailinit -u -d

# Generate zsh completion script
eval "$(sailinit --completion zsh)"
```

### Output Streams and Exit Codes

`sailinit` is built to be driven by scripts, CI jobs and coding agents, so the
two output streams have fixed jobs:

| Stream | Carries |
|---|---|
| **stdout** | data only: JSON payloads, the `--port`/`--ports` values, the `--list`/`--status` tables, the `--doctor` report, completion scripts |
| **stderr** | everything human-facing: progress, warnings, errors, prompts |

`sailinit --list -j | jq` is therefore safe even when something goes wrong:
the error goes to stderr and never lands inside the JSON.

Exit codes:

| Code | Name | Meaning |
|---|---|---|
| `0` | OK | The requested work finished |
| `1` | Error | A runtime failure: Docker unreachable, a command returned non-zero, the registry was unreadable |
| `2` | Usage | The invocation was wrong: unknown flag, bad argument, unsupported `--completion` shell |
| `3` | Aborted | Nothing was done because a human was needed: a prompt was declined, or there was no terminal to ask on and `--yes` was not given |
| `4` | Unhealthy | `--doctor` ran fine but at least one check FAILed |

### Non-Interactive Use

When `sailinit` needs to ask something but stdin is not a terminal, it refuses
to guess. It explains what it was about to ask and exits `3`:

```
$ echo | sailinit
sailinit: no TTY and -y not given
  needs confirmation: ports 8051, 3351 already in use
  re-run with -y to accept, or free the ports
$ echo $?
3
```

Pass `-y` to accept the defaults and run unattended:

```bash
sailinit -y
```

This applies to the first-ever run too, where the starting suffix would
otherwise be prompted for, so a scripted first install is `sailinit -y`.

### Isolating State with `SAILINIT_HOME`

Set `SAILINIT_HOME` to keep the registry and the global config in a directory of
your choosing:

```bash
SAILINIT_HOME=/tmp/sailinit-sandbox sailinit --ports
```

It holds `ports.json` and `config.json`, is created if missing, and takes
precedence over every other location including the legacy
`~/.laravel-sail-ports.json` — so a sandboxed run can neither read nor write
your real registry. Useful for tests, CI, throwaway experiments, and for letting
an agent inspect port allocation without touching your machine's state.

### Reading Ports

```bash
sailinit --ports
```

Prints every port for the current project as `KEY=VALUE` lines, sorted, ready to
`source`:

```
APP_PORT=8051
FORWARD_DB_PORT=3351
FORWARD_MAILPIT_DASHBOARD_PORT=18151
...
```

With `-j` it emits `{"path": ..., "suffix": ..., "ports": {...}}`, where `ports`
has the same shape as in `--list -j`.

Prefer this over computing `8000 + suffix` yourself: the base ports are
configurable, so a hardcoded offset is wrong on any machine with a custom
`config.json` or `SAILINIT_BASE_*` environment variable.

For a project that is not registered yet, the reported suffix is the one that
*would* be assigned on the next run — a projection, not a commitment.

### Safety Checks

Before running setup in the current directory, `sailinit` checks for the presence of `composer.json` or `artisan`. If neither file is found, it warns you:
```
Warning: No Laravel project files (composer.json or artisan) detected in current directory.
Continue anyway? [y/N]:
```
In non-interactive mode (`-y`), setup continues automatically.

### Diagnostics

```bash
sailinit --doctor
```

Runs a read-only health check and prints each result with a suggested fix. It
never changes anything itself.

```
sailinit doctor
[ OK ] Docker: daemon is running
[ OK ] Registry: 3 project(s) tracked in /Users/you/.config/sailinit/ports.json
[FAIL] Duplicate suffixes: suffix 51: /Users/you/blog, /Users/you/shop
       fix: run sailinit --remove in one project, then sailinit to reassign it
[WARN] Orphaned projects: 1 registered director(ies) no longer exist: /Users/you/old
       fix: run sailinit --clean
[ OK ] Current project: registered with suffix 52 (APP_PORT 8052)
[ OK ] .env ports: .env matches the registry (APP_PORT 8052)
[ OK ] Port availability: all forwarded ports are free
```

What it checks:

| Check | Severity | Meaning |
|---|---|---|
| Docker daemon | WARN | Docker Desktop or the Docker service is not reachable |
| Registry readable | FAIL | The state file is corrupt or unreadable |
| Registry max suffix | FAIL | `max_suffix` is below an allocated suffix, so the next project would collide |
| Duplicate suffixes | FAIL | Two projects share a suffix, and therefore every forwarded port |
| Orphaned projects | WARN | A registered directory no longer exists |
| Current project | WARN | This Laravel project is not registered yet |
| `.env` ports | FAIL | `.env` and the registry disagree about `APP_PORT` |
| Port availability | WARN | Something already holds one of this project's ports |

**Exit code** is `4` if any check FAILs and `0` otherwise, so `--doctor` works as
a CI or pre-flight gate. `4` is distinct from `1` so a gate can tell "doctor
found problems" from "doctor itself broke". WARN does not affect the exit code.
Add `-j` for JSON.

Checks that depend on the current directory are skipped when you are not inside
a Laravel project, so `--doctor` is safe to run anywhere.

### Opening a Project

```bash
sailinit --open
```

Opens `http://localhost:<APP_PORT>` for the current project using `open` on
macOS or `xdg-open` on Linux. If the containers are not running it warns and
opens the tab anyway, so the page is ready as `sail up` finishes. When no opener
is available the URL is printed instead of failing. `--dry-run` prints the URL
without opening it.

### Upgrading

```bash
sailinit --upgrade
```

This checks the latest GitHub release, and if it is newer than the running
binary it downloads the asset for your platform, verifies its SHA-256 against
the release's `sha256sums.txt`, runs it once to confirm it works on your
machine, then atomically swaps it into place. The previous binary is kept aside
until the swap succeeds and restored if anything fails.

Pair it with `--dry-run` to check for a new version without installing:

```bash
sailinit --upgrade --dry-run
```

The download is aborted if the release does not publish `sha256sums.txt`, or if
it lists no entry for your platform's binary.

Notes:
- `--upgrade` was introduced in v1.5.0. On v1.4.0 or earlier, upgrade manually
  once using the instructions above; `--upgrade` works from then on.
- Upgrading needs write access to the directory holding the binary. If it is
  root-owned (common for `/usr/local/bin` on Linux), re-run with `sudo`.
- Development builds (`sailinit --version` prints `dev`) refuse to self-update,
  since that would replace your local build with a release. Pass `--yes` to
  override.
- Symlinked installs are resolved first, so the real binary is replaced rather
  than the symlink.

### Custom Base Port Offsets

By default, ports are calculated using default base offsets (`APP_PORT = 8000 + suffix`, etc.). You can customize the base ports globally by creating `~/.config/sailinit/config.json` (or `$SAILINIT_HOME/config.json`):

```json
{
  "base_app_port": 8000,
  "base_db_port": 3300,
  "base_redis_port": 6300,
  "base_meilisearch_port": 7700,
  "base_mailpit_dashboard_port": 18100,
  "base_mailpit_port": 1000,
  "base_vite_port": 5100
}
```

Or override individual base ports using environment variables:
```bash
export SAILINIT_BASE_APP_PORT=9000
export SAILINIT_BASE_DB_PORT=4300
```

### Project List Output

When using `--list`, projects are displayed in a formatted table:

```
Project                                   Suffix  App Port  DB Port  Redis Port  Vite Port  Status
/Users/user/projects/blog                 51      8051      3351     6351        5151       OK
/Users/user/projects/shop                 52      8052      3352     6352        5152       OK
/Users/user/deleted-project               49      8049      3349     6349        5149       [X] Missing
```

Projects marked with `[X] Missing` no longer exist on disk and can be removed with `--clean`.

### Status Output

When using `--status`, container status is checked for each project:

```
Project                                   Suffix  App Port  Containers
/Users/user/projects/blog                 51      8051      3 running
/Users/user/projects/shop                 52      8052      stopped
```

Add `-j` for the machine-readable form:

```json
[
  {
    "path": "/Users/user/projects/blog",
    "suffix": 51,
    "exists": true,
    "app_port": 8051,
    "containers": { "state": "running", "running": 3 }
  }
]
```

`state` is one of `running`, `stopped`, `no sail` (dependencies not installed),
`unknown` (the `sail ps` call failed) or `missing` (the directory is gone). An
empty registry yields `[]` rather than a message, so the output is always
parseable.

### Previewing Registry Changes

`--clean` and `--remove` both edit the port registry, and both honour
`--dry-run`:

```bash
sailinit -c -d   # list the orphaned entries that would be dropped
sailinit -r -d   # show what removing this project would do
```

Nothing is written, and the preview lists exactly what the real run would
touch.

## Creating New Projects

The `--new` (`-n`) flag creates a brand new Laravel project from scratch using [Laravel's build service](https://laravel.build):

```bash
sailinit -n my-blog -w mysql,redis,mailpit
```

This runs the following steps automatically:
1. Verifies Docker daemon is running
2. Downloads and creates the project via `curl -s "https://laravel.build/my-blog?with=mysql,redis,mailpit" | bash`
3. Stops default containers started by Laravel's installer
4. Assigns a unique port suffix (with optional interactive prompt)
5. Configures `.env` with collision-free ports
6. Starts the project with `sail up -d`

The only prerequisite is Docker — no local PHP or Composer needed.

## Colored Output

SailInit uses ANSI colors for better readability:
- **Green**: Success messages
- **Yellow**: Warnings
- **Red**: Errors
- **Cyan**: Informational messages

To disable colors, set the `NO_COLOR` environment variable:
```bash
NO_COLOR=1 sailinit --list
```

## Database Settings Handling

The tool uses smart database configuration to avoid breaking existing projects:

| Scenario | Database Settings |
|----------|-------------------|
| `.env` doesn't exist (created from example) | Set to Sail defaults |
| `.env` already exists | **Left unchanged** |
| `--reset-db` flag used | Force overwrite to Sail defaults |

**Sail defaults**: `DB_CONNECTION=mysql`, `DB_HOST=mysql`, `DB_DATABASE=laravel`, `DB_USERNAME=sail`, `DB_PASSWORD=password`

This prevents issues where custom database names get overwritten and then fail to authenticate because Docker/MySQL volumes retain the original credentials.

## How Port Management Works
The tool maintains a state JSON file:
- **`SAILINIT_HOME`**: `$SAILINIT_HOME/ports.json` when that variable is set; it wins over both locations below.
- **Legacy location**: `~/.laravel-sail-ports.json` (auto-detected if present for backwards compatibility).
- **New location**: `~/.config/sailinit/ports.json` (used for new installations).

### Port Suffix Validation
Suffixes must be between 0 and 47435 to ensure all calculated ports stay within the valid TCP port range (max 65535). The highest base port is 18100 (Mailpit Dashboard), so `18100 + 47435 = 65535`.

### First-Time Setup
On the very first run (when the state file doesn't exist), the tool will detect this and prompt you to enter a starting suffix (defaults to `48`). In non-interactive mode (`-y`), it defaults to `48` automatically.

### Port Availability Check
After confirming a suffix, the tool checks whether the OS-level ports are already in use. If any ports are busy, you'll see a warning listing the occupied ports.

### Ongoing Tracking
The tool tracks:
- The maximum suffix used so far.
- A mapping of project directories to their assigned suffixes.

Ports are calculated as:
- **APP_PORT**: `8000 + suffix`
- **FORWARD_DB_PORT**: `3300 + suffix`
- **FORWARD_REDIS_PORT**: `6300 + suffix`
- **FORWARD_MEILISEARCH_PORT**: `7700 + suffix`
- **FORWARD_MAILPIT_DASHBOARD_PORT**: `18100 + suffix`
- **FORWARD_MAILPIT_PORT**: `1000 + suffix`
- **VITE_PORT**: `5100 + suffix`
- **FORWARD_MINIO_PORT**: `9000 + suffix`
- **FORWARD_MINIO_CONSOLE_PORT**: `8900 + suffix`
- **FORWARD_TYPESENSE_PORT**: `8108 + suffix`
- **FORWARD_SOKETI_PORT**: `6001 + suffix`
- **FORWARD_SELENIUM_PORT**: `4444 + suffix`

This ensures that even with hundreds of projects, you won't have conflicting ports on your local machine.

## License

Released under the [MIT License](LICENSE). Copyright (c) 2026 Attila Bokor.
