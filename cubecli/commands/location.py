import typer
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_error, print_json, create_table,
    get_context_value, with_spinner, format_price, console, handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command("list")
def list_locations(ctx: typer.Context):
    """List all available locations"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching locations...") as progress:
        task = progress.add_task("Fetching locations...", total=None)
        try:
            response = client.get("/pricing")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns {"vps": {"locations": [...]}}
    vps_data = response.get("vps", {})
    vps_locations = vps_data.get("locations", [])
    
    if json_output:
        # Extract only location information for JSON output
        locations_info = []
        for location in vps_locations:
            locations_info.append({
                "location_name": location.get("location_name"),
                "description": location.get("description")
            })
        print_json(locations_info)
    else:
        if not vps_locations:
            print_error("No locations found")
            return
        
        console.print()
        table = create_table("Available Locations", ["Code", "Name"])
        
        for location in vps_locations:
            location_name = location.get("location_name", "Unknown")
            display_name = location.get("description", location_name)
            
            table.add_row(
                location_name,
                display_name
            )
        
        console.print(table)