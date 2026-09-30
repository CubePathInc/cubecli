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
# Log in (asks for an API token)
cubecli login

# List your projects
cubecli project list

# List VPS instances
cubecli vps list

# Get help on any command
cubecli <command> --help
```

## Authentication

`cubecli login` stores your credentials in a profile. It asks for an API
token, which you create in the dashboard at
https://my.cubepath.com/organization/tokens. Once the CubePath API supports
browser sign-in, the same command opens your browser instead, with no update
needed.

```bash
# Log in the active profile ('default' if there is none)
cubecli login

# Log in a named profile
cubecli login work

# See how every profile is authenticated
cubecli auth status

# Remove the stored credentials
cubecli logout work
cubecli logout --all
```

### CI and scripts

For non-interactive use, store a token in a profile or pass it through the
environment:

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

## MCP server

The CubePath MCP server (`https://mcp.cubepath.com/mcp`) gives AI agents
direct access to your infrastructure through tools. `cubecli login` also offers
to add it to the agents on your machine (skip with `--skip-mcp`).

```bash
cubecli mcp install                     # every agent found on this machine
cubecli mcp install --agent claude      # claude, codex, gemini, cursor or vscode
cubecli mcp status
cubecli mcp uninstall
```

Only the server URL is written to each agent's configuration. The first time
an agent connects, it opens your browser to approve the access, where you
choose the organization and the permissions. Disconnect agents at any time at
https://my.cubepath.com/account/connections.

## Profiles (multiple accounts and organizations)

Each profile holds the credentials of one organization. Log in once per
organization and switch between them.

```bash
# Log in a new profile (one API token per organization)
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
| `cubecli baremetal reinstall start\|status\|cancel <id>` | OS reinstallation |
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

### Object Storage

S3-compatible buckets (alias `s3`). Tiers take a slug, a uuid or `ia`; buckets
take their uuid or name.

| Command | Description |
|---------|-------------|
| `cubecli objectstorage tiers` | Tiers with endpoint, prices and free tier |
| `cubecli objectstorage bucket list\|get\|create\|update\|delete` | Bucket management (`create <name> --tier ia`, `delete --purge` also deletes the content) |
| `cubecli objectstorage key list\|create\|delete` | Access keys; the secret is shown once |
| `cubecli objectstorage usage [--period YYYY-MM]` | Month usage and cost per tier and bucket |

```bash
cubecli s3 bucket create photos --tier ia
cubecli s3 key create --name backups --tier ia --output env > .env.cubepath-storage   # new file; or rclone, aws
aws s3 ls s3://photos --endpoint-url https://eu.cubestorage.io --region eu
```

Buckets are private. To serve one publicly, add it as the origin of a CDN zone:
`cubecli cdn origin create <zone_uuid> --name photos --bucket photos`. Deleting that
origin disconnects the bucket.

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
