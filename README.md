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
# Configure your API token
cubecli config setup

# List your projects
cubecli project list

# List VPS instances
cubecli vps list

# Get help on any command
cubecli <command> --help
```

## Configuration

CubeCLI reads credentials from (in order):

1. `CUBE_API_TOKEN` environment variable
2. Active profile in `~/.cubecli/config.json`

```bash
# Set up interactively (validates the token) — creates the 'default' profile
cubecli config setup

# Or use an environment variable
export CUBE_API_TOKEN="your-api-token"

# Optionally override the API URL
export CUBE_API_URL="https://api.cubepath.com"
```

## Profiles (multiple accounts)

Store multiple API tokens and switch between them — useful for managing a
personal account and a work account from the same shell.

```bash
# Add a new profile (prompts for the token and validates it)
cubecli profile add work

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
--json       Output in JSON format
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
