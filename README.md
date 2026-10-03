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

### Account

| Command | Description |
|---------|-------------|
| `cubecli project list\|show\|create\|update\|delete` | Projects |
| `cubecli ssh-key list\|create\|update\|delete` | SSH keys |

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
| `cubecli vps protection <id> --enable\|--disable` | Destruction protection |
| `cubecli vps move-project <id> --project <id>` | Move to another project |
| `cubecli vps ssh-key add\|remove <id> <key_id>` | SSH keys of a VPS |
| `cubecli vps network attach\|detach <id>` | Private network |
| `cubecli vps console <id>` | VNC console session |
| `cubecli availability-group list\|show\|create\|delete\|add-vps\|remove-vps\|move-project` | Availability groups |

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
| `cubecli baremetal os <id>` | Installable OS and disk layouts |
| `cubecli baremetal kvm <id>` | KVM console URL and credentials |
| `cubecli baremetal protection\|move-project <id>` | Protection and project moves |
| `cubecli baremetal ssh-key add\|remove <id> <key_id>` | SSH keys of a server |
| `cubecli baremetal network attach\|detach <id>` | Private network |

### Networking

| Command | Description |
|---------|-------------|
| `cubecli network create\|list\|update\|delete` | Private networks |
| `cubecli network route list\|create\|delete` | Static routes |
| `cubecli network bgp-peer list\|create\|update\|delete` | BGP sessions (dynamic routes) |
| `cubecli network move-project <id>` | Move a network to another project |
| `cubecli firewall group list\|show\|create\|update\|delete` | VPS firewall groups |
| `cubecli firewall assign <vps_id> --group <id>` | Set the firewall groups of a VPS |
| `cubecli nat-gateway plans\|list\|show\|create\|update\|delete\|resize\|move\|protection` | NAT gateways |
| `cubecli floating-ip list\|acquire\|release` | Floating IP management |
| `cubecli floating-ip assign\|unassign <address>` | IP assignment |
| `cubecli floating-ip reverse-dns <address>` | Reverse DNS |
| `cubecli location list` | List available locations |
| `cubecli ddos-attack list` | View DDoS attack history |
| `cubecli ddos-attack details\|traffic <id>` | Attack details and traffic |
| `cubecli ddos-mitigation ips` | Premium-protected IPs |
| `cubecli ddos-mitigation profile show\|update\|delete <ip>` | Protection profile of an IP |
| `cubecli ddos-mitigation profile countries\|asns\|prefix-lists <ip>` | Filters of a profile (`set-*` to replace) |
| `cubecli ddos-mitigation rule list\|create\|delete\|delete-matching` | Edge firewall rules |
| `cubecli ddos-mitigation prefix-list list\|create\|delete\|entries\|add-entry\|remove-entry` | Prefix lists |
| `cubecli ddos-mitigation countries\|asns` | Geo and ASN catalogs |
| `cubecli ddos-mitigation traffic protected-ips\|capture\|stats` | Traffic seen at the edge |

### DNS

| Command | Description |
|---------|-------------|
| `cubecli dns zone list\|show\|create\|delete` | Zone management |
| `cubecli dns zone verify\|scan <uuid>` | Zone verification & import |
| `cubecli dns record list\|create\|update\|delete` | Record management |
| `cubecli dns soa show\|update <uuid>` | SOA configuration |
| `cubecli dns zone create <domain> --zone-file <file>\|--scan` | Create and fill a zone |
| `cubecli dns zone import <uuid> <file>` | Import a BIND zone file |
| `cubecli dns zone move-project <uuid>` | Move a zone to another project |
| `cubecli dns health-check list\|show\|set\|delete` | Failover health checks |
| `cubecli dns regions` | GeoDNS regions |

### Load Balancers

| Command | Description |
|---------|-------------|
| `cubecli lb list\|show\|create\|update\|delete` | LB management |
| `cubecli lb resize <uuid>` | Resize a load balancer |
| `cubecli lb listener create\|update\|delete` | Listener management |
| `cubecli lb target add\|update\|remove\|drain` | Target management |
| `cubecli lb health-check configure\|delete` | Health check config |
| `cubecli lb plan list` | List available plans |
| `cubecli lb target add-batch <uuid> <listener>` | Add up to 50 targets at once |
| `cubecli lb protection\|move-project <uuid>` | Protection and project moves |

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
| `cubecli cdn cache purge\|purges <uuid>` | Purge the edge cache and follow it |
| `cubecli cdn token-auth enable\|disable\|rotate-secret\|sign-url` | Signed URLs |

### Object Storage

S3-compatible buckets (alias `s3`). Tiers take a slug, a uuid or `ia`; buckets
take their uuid or name.

