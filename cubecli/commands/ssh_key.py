import typer
from pathlib import Path
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_info, print_json, create_table,
    confirm_action, get_context_value, with_spinner, console, handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command()
def create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="Name for the SSH key"),
    public_key_from_file: Optional[Path] = typer.Option(None, "--public-key-from-file", "-f", help="Path to public key file"),
    public_key: Optional[str] = typer.Option(None, "--public-key", "-k", help="Public key string"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Create a new SSH key"""
    api_token = get_context_value(ctx, "api_token")
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    # Get public key
    if public_key_from_file:
        if not public_key_from_file.exists():
            print_error(f"File not found: {public_key_from_file}")
            raise typer.Exit(1)
        ssh_key = public_key_from_file.read_text().strip()
    elif public_key:
        ssh_key = public_key.strip()
    else:
        print_error("Either --public-key-from-file or --public-key must be provided")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Creating SSH key...") as progress:
        task = progress.add_task("Creating SSH key...", total=None)
        try:
            response = client.post("/sshkey/create", {
                "name": name,
                "ssh_key": ssh_key
            })
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"SSH key '{name}' created successfully!")

@app.command("list")
def list_keys(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all SSH keys"""
    api_token = get_context_value(ctx, "api_token")
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching SSH keys...") as progress:
        task = progress.add_task("Fetching SSH keys...", total=None)
        try:
            response = client.get("/sshkey/user/sshkeys")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API might return the list directly or wrapped in an object
    if isinstance(response, list):
        ssh_keys = response
    else:
        ssh_keys = response.get("ssh_keys", response.get("sshkeys", []))
    
    if json_output:
        print_json(ssh_keys)
    else:
        if not ssh_keys:
            print_error("No SSH keys found")
            return

        table = create_table("SSH Keys", ["ID", "Name", "Type", "Fingerprint"])

        for key in ssh_keys:
            table.add_row(
                str(key["id"]),
                key["name"],
                key.get("key_type", "N/A"),
                key.get("fingerprint", "N/A")
            )

        console.print()
        console.print(table)

@app.command()
def delete(
    ctx: typer.Context,
    key_id: int = typer.Argument(..., help="SSH key ID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Delete an SSH key"""
    api_token = get_context_value(ctx, "api_token")
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not force:
        if not confirm_action(f"Are you sure you want to delete SSH key {key_id}?"):
            print_error("Operation cancelled")
            return
    
    client = APIClient(api_token)
    
    with with_spinner("Deleting SSH key...") as progress:
        task = progress.add_task("Deleting SSH key...", total=None)
        try:
            response = client.delete(f"/sshkey/{key_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json({"detail": "SSH key deleted successfully", "key_id": key_id})
    else:
        print_success(f"SSH key {key_id} deleted successfully!")