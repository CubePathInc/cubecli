# CubeCLI

The official command-line interface for [CubePath Cloud](https://cubepath.com).

Built in Go — single binary, no dependencies, blazing fast.

## Installation

### Quick install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/CubePathInc/cubecli/main/install.sh | sh
```

### Manual download

Download the latest release for your platform from the [Releases](https://github.com/CubePathInc/cubecli/releases) page.

### From source

```bash
go install github.com/CubePathInc/cubecli@latest
```

### Self-update

Once installed, CubeCLI can update itself:

```bash
cubecli update
```

## Quick Start

```bash
# Log in with your browser
cubecli login

# List your projects
cubecli project list

# List VPS instances
cubecli vps list

# Get help on any command
cubecli <command> --help
```

## Authentication

`cubecli login` opens your browser, you approve the access on the CubePath
dashboard (choosing the organization and the permissions), and the session is
stored in a profile. The access token is refreshed automatically; you stay
logged in as long as you use the CLI at least once every 30 days.

```bash
# Log in the active profile ('default' if there is none)
cubecli login

# Log in a named profile
cubecli login work

# On a machine without a browser (e.g. over SSH): prints the URL instead
cubecli login --no-browser

# See how every profile is authenticated
cubecli auth status

# Revoke the session and remove the stored credentials
cubecli logout work
cubecli logout --all
```

The consent screen only pre-selects read permissions. Tick the write
permissions you need there, or the CLI can list resources but not create or
change them.

Browser sessions can be disconnected at any time from the dashboard, under
Account > Connected apps (https://my.cubepath.com/account/connections).

### API tokens (CI and scripts)

For non-interactive use, create an API token in the dashboard and either store
it in a profile or pass it through the environment:

```bash
# Store a token in a profile (prompts for it and validates it)
cubecli login ci --token

# Or use an environment variable, which overrides every profile
export CUBE_API_TOKEN="your-api-token"

# Optionally override the API URL
export CUBE_API_URL="https://api.cubepath.com"
```

`cubecli profile add` and `cubecli config setup` still read a token from stdin
when it is piped, so existing scripts keep working.

CubeCLI reads credentials from (in order):

1. `CUBE_API_TOKEN` environment variable
2. Active profile in `~/.cubecli/config.json`

## AI agent skills

The [CubePath skills](https://github.com/CubePathInc/skills) teach AI coding
agents (Claude Code, Codex, Gemini CLI, Cursor) to manage CubePath with
cubecli. After your first `cubecli login`, CubeCLI offers to install them for
the agents it finds on your machine (skip with `--skip-skills`).

```bash
cubecli skills install                  # detected agents
cubecli skills install --agent claude   # ~/.claude/skills (Claude Code)
cubecli skills install --agent agents   # ~/.agents/skills (Codex, Gemini CLI, Cursor)
cubecli skills install --project        # ./.claude/skills or ./.agents/skills, to commit with a repo
cubecli skills list                     # installed versions, updates available
cubecli skills update
cubecli skills uninstall
```

Downloads are checked against the release's SHA256SUMS. CubeCLI only touches
skill folders it installed itself, and leaves alone any you have edited unless
you pass `--force`. In Claude Code you can install them as a plugin instead:
`/plugin marketplace add CubePathInc/skills`.

## Profiles (multiple accounts and organizations)

Each profile holds the credentials of one organization. Log in once per
organization and switch between them.

```bash
# Log in a new profile (choose the organization on the consent screen)
cubecli login work
cubecli login personal

# Point a profile at a different API URL (e.g. staging)
cubecli login staging --api-url https://api.staging.cubepath.com

# Log in and make it the active profile
cubecli login work --use

# List configured profiles (active marked with *)
cubecli profile list

# Switch the active profile
cubecli profile use work

# Show the active profile
cubecli profile current

# Override per invocation without changing the active profile
cubecli --profile personal vps list
CUBE_PROFILE=personal cubecli vps list

# Remove a profile
cubecli profile delete work

# Rename a profile
cubecli profile rename work corp
```

Legacy configs with a single `api_token` field are migrated automatically into
a profile called `default` the first time you run CubeCLI.

## Commands

### Compute

| Command | Description |
|---------|-------------|
| `cubecli vps create` | Create a new VPS instance |
| `cubecli vps list` | List all VPS instances |
| `cubecli vps show <id>` | Show VPS details |
| `cubecli vps destroy <id>` | Destroy a VPS instance |
| `cubecli vps power start\|stop\|restart\|reset <id>` | Power management |
| `cubecli vps resize <id>` | Resize a VPS |
| `cubecli vps change-password <id>` | Change root password |
| `cubecli vps reinstall <id>` | Reinstall OS |
| `cubecli vps backup list\|create\|restore\|delete <id>` | Backup management |
| `cubecli vps backup settings\|configure <id>` | Auto-backup settings |
| `cubecli vps iso list\|mount\|unmount <id>` | ISO management |
| `cubecli vps plan list` | List available plans |
| `cubecli vps template list` | List OS templates |

### Baremetal

| Command | Description |
|---------|-------------|
| `cubecli baremetal deploy` | Deploy a new baremetal server |
| `cubecli baremetal list` | List all baremetal servers |
| `cubecli baremetal show <id>` | Show server details |
| `cubecli baremetal sensors <id>` | Show BMC sensor data |
| `cubecli baremetal power start\|stop\|restart <id>` | Power management |
| `cubecli baremetal reinstall start\|status <id>` | OS reinstallation |
| `cubecli baremetal monitoring enable\|disable\|status <id>` | Monitoring |
| `cubecli baremetal rescue <id>` | Boot into rescue mode |
| `cubecli baremetal reset-bmc <id>` | Reset the BMC |
| `cubecli baremetal ipmi <id>` | Create IPMI proxy session |
| `cubecli baremetal model list` | List available models |

### Networking

| Command | Description |
|---------|-------------|
| `cubecli network create\|list\|update\|delete` | Private networks |
| `cubecli floating-ip list\|acquire\|release` | Floating IP management |
| `cubecli floating-ip assign\|unassign <address>` | IP assignment |
| `cubecli floating-ip reverse-dns <address>` | Reverse DNS |
| `cubecli location list` | List available locations |
| `cubecli ddos-attack list` | View DDoS attack history |

### DNS

| Command | Description |
|---------|-------------|
| `cubecli dns zone list\|show\|create\|delete` | Zone management |
| `cubecli dns zone verify\|scan <uuid>` | Zone verification & import |
| `cubecli dns record list\|create\|update\|delete` | Record management |
| `cubecli dns soa show\|update <uuid>` | SOA configuration |

### Load Balancers

| Command | Description |
|---------|-------------|
| `cubecli lb list\|show\|create\|update\|delete` | LB management |
| `cubecli lb resize <uuid>` | Resize a load balancer |
| `cubecli lb listener create\|update\|delete` | Listener management |
| `cubecli lb target add\|update\|remove\|drain` | Target management |
| `cubecli lb health-check configure\|delete` | Health check config |
| `cubecli lb plan list` | List available plans |

### CDN

| Command | Description |
|---------|-------------|
| `cubecli cdn zone list\|show\|create\|update\|delete` | Zone management |
| `cubecli cdn zone pricing <uuid>` | Zone pricing details |
| `cubecli cdn origin list\|create\|update\|delete` | Origin management |
| `cubecli cdn rule list\|show\|create\|update\|delete` | Edge rules |
| `cubecli cdn waf list\|show\|create\|update\|delete` | WAF rules |
| `cubecli cdn metrics summary\|requests\|bandwidth\|cache` | Analytics |
| `cubecli cdn metrics top-urls\|top-countries\|top-asn` | Top analytics |
| `cubecli cdn plan list` | List available plans |

## Global Flags

```
--json           Output in JSON format
--profile        Profile to use (overrides CUBE_PROFILE and the active profile)
-v, --verbose    Enable verbose output
-h, --help       Help for any command
```

Destructive commands (`delete`, `destroy`, etc.) will prompt for confirmation unless `--force` / `-f` is passed.

## JSON Output

Every command supports `--json` for scripting and piping:

```bash
# Get VPS list as JSON
cubecli vps list --json

# Pipe to jq
cubecli vps list --json | jq '.[].vps[].name'

# Use in scripts
VPS_ID=$(cubecli vps list --json | jq -r '.[0].vps[0].id')
```

## Shell Completions

```bash
# Bash
cubecli completion bash > /etc/bash_completion.d/cubecli

# Zsh
cubecli completion zsh > "${fpath[1]}/_cubecli"

# Fish
cubecli completion fish > ~/.config/fish/completions/cubecli.fish

# PowerShell
cubecli completion powershell > cubecli.ps1
```

## Building from Source

```bash
git clone https://github.com/CubePathInc/cubecli.git
cd cubecli
make build
./cubecli version
```

### Build with version info

```bash
make build VERSION=1.0.0
```

## License

Copyright CubePath, Inc. All rights reserved.