| Command | Description |
|---------|-------------|
| `cubecli objectstorage tiers` | Tiers with endpoint, prices and free tier |
| `cubecli objectstorage bucket list\|get\|create\|update\|delete` | Bucket management (`create <name> --tier ia`, `delete --purge` also deletes the content) |
| `cubecli objectstorage bucket object-lock set <bucket> --mode governance\|compliance --days N\|--years N` | Change the default retention of a bucket with Object Lock (`--remove` drops a governance rule) |
| `cubecli objectstorage key list\|create\|delete` | Access keys; the secret is shown once (`create --bypass-governance` for keys that may delete governance versions) |
| `cubecli objectstorage usage [--period YYYY-MM] [--tag k=v]` | Month usage and cost per tier and bucket |
| `cubecli objectstorage presign <bucket>/<key> [--expires 1h]` | Temporary download link for one object, signed locally with your access key |
| `cubecli objectstorage bucket metrics <bucket> [--range 24h] [--part storage,traffic,responses]` | Stored size, traffic and responses over 1h to 30d (needs an API token: GraphQL) |
| `cubecli objectstorage bucket lifecycle get\|set\|delete <bucket>` | Lifecycle rules: `set --file rules.json` (or `-` for stdin) or `set --expire-days 30 --prefix logs/`; `--wait` until applied |
| `cubecli objectstorage replication list\|get\|create\|update\|delete\|resync <replication>` | Replicate a bucket to another CubePath bucket or to an external S3 bucket (takes the replication uuid or the source bucket name) |
| `cubecli objectstorage replication revoke <uuid>` | Stop a replication from another organization into one of your buckets |
| `cubecli objectstorage replication grant create\|list\|delete` | One use tokens that let another organization replicate into a bucket; the token is shown once |

```bash
cubecli s3 bucket create photos --tier ia --tag env=prod --tag team=web
cubecli s3 bucket list --tag env=prod --tag team     # key=value or just key; all must match
cubecli s3 bucket update photos --tag env=staging    # replaces every tag; --clear-tags removes them
cubecli s3 key create --name backups --tier ia --output env > .env.cubepath-storage   # new file; or rclone, aws
aws s3 ls s3://photos --endpoint-url https://eu.cubestorage.io --region eu
cubecli s3 bucket metrics photos --range 7d --part traffic        # egress, CDN and requests of the week
```

Lifecycle rules delete objects in the background, permanently. They are set through
cubecli, the API and the dashboard (the S3 `PutBucketLifecycleConfiguration` call
answers 403; reading them with S3 works). Setting rules replaces all of them; they are
applied within seconds (up to about 12 minutes after a previous change of the same bucket)
and objects go within 48 hours of their due date. In a versioned bucket an expiration
only adds a delete marker: add a `noncurrent_version_expiration` rule to free the space.
Incomplete multipart uploads are always aborted after 7 days.

```bash
cubecli s3 bucket lifecycle set logs --expire-days 30 --prefix logs/ --wait
cubecli s3 bucket lifecycle set photos --file rules.json    # {"rules": [{"id": "old-versions", "enabled": true,
                                                            #   "noncurrent_version_expiration": {"noncurrent_days": 30}}]}
cubecli s3 bucket lifecycle get photos
cubecli s3 bucket lifecycle delete photos
```

Replication copies every new object version of a bucket, asynchronously, to one
destination: another CubePath bucket of the same tier, or a bucket of an external S3
compatible provider over HTTPS (port 443 only). Versioning must be enabled on both
sides, and buckets with Object Lock cannot be sources. A CubePath destination lives in
the same location as the source, so it is not a disaster recovery copy: use an external
destination for an off site copy. Data sent to an external destination is billed as
egress of the source bucket. The secret of an external destination is never taken from
the command line: pipe it with `--secret-key-stdin` or set `CUBEPATH_REPL_SECRET`.

```bash
cubecli s3 replication create photos --dest-bucket photos-copy          # same organization
printf '%s' "$AWS_SECRET" | cubecli s3 replication create photos --external --provider aws \
  --endpoint s3.eu-west-1.amazonaws.com --region eu-west-1 --bucket acme-photos-backup \
  --access-key AKIA... --secret-key-stdin
cubecli s3 replication get photos                                       # health, initial copy, metrics
cubecli s3 replication update photos --enabled=false                    # pause; --enabled resumes
cubecli s3 replication resync photos --older-than-days 3
cubecli s3 replication grant create photos-backup --note "for Acme"     # another organization: give it the
cubecli s3 replication create photos --dest-bucket <uuid> --grant-token cprg_...   # token and the bucket uuid
```

Buckets are private. To serve one publicly, add it as the origin of a CDN zone:
`cubecli cdn origin create <zone_uuid> --name photos --bucket photos`. Deleting that
origin disconnects the bucket.

Object Lock (WORM) is chosen when the bucket is created and can never be added
later; it keeps versioning enabled and starts the bucket protected. Governance
retention can be bypassed by keys created with `--bypass-governance`; compliance
retention cannot be deleted or shortened by anyone before its date (cubecli asks
for confirmation unless `--yes`). Deleting a bucket keeps the versions still under
retention or legal hold (`bucket get` shows "Locked content kept") and they keep
being billed.

