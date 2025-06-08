import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, console, handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command("list")
def list_floating_ips(ctx: typer.Context):
    """List all floating IPs"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching floating IPs...") as progress:
        task = progress.add_task("Fetching floating IPs...", total=None)
        try:
            # Floating IPs are part of VPS data in projects
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # Extract floating IPs from VPS data
    project_list = response if isinstance(response, list) else response.get("projects", [])
    
    all_floating_ips = []
    for item in project_list:
        project = item.get("project", {})
        vps_list = item.get("vps", [])
        for vps in vps_list:
            floating_ip = vps.get("floating_ip", {})
            if floating_ip and floating_ip.get("address"):
                # Add VPS and project info to the floating IP
                floating_ip["vps_id"] = vps.get("id")
                floating_ip["vps_name"] = vps.get("name")
                floating_ip["project_name"] = project.get("name")
                floating_ip["location"] = vps.get("location", {}).get("description", "N/A")
                all_floating_ips.append(floating_ip)
    
    if json_output:
        print_json(all_floating_ips)
    else:
        if not all_floating_ips:
            print_error("No floating IPs found")
            return
        
        console.print()
        table = create_table("Floating IPs", ["IP Address", "VPS", "Project", "Location"])
        
        for ip in all_floating_ips:
            table.add_row(
                ip.get("address", "N/A"),
                ip.get("vps_name", "N/A"),
                ip.get("project_name", "N/A"),
                ip.get("location", "N/A")
            )
        
        console.print(table)