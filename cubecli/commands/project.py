import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, format_status, console,
    handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command()
def create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="Project name"),
    description: Optional[str] = typer.Option(None, "--description", "-d", help="Project description"),
):
    """Create a new project"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    data = {"name": name}
    if description:
        data["description"] = description
    
    with with_spinner("Creating project...") as progress:
        task = progress.add_task("Creating project...", total=None)
        try:
            response = client.post("/projects/", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"Project '{name}' created successfully!")
        print_success(f"Project ID: {response.get('id', 'N/A')}")

@app.command("list")
def list_projects(ctx: typer.Context):
    """List all projects"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching projects...") as progress:
        task = progress.add_task("Fetching projects...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns a list directly, not a dict with "projects" key
    projects = response if isinstance(response, list) else response.get("projects", [])
    
    if json_output:
        print_json(projects)
    else:
        if not projects:
            print_error("No projects found")
            return
        
        table = create_table("Projects", ["ID", "Name", "Description", "VPS Count", "Networks", "Created"])
        
        for item in projects:
            # Extract project info from the structure
            project = item.get("project", {})
            vps_count = len(item.get("vps", []))
            networks_count = len(item.get("networks", []))
            
            description = project.get("description", "N/A")
            if description and len(description) > 30:
                description = description[:30] + "..."
            
            table.add_row(
                str(project.get("id", "N/A")),
                project.get("name", "N/A"),
                description,
                str(vps_count),
                str(networks_count),
                project.get("created_at", "N/A")
            )
        
        console.print()
        console.print(table)

@app.command()
def show(
    ctx: typer.Context,
    project_id: int = typer.Argument(..., help="Project ID to show"),
):
    """Show detailed project information"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching project details...") as progress:
        task = progress.add_task("Fetching project details...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns a list directly
    projects = response if isinstance(response, list) else response.get("projects", [])
    
    # Find the specific project
    project_data = None
    for item in projects:
        proj = item.get("project", {})
        if proj.get("id") == project_id:
            project_data = item
            break
    
    if not project_data:
        print_error(f"Project {project_id} not found")
        raise typer.Exit(1)
    
    project = project_data.get("project", {})
    
    if json_output:
        print_json(project_data)
    else:
        console.print()
        console.print(f"[bold]Project: {project.get('name', 'N/A')}[/bold]")
        console.print(f"ID: {project.get('id', 'N/A')}")
        console.print(f"Description: {project.get('description', 'N/A')}")
        console.print(f"Created: {project.get('created_at', 'N/A')}")
        
        # Show VPS if any
        vps_list = project_data.get("vps", [])
        if vps_list:
            console.print()
            vps_table = create_table(f"VPS in Project ({len(vps_list)})", ["ID", "Name", "Status", "IP", "Plan"])
            for vps in vps_list:
                # Extract floating IP
                floating_ip = vps.get("floating_ip", {})
                ip_address = floating_ip.get("address", "N/A")
                
                # Extract plan info
                plan = vps.get("plan", {})
                plan_name = plan.get("plan_name", "N/A")
                
                vps_table.add_row(
                    str(vps.get("id", "N/A")),
                    vps.get("name", "N/A"),
                    format_status(vps.get("status", "unknown")),
                    ip_address,
                    plan_name
                )
            console.print(vps_table)
        
        # Show networks if any
        networks = project_data.get("networks", [])
        if networks:
            console.print()
            net_table = create_table(f"Networks in Project ({len(networks)})", ["ID", "Name", "IP Range", "Location"])
            for net in networks:
                net_table.add_row(
                    str(net.get("id", "N/A")),
                    net.get("name", "N/A"),
                    f"{net.get('ip_range', 'N/A')}/{net.get('prefix', 'N/A')}",
                    net.get("location_name", "N/A")
                )
            console.print(net_table)

@app.command()
def delete(
    ctx: typer.Context,
    project_id: int = typer.Argument(..., help="Project ID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Delete a project"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not force:
        if not confirm_action(f"Are you sure you want to delete project {project_id}?"):
            print_error("Operation cancelled")
            return
    
    client = APIClient(api_token)
    
    with with_spinner("Deleting project...") as progress:
        task = progress.add_task("Deleting project...", total=None)
        try:
            response = client.delete(f"/projects/{project_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json({"message": "Project deleted successfully", "project_id": project_id})
    else:
        print_success(f"Project {project_id} deleted successfully!")