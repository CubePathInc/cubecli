import typer
from typing import Optional
from rich.console import Console

from cubecli.commands import config, ssh_key, project, network, vps, location, floating_ip, baremetal, ddos_attack, dns, loadbalancer, cdn
from cubecli.commands.update import update_cubecli
from cubecli.config import load_config, ConfigError
from cubecli.utils import print_error

app = typer.Typer(
    name="cubecli",
    help="CubePath Cloud CLI - Manage your cloud infrastructure", 
    rich_markup_mode="rich",
    add_completion=True,
    no_args_is_help=True,
    context_settings={"help_option_names": ["--help", "-h"]},
)

console = Console()

# Add subcommands
app.add_typer(config.app, name="config", help="Configure CubeCLI")
app.add_typer(ssh_key.app, name="ssh-key", help="Manage SSH keys")
app.add_typer(project.app, name="project", help="Manage projects")
app.add_typer(network.app, name="network", help="Manage networks")
app.add_typer(vps.app, name="vps", help="Manage VPS instances")
app.add_typer(baremetal.app, name="baremetal", help="Manage baremetal servers")
app.add_typer(location.app, name="location", help="List available locations")
app.add_typer(floating_ip.app, name="floating-ip", help="Manage floating IPs")
app.add_typer(ddos_attack.app, name="ddos-attack", help="View DDoS attack history")
app.add_typer(dns.app, name="dns", help="Manage DNS zones and records")
app.add_typer(loadbalancer.app, name="lb", help="Manage load balancers")
app.add_typer(cdn.app, name="cdn", help="Manage CDN zones and distribution")
app.command(name="update", help="Update cubecli to the latest version")(update_cubecli)

@app.callback()
def main(ctx: typer.Context):
    """
    CubePath Cloud CLI - Manage your cloud infrastructure
    """
    # Store config in context
    ctx.ensure_object(dict)

    # Check for configuration on commands that need it
    if ctx.invoked_subcommand and ctx.invoked_subcommand != "config":
        try:
            config = load_config()
            ctx.obj["config"] = config
            ctx.obj["api_token"] = config.get("api_token")
        except ConfigError:
            if ctx.invoked_subcommand not in ["--help", "help"]:
                print_error("No API token configured. Run 'cubecli config' first.")
                raise typer.Exit(1)

if __name__ == "__main__":
    app()