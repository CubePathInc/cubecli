# CubeCLI

Official command-line interface for CubePath Cloud API.

## Installation

### Method 1: Install with pipx (Recommended)

```bash
# Install in isolated environment with pipx
pipx install git+https://github.com/CubePathInc/cubecli.git
```

### Method 2: Install from source

```bash
git clone https://github.com/CubePathInc/cubecli.git
cd cubecli
pipx install .
```


## Configuration

Configure your API token:

```bash
cubecli config
```

Or set environment variable:

```bash
export CUBE_API_TOKEN=your-api-token
```

## Quick Start

1. **Configure your API token:**
   ```bash
   cubecli config
   ```
   
2. **Verify connection:**
   ```bash
   cubecli project list
   ```

## Usage

### SSH Keys

```bash
# Create SSH key
cubecli ssh-key create --name demo --public-key-from-file ~/.ssh/id_rsa.pub

# List SSH keys
cubecli ssh-key list

# Delete SSH key
cubecli ssh-key delete <key-id>
```

### Projects

```bash
# List projects
cubecli project list

# Create project
cubecli project create --name "My Project" --description "Test project"

# Delete project
cubecli project delete <project-id>
```

### Networks

```bash
# List networks
cubecli network list

# Create network
cubecli network create --name test --location <location> --cidr 10.0.0.0/24 --project <project-id>

# Delete network
cubecli network delete <network-id>
```

### VPS

```bash
# Create VPS
cubecli vps create --name demoserver --plan cx11 --template "Debian 12" --ssh demo --project <project-id>

# List VPS
cubecli vps list

# Destroy VPS
cubecli vps destroy <vps-id>

# Power actions
cubecli vps power restart <vps-id>
cubecli vps power stop <vps-id>
cubecli vps power start <vps-id>

# Resize VPS
cubecli vps resize <vps-id> --plan rz.nano

# Change password
cubecli vps change-password <vps-id> --new-password test123

# Reinstall VPS
cubecli vps reinstall <vps-id> --template "Debian 12"
```

### Locations

```bash
# List available locations
cubecli location list
```

### VPS Plans & Templates

```bash
# List available VPS plans with pricing
cubecli vps plan list

# List available VPS templates
cubecli vps template list
```

### Floating IPs

```bash
# List all floating IPs
cubecli floating-ip list
```

## Global Options

- `--json` - Output in JSON format
- `--verbose` - Enable verbose output
- `--help` - Show help message
