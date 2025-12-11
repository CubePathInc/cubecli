import typer
from rich.prompt import Prompt
from cubecli.config import save_config, load_config, ConfigError
from cubecli.utils import print_success, print_error, print_info
from cubecli.api_client import APIClient

app = typer.Typer(no_args_is_help=True)

@app.command()
def setup():
    """Configure CubeCLI with your API token"""
    print_info("Configure CubeCLI")
    
    # Check if already configured
    try:
        existing_config = load_config()
        if existing_config.get("api_token"):
            update = typer.confirm("API token already configured. Update it?", default=False)
            if not update:
                return
    except ConfigError:
        pass
    
    # Prompt for API token
    api_token = Prompt.ask("Enter your API token", password=True)
    
    if not api_token:
        print_error("API token cannot be empty")
        raise typer.Exit(1)
    
    # Test the API token
    print_info("Testing API token...")
    try:
        client = APIClient(api_token)
        # Test with a simple endpoint that supports token auth
        client.get("/sshkey/user/sshkeys")
        print_success("API token is valid!")
    except Exception as e:
        print_error(str(e))
        raise typer.Exit(1)
    
    # Save configuration
    config = {"api_token": api_token}
    save_config(config)
    print_success("Configuration saved successfully!")

@app.command()
def show():
    """Show current configuration"""
    try:
        config = load_config()
        api_token = config.get("api_token", "")
        # Mask the token
        masked_token = api_token[:8] + "..." + api_token[-4:] if len(api_token) > 12 else "***"
        print_info(f"API Token: {masked_token}")
    except ConfigError:
        print_error("No configuration found. Run 'cubecli config setup' first.")
        raise typer.Exit(1)