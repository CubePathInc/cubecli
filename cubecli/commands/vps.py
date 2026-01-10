import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, format_status,
    format_bytes, console, with_progress_bar, handle_api_exception
)
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TimeRemainingColumn
from rich.table import Table
from rich import box
import httpx

app = typer.Typer(no_args_is_help=True)

# Power subcommand
power_app = typer.Typer(no_args_is_help=True)
app.add_typer(power_app, name="power", help="VPS power management")

# Plan subcommand
plan_app = typer.Typer(no_args_is_help=True)
app.add_typer(plan_app, name="plan", help="VPS plan information")

# Template subcommand
template_app = typer.Typer(no_args_is_help=True)
app.add_typer(template_app, name="template", help="VPS template information")

@app.command()
def create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="VPS hostname"),
    plan: str = typer.Option(..., "--plan", "-p", help="Plan name (e.g., cx11)"),
    template: str = typer.Option(..., "--template", "-t", help="Template name (e.g., debian-12)"),
    project_id: int = typer.Option(..., "--project", help="Project ID"),
    location: str = typer.Option(..., "--location", "-l", help="Location name"),
    ssh_keys: Optional[list[str]] = typer.Option(None, "--ssh", "-s", help="SSH key name (can be used multiple times)"),
    network_id: Optional[int] = typer.Option(None, "--network", help="Network ID"),
    label: Optional[str] = typer.Option(None, "--label", help="VPS label"),
    password: Optional[str] = typer.Option(None, "--password", help="Root password"),
):
    """Create a new VPS"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    # Require either SSH key or password
    if not ssh_keys and not password:
        raise typer.BadParameter("You must provide either --ssh (SSH key) or --password")

    client = APIClient(api_token)

    data = {
        "name": name,
        "plan_name": plan,
        "template_name": template,
        "location_name": location,
        "label": label or name
    }

    if ssh_keys:
        data["ssh_key_names"] = ssh_keys
    if network_id:
        data["network_id"] = network_id
    if password:
        data["password"] = password
    
    # Use progress bar for VPS creation
    with Progress(
        SpinnerColumn(spinner_name="dots", style="green"),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(bar_width=40, style="green", complete_style="green"),
        TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
        transient=True,
    ) as progress:
        task = progress.add_task("Creating VPS...", total=100)
        
        try:
            # Simulate progress
            progress.update(task, advance=20, description="Validating request...")
            import time
            time.sleep(0.3)
            
            progress.update(task, advance=20, description="Sending to cloud...")
            response = client.post(f"/vps/create/{project_id}", data)
            
            progress.update(task, advance=30, description="Provisioning VPS...")
            time.sleep(0.5)
            
            progress.update(task, advance=20, description="Configuring network...")
            time.sleep(0.3)
            
            progress.update(task, advance=10, description="VPS created!")
            progress.update(task, completed=100)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"VPS '{name}' created successfully!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")
        print_success("VPS is being provisioned. Use 'cubecli vps list' to check status.")

@app.command("list")
def list_vps(
    ctx: typer.Context,
    project_id: Optional[int] = typer.Option(None, "--project", "-p", help="Filter by project ID"),
    location_filter: Optional[str] = typer.Option(None, "--location", "-l", help="Filter by location name")
):
    """List all VPS instances"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching VPS instances...") as progress:
        task = progress.add_task("Fetching VPS instances...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns a list directly
    project_list = response if isinstance(response, list) else response.get("projects", [])

    # Extract all VPS from all projects
    all_vps = []
    for item in project_list:
        project = item.get("project", {})
        vps_list = item.get("vps", [])
        for vps in vps_list:
            vps["project_name"] = project.get("name", "N/A")
            vps["project_id"] = project.get("id", "N/A")
            # Extract main IP from floating_ips array
            floating_ips_data = vps.get("floating_ips", {})
            # Check if floating_ips is a dict with 'list' key or a direct list
            if isinstance(floating_ips_data, dict):
                floating_ips = floating_ips_data.get("list", [])
            else:
                floating_ips = floating_ips_data if isinstance(floating_ips_data, list) else []

            if floating_ips and len(floating_ips) > 0:
                # Get the first IPv4 address
                ipv4_ips = [ip for ip in floating_ips if ip.get("type") == "IPv4"]
                if ipv4_ips:
                    vps["main_ip"] = ipv4_ips[0]["address"]
                else:
                    # If no IPv4, get the first IP
                    vps["main_ip"] = floating_ips[0]["address"]
            else:
                vps["main_ip"] = "N/A"
            # Extract plan name
            plan = vps.get("plan", {})
            vps["plan_name"] = plan.get("plan_name", "N/A")
            # Extract OS name (use os_name instead of template_name for consistency with baremetal)
            template = vps.get("template", {})
            vps["os_name"] = template.get("os_name", "N/A")
            # Extract location (this might need adjustment based on actual data structure)
            location = vps.get("location", {})
            vps["location_name"] = location.get("location_name", "N/A")
            all_vps.append(vps)

    if project_id is not None:
        all_vps = [vps for vps in all_vps if vps.get("project_id") == project_id]

    if location_filter is not None:
        all_vps = [vps for vps in all_vps if vps.get("location_name") == location_filter]

    if json_output:
        print_json(all_vps)
    else:
        if not all_vps:
            print_error("No VPS instances found")
            return
        
        table = create_table("VPS Instances", ["ID", "Name", "Project", "Status", "IP", "Plan", "OS", "Location"])

        for vps in all_vps:
            table.add_row(
                str(vps["id"]),
                vps["name"],
                vps["project_name"],
                format_status(vps.get("status", "unknown")),
                vps.get("main_ip", "N/A"),
                vps.get("plan_name", "N/A"),
                vps.get("os_name", "N/A"),
                vps.get("location_name", "N/A")
            )
        
        console.print()
        console.print(table)

@app.command()
def show(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to show"),
):
    """Show detailed VPS information"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    # Get VPS details
    with with_spinner("Fetching VPS details...") as progress:
        task = progress.add_task("Fetching VPS details...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns a list directly
    project_list = response if isinstance(response, list) else response.get("projects", [])
    
    # Find the VPS
    vps_found = None
    for item in project_list:
        project = item.get("project", {})
        vps_list = item.get("vps", [])
        for vps in vps_list:
            if vps.get("id") == vps_id:
                vps["project_name"] = project.get("name", "N/A")
                # Extract additional info
                floating_ips = vps.get("floating_ips", [])
                if floating_ips and len(floating_ips) > 0:
                    # Get the first IPv4 address
                    ipv4_ips = [ip for ip in floating_ips if ip.get("type") == "IPv4"]
                    if ipv4_ips:
                        vps["main_ip"] = ipv4_ips[0]["address"]
                    else:
                        # If no IPv4, get the first IP
                        vps["main_ip"] = floating_ips[0]["address"]
                else:
                    vps["main_ip"] = "N/A"
                plan = vps.get("plan", {})
                vps["plan_name"] = plan.get("plan_name", "N/A")
                template = vps.get("template", {})
                vps["os_name"] = template.get("os_name", "N/A")
                location = vps.get("location", {})
                vps["location_name"] = location.get("location_name", "N/A")
                vps_found = vps
                break
        if vps_found:
            break
    
    if not vps_found:
        print_error(f"VPS {vps_id} not found")
        raise typer.Exit(1)
    
    if json_output:
        print_json(vps_found)
    else:
        console.print()
        
        # Basic info table
        info_table = Table(show_header=False, box=box.ROUNDED, show_lines=True)
        info_table.add_column("Property", style="bold")
        info_table.add_column("Value")
        
        info_table.add_row("VPS Name", vps_found['name'])
        info_table.add_row("ID", str(vps_found['id']))
        info_table.add_row("Status", format_status(vps_found.get('status', 'unknown')))
        info_table.add_row("Project", vps_found.get('project_name', 'N/A'))
        info_table.add_row("Location", vps_found.get('location_name', 'N/A'))
        info_table.add_row("Created", vps_found.get('created_at', 'N/A'))
        
        console.print(info_table)
        
        # Plan details table
        plan = vps_found.get('plan', {})
        if plan:
            console.print()
            plan_table = Table(title="Plan Details", box=box.ROUNDED, show_lines=True, title_style="bold green")
            plan_table.add_column("Resource", style="bold")
            plan_table.add_column("Value")
            
            plan_table.add_row("Plan", plan.get('plan_name', 'N/A'))
            plan_table.add_row("vCPUs", str(plan.get('cpu', 'N/A')))
            plan_table.add_row("RAM", f"{plan.get('ram', 0)} MB")
            plan_table.add_row("Storage", f"{plan.get('storage', 0)} GB")
            plan_table.add_row("Bandwidth", f"{plan.get('bandwidth', 0)} GB")
            plan_table.add_row("Price/Hour", f"${plan.get('price_per_hour', 0)}")
            
            console.print(plan_table)
        
        # Network info table
        console.print()
        net_table = Table(title="Network Information", box=box.ROUNDED, show_lines=True, title_style="bold green")
        net_table.add_column("Type", style="bold cyan")
        net_table.add_column("Details", style="white")
        
        # Add floating IPs
        floating_ips = vps_found.get('floating_ips', [])
        for ip in floating_ips:
            ip_type = ip.get('type', 'Unknown')
            address = ip['address']
            if ip_type == 'IPv4':
                net_table.add_row("Public IPv4", f"[bold green]{address}[/bold green]")
            elif ip_type == 'IPv6':
                net_table.add_row("Public IPv6", f"[green]{address}[/green]")
            else:
                net_table.add_row(f"Public {ip_type}", address)
        
        # Add IPv6 if available (legacy support)
        ipv6 = vps_found.get('ipv6')
        if ipv6 and not any(ip.get('type') == 'IPv6' for ip in floating_ips):
            net_table.add_row("Public IPv6", f"[green]{ipv6}[/green]")
        
        # Add network info if available
        network = vps_found.get('network')
        if network:
            network_name = network.get('name', 'N/A')
            assigned_ip = network.get('assigned_ip', 'N/A')
            net_table.add_row("Private Network", f"[yellow]{network_name}[/yellow] → [dim]{assigned_ip}[/dim]")
        
        console.print(net_table)
        
        # Access info table
        console.print()
        access_table = Table(title="Access Information", box=box.ROUNDED, show_lines=True, title_style="bold green")
        access_table.add_column("Property", style="bold")
        access_table.add_column("Value")

        access_table.add_row("OS", vps_found.get('os_name', 'N/A'))
        access_table.add_row("Username", vps_found.get('user', 'root'))

        # Check if SSH keys are attached (supports multiple keys)
        ssh_keys = vps_found.get('ssh_keys', [])
        if ssh_keys:
            ssh_key_names = [key.get('name', 'N/A') for key in ssh_keys]
            access_table.add_row("SSH Keys", ", ".join(ssh_key_names))

        console.print(access_table)

@app.command()
def destroy(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to destroy"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Destroy a VPS"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not force:
        if not confirm_action(f"Are you sure you want to destroy VPS {vps_id}? This action cannot be undone."):
            print_error("Operation cancelled")
            return
    
    client = APIClient(api_token)
    
    with with_spinner("Destroying VPS...") as progress:
        task = progress.add_task("Destroying VPS...", total=None)
        try:
            response = client.post(f"/vps/destroy/{vps_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} destruction initiated!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@power_app.command("start")
def power_start(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
):
    """Start a VPS"""
    _power_action(ctx, vps_id, "start_vps", "Starting")

@power_app.command("stop")
def power_stop(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
):
    """Stop a VPS"""
    _power_action(ctx, vps_id, "stop_vps", "Stopping")

@power_app.command("restart")
def power_restart(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
):
    """Restart a VPS"""
    _power_action(ctx, vps_id, "restart_vps", "Restarting")

@power_app.command("reset")
def power_reset(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
):
    """Force reset a VPS"""
    _power_action(ctx, vps_id, "reset_vps", "Resetting")

def _power_action(ctx: typer.Context, vps_id: int, action: str, verb: str):
    """Common power action handler"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    # Use progress bar for restart/reset actions
    if action in ["restart_vps", "reset_vps"]:
        with with_progress_bar(f"{verb} VPS...", total=100) as progress:
            task = progress.add_task(f"{verb} VPS...", total=100)
            try:
                # Send request
                progress.update(task, advance=30)
                response = client.post(f"/vps/{vps_id}/power/{action}")
                
                # Simulate progress
                import time
                progress.update(task, advance=40)
                time.sleep(0.5)
                progress.update(task, advance=30)
                progress.update(task, completed=100)
            except Exception as e:
                handle_api_exception(e, progress)
    else:
        # Use spinner for simple start/stop
        with with_spinner(f"{verb} VPS...") as progress:
            task = progress.add_task(f"{verb} VPS...", total=None)
            try:
                response = client.post(f"/vps/{vps_id}/power/{action}")
                progress.update(task, completed=True)
            except Exception as e:
                handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} {verb.lower()} command sent!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@app.command()
