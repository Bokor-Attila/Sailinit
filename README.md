# SailInit - Laravel Sail Setup Automator

A Go-based tool to automate the initialization of Laravel Sail projects with intelligent port management and environment configuration.

## Features

- **Automated Dependency Install**: Runs Composer via Docker (no local PHP needed).
- **Collision-Free Ports**: Automatically allocates unique ports for each project.
- **Interactive Suffix Selection**: Suggests the next available port suffix and allows manual overrides.
- **Port Conflict Detection**: Prevents assigning the same port suffix to multiple projects.
- **Port Availability Check**: Warns if OS-level ports are already in use before starting.
- **Port Suffix Validation**: Ensures suffixes stay within valid TCP port range (0-47435).
- **Clean .env Formatting**: Groups all port settings at the end of the file with proper spacing.
- **One-Step Startup**: Automatically runs `sail up -d` after configuration.
- **Colored Output**: ANSI-colored terminal output with `NO_COLOR` support.
- **Dry-Run Mode**: Preview what would happen without making any changes.
- **Sail Lifecycle**: Stop, bring down, and check status of Sail containers.

## Installation

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
- Linux (`sailinit-linux-amd64`)
- macOS Intel (`sailinit-macos-amd64`)
- macOS Apple Silicon (`sailinit-macos-arm64`)

After downloading, move it to your path:
```bash
chmod +x sailinit-macos-arm64
# Remove quarantine attribute (macOS only)
xattr -d com.apple.quarantine sailinit-macos-arm64
sudo mv sailinit-macos-arm64 /usr/local/bin/sailinit
```

## Usage

Run the following command in your Laravel project root:

```bash
sailinit [flags] [php_version]
```

### Flags

| Flag | Description |
|------|-------------|
| `--version` | Print version and exit |
| `--list` | List all registered projects with port details and status |
| `--status` | Show all projects with container running status |
| `--clean` | Remove entries for project directories that no longer exist |
| `--remove` | Remove the current project from the port registry |
| `--stop` | Run `sail stop` in the current project |
| `--down` | Run `sail down` in the current project |
| `--fresh` | Force re-run composer install even if `vendor/bin/sail` exists |
| `--reset-db` | Reset database settings to Sail defaults (mysql, laravel, sail/password) |
| `--dry-run` | Show what would happen without making changes |

### Arguments

- **php_version**: Optional (e.g., `81`, `82`, `83`, `84`).
    - If omitted, the tool will scan `compose.yaml` or `docker-compose.yaml` to detect the version.
    - If detection fails, it defaults to `84`.
    - If you provide a version that differs from the detected one, the tool will warn you.

### Examples

```bash
# Auto-detects PHP version
sailinit

# Print version
sailinit --version

# Manually specifies version (warns if different from compose file)
sailinit 82

# List all registered projects with detailed port info
sailinit --list

# Show all projects with container status
sailinit --status

# Clean up orphaned projects (directories that no longer exist)
sailinit --clean

# Remove the current project from port registry
sailinit --remove

# Stop containers in the current project
sailinit --stop

# Bring down containers in the current project
sailinit --down

# Force reinstall dependencies even if sail already exists
sailinit --fresh

# Reset database settings to Sail defaults (useful when DB credentials are out of sync)
sailinit --reset-db

# Preview what would happen without making any changes
sailinit --dry-run
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
The tool maintains a state file at `~/.laravel-sail-ports.json`.

### Port Suffix Validation
Suffixes must be between 0 and 47435 to ensure all calculated ports stay within the valid TCP port range (max 65535). The highest base port is 18100 (Mailpit Dashboard), so `18100 + 47435 = 65535`.

### First-Time Setup
On the very first run (when the state file doesn't exist), the tool will detect this and **prompt you to enter a starting suffix** (defaults to `48`). This suffix will be used for your current project, and subsequent projects will automatically increment from the highest suffix used.

### Port Availability Check
After confirming a suffix, the tool checks whether the OS-level ports are already in use. If any ports are busy, you'll see a warning listing the occupied ports and can choose to continue or abort.

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

This ensures that even with hundreds of projects, you won't have conflicting ports on your local machine.
