# SailInit - Laravel Sail Setup Automator

A Go-based tool to automate the initialization of Laravel Sail projects with intelligent port management and environment configuration.

## Features

- **Automated Dependency Install**: Runs Composer via Docker (no local PHP needed).
- **Collision-Free Ports**: Automatically allocates unique ports for each project.
- **Interactive Suffix Selection**: Suggests the next available port suffix and allows manual overrides.
- **Port Conflict Detection**: Prevents assigning the same port suffix to multiple projects.
- **Clean .env Formatting**: Groups all port settings at the end of the file with proper spacing.
- **One-Step Startup**: Automatically runs `sail up -d` after configuration.

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

### From Binary (GitLab Release)
Download the `sailinit` binary from your GitLab project's **Releases** page. Binaries are automatically built for:
- Linux (`sailinit-linux`)
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
| `--list` | List all registered projects with their port suffixes |
| `--clean` | Remove entries for project directories that no longer exist |
| `--fresh` | Force re-run composer install even if `vendor/bin/sail` exists |
| `--reset-db` | Reset database settings to Sail defaults (mysql, laravel, sail/password) |

### Arguments

- **php_version**: Optional (e.g., `81`, `82`, `83`, `84`).
    - If omitted, the tool will scan `compose.yaml` or `docker-compose.yaml` to detect the version.
    - If detection fails, it defaults to `84`.
    - If you provide a version that differs from the detected one, the tool will warn you.

### Examples

```bash
# Auto-detects PHP version
sailinit

# Manually specifies version (warns if different from compose file)
sailinit 82

# List all registered projects
sailinit --list

# Clean up orphaned projects (directories that no longer exist)
sailinit --clean

# Force reinstall dependencies even if sail already exists
sailinit --fresh

# Reset database settings to Sail defaults (useful when DB credentials are out of sync)
sailinit --reset-db
```

### Project List Output

When using `--list`, projects are displayed with their path, suffix, and app port:

```
     Project                              Suffix   App Port
     /Users/user/projects/blog            51       8051
     /Users/user/projects/shop            52       8052
[X]  /Users/user/deleted-project          49       8049
```

Projects marked with `[X]` no longer exist on disk and can be removed with `--clean`.

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

### First-Time Setup
On the very first run (when the state file doesn't exist), the tool will detect this and **prompt you to enter a starting suffix** (defaults to `48`). This suffix will be used for your current project, and subsequent projects will automatically increment from the highest suffix used.

### Ongoing Tracking
The tool tracks:
- The maximum suffix used so far.
- A mapping of project directories to their assigned suffixes.

Ports are calculated as:
- **APP_PORT**: `8000 + suffix`
- **FORWARD_DB_PORT**: `3300 + suffix`
- **FORWARD_REDIS_PORT**: `6300 + suffix`
- **VITE_PORT**: `5100 + suffix`
- **FORWARD_MAILPIT_DASHBOARD_PORT**: `18100 + suffix`
- ...and others.

This ensures that even with hundreds of projects, you won't have conflicting ports on your local machine.