def resize(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to resize"),
    plan: str = typer.Option(..., "--plan", "-p", help="New plan name"),
):
    """Resize a VPS to a different plan"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not confirm_action(f"Resize VPS {vps_id} to plan {plan}? The VPS will be restarted."):
        print_error("Operation cancelled")
        return
    
    client = APIClient(api_token)
    
    with with_spinner("Resizing VPS...") as progress:
        task = progress.add_task("Resizing VPS...", total=None)
        try:
            response = client.post(f"/vps/resize/vps_id/{vps_id}/resize_plan/{plan}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} resize to {plan} initiated!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@app.command("change-password")
def change_password(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    new_password: str = typer.Option(..., "--new-password", "-p", help="New root password"),
):
    """Change VPS root password"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Changing password...") as progress:
        task = progress.add_task("Changing password...", total=None)
        try:
            response = client.post(f"/vps/{vps_id}/change-password", {"password": new_password})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"Password changed for VPS {vps_id}!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@app.command()
def reinstall(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to reinstall"),
    template: str = typer.Option(..., "--template", "-t", help="Template name (e.g., 'Debian 12')"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Reinstall VPS with a new template"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    if not force:
        if not confirm_action(f"Reinstall VPS {vps_id} with {template}? All data will be lost!"):
            print_error("Operation cancelled")
            return
    
    client = APIClient(api_token)
    
    with with_progress_bar("Reinstalling VPS...", total=100) as progress:
        task = progress.add_task("Reinstalling VPS...", total=100)
        try:
            # Simulate progress
            progress.update(task, advance=20, description="Preparing reinstall...")
            import time
            time.sleep(0.3)
            
            progress.update(task, advance=20, description="Sending request...")
            response = client.post(f"/vps/reinstall/{vps_id}", {"template_name": template})
            
            progress.update(task, advance=30, description="Wiping disk...")
            time.sleep(0.5)
            
            progress.update(task, advance=20, description="Installing OS...")
            time.sleep(0.3)
            
            progress.update(task, advance=10, description="Reinstall initiated!")
            progress.update(task, completed=100)
        except Exception as e:
            handle_api_exception(e, progress)
    
    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} reinstallation initiated with {template}!")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@plan_app.command("list")
def plan_list(ctx: typer.Context):
    """List all available VPS plans with pricing by location"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching VPS plans and pricing...") as progress:
        task = progress.add_task("Fetching VPS plans and pricing...", total=None)
        try:
            response = client.get("/pricing")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns {"vps": {"locations": [...]}}
    vps_data = response.get("vps", {})
    vps_locations = vps_data.get("locations", [])
    
    if json_output:
        print_json(vps_locations)
    else:
        if not vps_locations:
            print_error("No locations found")
            return
        
        # Group by location
        for location in vps_locations:
            # Updated structure has location_name instead of name
            location_name = location.get("location_name", "Unknown")
            display_name = location.get("description", location_name)
            
            console.print()
            console.print(f"[bold]{display_name}[/bold] ({location_name})")
            
            # Show available clusters and their plans
            clusters = location.get("clusters", [])
            for cluster in clusters:
                console.print()
                console.print(f"  [bold green]{cluster.get('cluster_name', 'Unknown Cluster')}[/bold green]")
                
                plans = cluster.get("plans", [])
                if plans:
                    table = create_table("", ["Plan", "vCPUs", "RAM", "Storage", "Bandwidth", "Price/Hour", "Price/Month"])
                    
                    for plan in plans:
                        # Convert price to float if it's a string
                        hourly_price = plan.get('price_per_hour', 0)
                        if isinstance(hourly_price, str):
                            hourly_price = float(hourly_price)
                        monthly_price = hourly_price * 24 * 30  # 30 days * 24 hours

                        table.add_row(
                            plan.get("plan_name", "N/A"),
                            str(plan.get("cpu", "N/A")),
                            f"{plan.get('ram', 0)/1024:.0f} GB" if plan.get('ram', 0) >= 1024 else f"{plan.get('ram', 0)} MB",
                            f"{plan.get('storage', 0)} GB",
                            f"{plan.get('bandwidth', 0)/1000:.0f} TB" if plan.get('bandwidth') else "N/A",
                            f"${hourly_price:.3f}",
                            f"${monthly_price:.2f}"
                        )
                    
                    console.print(table)

@template_app.command("list")
def template_list(ctx: typer.Context):
    """List all available VPS templates"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)
    
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)
    
    client = APIClient(api_token)
    
    with with_spinner("Fetching VPS templates...") as progress:
        task = progress.add_task("Fetching VPS templates...", total=None)
        try:
            response = client.get("/pricing")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)
    
    # The API returns {"vps": {"templates": [...]}}
    vps_data = response.get("vps", {})
    templates = vps_data.get("templates", [])
    
    if json_output:
        print_json(templates)
    else:
        if not templates:
            print_error("No templates found")
            return
        
        console.print()
        table = create_table("Available VPS Templates", ["Template Name", "OS", "Version"])

        # Sort templates by name
        sorted_templates = sorted(templates, key=lambda x: x.get("template_name", ""))

        for template in sorted_templates:
            table.add_row(
                template["template_name"],
                template["os_name"],
                template.get("version", "N/A")
            )

        console.print(table)
        console.print()
