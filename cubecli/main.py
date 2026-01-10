import typer
from typing import Optional
from rich.console import Console

from cubecli.commands import config, ssh_key, project, network, vps, location, floating_ip, baremetal, ddos_attack, update
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
app.add_typer(update.app, name="update", help="Update cubecli to the latest version")

# Global options
verbose_option = typer.Option(False, "--verbose", "-v", help="Enable verbose output")
json_option = typer.Option(False, "--json", help="Output in JSON format")

@app.callback()
def main(
    ctx: typer.Context,
    verbose: bool = verbose_option,
    json_output: bool = json_option,
):
    """
    CubePath Cloud CLI - Manage your cloud infrastructure
    """
    # Store global options in context
    ctx.ensure_object(dict)
    ctx.obj["verbose"] = verbose
    ctx.obj["json"] = json_output
    
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