```bash
cubecli s3 bucket create veeam --tier ia --object-lock --accept-object-lock-terms   # Veeam sets retention per object
cubecli s3 bucket create archive --tier ia --object-lock --lock-mode governance --lock-days 30 --accept-object-lock-terms
cubecli s3 bucket object-lock set archive --mode governance --days 90 --accept-object-lock-terms
cubecli s3 key create --name veeam --tier ia --bucket veeam --bypass-governance --output aws
cubecli s3 bucket delete archive --purge --bypass-governance   # also deletes governance versions
```

Bucket tags (at most 50 per bucket) are managed with cubecli, the API and the
dashboard; S3 bucket tagging calls (`GetBucketTagging`, `PutBucketTagging`) answer
403. Object tags are standard S3 object tagging and work with any S3 client:

```bash
aws s3api put-object-tagging --bucket photos --key 2026/report.pdf \
  --tagging 'TagSet=[{Key=class,Value=archive}]' \
  --endpoint-url https://eu.cubestorage.io --region eu
```

To share one file for a while, sign a presigned URL. It is signed on your machine
(the secret is never sent), lasts at most 24 hours, always downloads as an
attachment, and every download counts as egress of the bucket. Deleting the access
key that signed it cuts the link before it expires. With `--endpoint` no login is needed:

```bash
export AWS_ACCESS_KEY_ID=CP... AWS_SECRET_ACCESS_KEY=...
cubecli s3 presign photos/2026/report.pdf --expires 6h
cubecli s3 presign photos/2026/report.pdf --endpoint https://eu.cubestorage.io --region eu --json
```

Event notifications send what happens in a bucket (objects created, removed or
tagged) to a signed webhook or to a Slack or Discord channel of Cloud Alerts. The
signing secret of a webhook is printed only by `create` and `rotate-secret`; after a
rotation the previous secret keeps signing for 24 hours. Every delivery carries
`CubePath-Timestamp` and `CubePath-Signature: v1=<hex HMAC-SHA256 of timestamp + "." + raw body>`;
reject deliveries older than 5 minutes.

```bash
cubecli s3 events destination create --name uploads-hook --webhook https://example.com/hooks/storage
cubecli s3 events destination create --name ops --channel <notificator id> --format cubepath
cubecli s3 events rule create --bucket photos --destination uploads-hook --events created,removed --prefix incoming/ --suffix .jpg
cubecli s3 events rule list --bucket photos
cubecli s3 events destination test uploads-hook
cubecli s3 events destination deliveries uploads-hook --status failed
cubecli s3 events destination rotate-secret uploads-hook
cubecli s3 events rule delete on-created-removed --bucket photos
```

### Kubernetes

| Command | Description |
|---------|-------------|
| `cubecli kubernetes versions\|plans` | Versions and plans |
| `cubecli kubernetes list\|show\|create\|update\|delete` | Cluster management |
| `cubecli kubernetes kubeconfig\|move\|loadbalancers <uuid>` | Access and placement |
| `cubecli kubernetes protection <uuid>` | Destruction protection |
| `cubecli kubernetes metrics <uuid> [--node <name>]` | Cluster or node health metrics |
| `cubecli kubernetes node-pool ...` / `addon ...` | Node pools and addons |

### Managed Databases

MySQL, PostgreSQL and Valkey (alias `mdb`). Plans are priced per node.

| Command | Description |
|---------|-------------|
| `cubecli mdb plans [--engine mysql]` | Plans per location |
| `cubecli mdb list\|show\|create\|update\|delete` | Instance management |
| `cubecli mdb scale <uuid> --replicas N\|--plan <uuid>` | Horizontal or vertical scaling |
| `cubecli mdb credentials\|rotate-credentials <uuid>` | Admin connection credentials |
| `cubecli mdb config show\|set <uuid>` | Engine parameters |
| `cubecli mdb metrics <uuid>` | Connections, CPU, memory, replication lag |
| `cubecli mdb database list\|create\|delete` | Logical databases |
| `cubecli mdb user list\|create\|delete` | Database users (the password is shown once) |
| `cubecli mdb protection <uuid>` | Destruction protection |

```bash
cubecli mdb create --name app-db --engine postgresql --version 17.5.0 --plan <plan_uuid> --project 12 --replicas 2
cubecli mdb credentials <uuid>
```

### Cloud Alerts

| Command | Description |
|---------|-------------|
| `cubecli alert notificator list\|show\|create\|update\|delete` | Slack, Discord or email channels |
| `cubecli alert list\|show\|create\|update\|delete` | Metric alerts on VPS, baremetal or availability groups |
| `cubecli alert history <id>` | When an alert fired and recovered |

### Video Transcoder

| Command | Description |
|---------|-------------|
| `cubecli transcoder create` | Submit a job (URL or S3 source, S3 destination) |
| `cubecli transcoder batch --file <json>` | Submit up to 1000 jobs |
| `cubecli transcoder list\|show\|outputs\|cancel` | Follow jobs and their files |

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
