import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, console, handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command()
def create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="Network name"),
    location: str = typer.Option(..., "--location", "-l", help="Location name"),
    cidr: str = typer.Option(..., "--cidr", "-c", help="Network CIDR (e.g., 10.0.0.0/24)"),
    project_id: int = typer.Option(..., "--project", "-p", help="Project ID"),
    label: Optional[str] = typer.Option(None, "--label", help="Network label"),
):
    """Create a new network"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    # Parse CIDR notation
    try:
        ip_range, prefix_str = cidr.split('/')
        prefix = int(prefix_str)
        if prefix < 0 or prefix > 32:
            raise ValueError("Prefix must be between 0 and 32")
    except ValueError as e:
        print_error(f"Invalid CIDR format. Use format like 10.0.0.0/24. Error: {e}")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    data = {
        "name": name,
        "location_name": location,
        "ip_range": ip_range,
        "prefix": prefix,
        "project_id": project_id
    }
    
    if label:
        data["label"] = label
    
    with with_spinner("Creating network...") as progress:
        task = progress.add_task("Creating network...", total=None)
        try:
            response = client.post("/networks/create_network", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"Network '{name}' created successfully!")
        print_success(f"Network ID: {response.get('id', 'N/A')}")
        print_success(f"IP Range: {cidr}")

@app.command("list")
def list_networks(ctx: typer.Context):
    """List all networks"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching networks...") as progress:
        task = progress.add_task("Fetching networks...", total=None)
        try:
            # Networks are part of projects
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns a list directly
    project_list = response if isinstance(response, list) else response.get("projects", [])
    
    # Extract all networks from all projects
    all_networks = []
    for item in project_list:
        project = item.get("project", {})
        networks = item.get("networks", [])
        for network in networks:
            network["project_name"] = project.get("name", "N/A")
            network["project_id"] = project.get("id", "N/A")
            all_networks.append(network)
    
    if json_output:
        print_json(all_networks)
    else:
        if not all_networks:
            print_error("No networks found")
            return
        
        table = create_table("Networks", ["ID", "Name", "Project", "IP Range", "Location", "VPS Count"])
        
        for network in all_networks:
            vps_count = len(network.get("vps", []))
            
            table.add_row(
                str(network["id"]),
                network["name"],
                network["project_name"],
                f"{network.get('ip_range', 'N/A')}/{network.get('prefix', 'N/A')}",
                network.get("location_name", "N/A"),
                str(vps_count)
            )
        
        console.print()
        console.print(table)

@app.command()
def update(
    ctx: typer.Context,
    network_id: int = typer.Argument(..., help="Network ID to update"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="New network name"),
    label: Optional[str] = typer.Option(None, "--label", help="New network label"),
):
    """Update a network"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not name and not label:
        print_error("At least one of --name or --label must be provided")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    data = {}
    if name:
        data["name"] = name
    if label:
        data["label"] = label
    
    with with_spinner("Updating network...") as progress:
        task = progress.add_task("Updating network...", total=None)
        try:
            response = client.put(f"/networks/{network_id}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"Network {network_id} updated successfully!")

@app.command()
def delete(
    ctx: typer.Context,
    network_id: int = typer.Argument(..., help="Network ID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Delete a network"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not force:
        if not confirm_action(f"Are you sure you want to delete network {network_id}?"):
            print_error("Operation cancelled")
            return
    
    client = APIClient(api_token)
    
    with with_spinner("Deleting network...") as progress:
        task = progress.add_task("Deleting network...", total=None)
        try:
            response = client.delete(f"/networks/{network_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json({"message": "Network deleted successfully", "network_id": network_id})
    else:
        print_success(f"Network {network_id} deleted successfully!")