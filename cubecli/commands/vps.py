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
from rich.columns import Columns
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

# Backup subcommand
backup_app = typer.Typer(no_args_is_help=True)
app.add_typer(backup_app, name="backup", help="VPS backup management")

# ISO subcommand
iso_app = typer.Typer(no_args_is_help=True)
app.add_typer(iso_app, name="iso", help="Mount/unmount ISO images")

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
    ipv4: bool = typer.Option(True, "--ipv4/--no-ipv4", help="Enable IPv4 (adds $1.50/month, IPv6 always included)"),
    firewall: Optional[int] = typer.Option(None, "--firewall", "-fw", help="Firewall group ID to attach"),
    backups: bool = typer.Option(False, "--backups/--no-backups", help="Enable automatic backups"),
    cloudinit: Optional[str] = typer.Option(None, "--cloudinit", "-c", help="Cloud-init config (file path or YAML string, Linux only)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Create a new VPS"""
    api_token = get_context_value(ctx, "api_token")

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
        "label": label or name,
        "ipv4": ipv4,
        "enable_backups": backups,
    }

    if ssh_keys:
        data["ssh_key_names"] = ssh_keys
    if network_id:
        data["network_id"] = network_id
    if password:
        data["password"] = password
    if firewall:
        data["firewall_group_ids"] = [firewall]

    # Process cloud-init: can be a file path or direct YAML content
    if cloudinit:
        import os
        if os.path.isfile(cloudinit):
            try:
                with open(cloudinit, 'r') as f:
                    data["custom_cloudinit"] = f.read()
            except Exception as e:
                print_error(f"Failed to read cloud-init file: {e}")
                raise typer.Exit(1)
        else:
            data["custom_cloudinit"] = cloudinit
    
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
    location_filter: Optional[str] = typer.Option(None, "--location", "-l", help="Filter by location name"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all VPS instances"""
    api_token = get_context_value(ctx, "api_token")
    
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
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Show detailed VPS information"""
    api_token = get_context_value(ctx, "api_token")
    
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
                floating_ips_data = vps.get("floating_ips", {})
                if isinstance(floating_ips_data, dict):
                    floating_ips = floating_ips_data.get("list", [])
                else:
                    floating_ips = floating_ips_data if floating_ips_data else []

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
        
        # VPS Details table
        info_table = Table(title="VPS Details", box=box.ROUNDED, show_lines=True, title_style="bold green")
        info_table.add_column("Property", style="bold")
        info_table.add_column("Value")

        info_table.add_row("Name", vps_found['name'])
        info_table.add_row("ID", str(vps_found['id']))
        info_table.add_row("Status", format_status(vps_found.get('status', 'unknown')))
        info_table.add_row("Project", vps_found.get('project_name', 'N/A'))
        info_table.add_row("Location", vps_found.get('location_name', 'N/A'))
        info_table.add_row("OS", vps_found.get('os_name', 'N/A'))
        info_table.add_row("Username", vps_found.get('user', 'root'))

        # Check if SSH keys are attached
        ssh_keys = vps_found.get('ssh_keys', [])
        if ssh_keys:
            ssh_key_names = [key.get('name', 'N/A') for key in ssh_keys]
            info_table.add_row("SSH Keys", ", ".join(ssh_key_names))

        # Resources & Network table
        resources_table = Table(title="Resources & Network", box=box.ROUNDED, show_lines=True, title_style="bold green")
        resources_table.add_column("Property", style="bold cyan")
        resources_table.add_column("Value", style="white")

        # Add plan details
        plan = vps_found.get('plan', {})
        if plan:
            resources_table.add_row("Plan", plan.get('plan_name', 'N/A'))
            resources_table.add_row("vCPUs", str(plan.get('cpu', 'N/A')))
            resources_table.add_row("RAM", f"{plan.get('ram', 0)} MB")
            resources_table.add_row("Storage", f"{plan.get('storage', 0)} GB")
            resources_table.add_row("Bandwidth", f"{plan.get('bandwidth', 0)} GB")
            resources_table.add_row("Price/Hour", f"${plan.get('price_per_hour', 0)}")

        # Add floating IPs
        floating_ips_data = vps_found.get('floating_ips', {})
        if isinstance(floating_ips_data, dict):
            floating_ips = floating_ips_data.get("list", [])
        else:
            floating_ips = floating_ips_data if floating_ips_data else []

        for ip in floating_ips:
            ip_type = ip.get('type', 'Unknown')
            address = ip['address']
            if ip_type == 'IPv4':
                resources_table.add_row("Public IPv4", f"[bold green]{address}[/bold green]")
            elif ip_type == 'IPv6':
                resources_table.add_row("Public IPv6", f"[green]{address}[/green]")
            else:
                resources_table.add_row(f"Public {ip_type}", address)

        # Add IPv6 if available (legacy support)
        ipv6 = vps_found.get('ipv6')
        if ipv6 and not any(ip.get('type') == 'IPv6' for ip in floating_ips):
            resources_table.add_row("Public IPv6", f"[green]{ipv6}[/green]")

        # Add network info if available
        network = vps_found.get('network')
        if network:
            network_name = network.get('name', 'N/A')
            assigned_ip = network.get('assigned_ip', 'N/A')
            resources_table.add_row("Private Network", f"[yellow]{network_name}[/yellow] → [dim]{assigned_ip}[/dim]")

        # Print tables in two columns with equal width
        console.print(Columns([info_table, resources_table], equal=True, expand=True))

@app.command()
def destroy(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to destroy"),
    release_ips: bool = typer.Option(True, "--release-ips/--keep-ips", help="Release floating IPs back to pool or keep them"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Destroy a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        ip_msg = "Floating IPs will be released." if release_ips else "Floating IPs will be kept."
        if not confirm_action(f"Are you sure you want to destroy VPS {vps_id}? {ip_msg} This action cannot be undone."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Destroying VPS...") as progress:
        task = progress.add_task("Destroying VPS...", total=None)
        try:
            response = client.post(f"/vps/destroy/{vps_id}", {"release_ips": release_ips})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} destruction initiated!")
        if not release_ips:
            print_success("Floating IPs have been kept in your account.")
        if "task_id" in response:
            print_success(f"Task ID: {response['task_id']}")

@app.command()
def update(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID to update"),
    label: Optional[str] = typer.Option(None, "--label", help="New VPS label"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="New VPS hostname"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update VPS details (label and/or hostname)"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not label and not name:
        print_error("At least one of --label or --name must be provided")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {}
    if label:
        data["label"] = label
    if name:
        data["name"] = name

    with with_spinner("Updating VPS...") as progress:
        task = progress.add_task("Updating VPS...", total=None)
        try:
            response = client.patch(f"/vps/update/{vps_id}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"VPS {vps_id} updated successfully!")

@power_app.command("start")
def power_start(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Start a VPS"""
    _power_action(ctx, vps_id, "start_vps", "Starting", json_output)

@power_app.command("stop")
def power_stop(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Stop a VPS"""
    _power_action(ctx, vps_id, "stop_vps", "Stopping", json_output)

@power_app.command("restart")
def power_restart(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Restart a VPS"""
    _power_action(ctx, vps_id, "restart_vps", "Restarting", json_output)

@power_app.command("reset")
def power_reset(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Force reset a VPS"""
    _power_action(ctx, vps_id, "reset_vps", "Resetting", json_output)

def _power_action(ctx: typer.Context, vps_id: int, action: str, verb: str, json_output: bool = False):
    """Common power action handler"""
    api_token = get_context_value(ctx, "api_token")
    
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
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Resize a VPS to a different plan"""
    api_token = get_context_value(ctx, "api_token")
    
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
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Change VPS root password"""
    api_token = get_context_value(ctx, "api_token")
    
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
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """Reinstall VPS with a new template"""
    api_token = get_context_value(ctx, "api_token")
    
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
def plan_list(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all available VPS plans with pricing by location"""
    api_token = get_context_value(ctx, "api_token")
    
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
def template_list(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all available VPS templates"""
    api_token = get_context_value(ctx, "api_token")
    
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


# ── Backup commands ────────────────────────────────────────────

@backup_app.command("list")
def backup_list(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List all backups for a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching backups...") as progress:
        task = progress.add_task("Fetching backups...", total=None)
        try:
            response = client.get(f"/vps/{vps_id}/backups")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        backups = response.get("backups", [])

        if not backups:
            print_error("No backups found")
            return

        console.print()
        table = create_table("Backups", ["ID", "Type", "Status", "Progress", "Size", "Notes", "Created"])

        for backup in backups:
            size = backup.get("size_gb")
            size_str = f"{size:.2f} GB" if size else "-"
            progress_val = backup.get("progress", 0)
            progress_str = f"{progress_val}%" if backup.get("status") == "in_progress" else "-"
            created = backup.get("created_at", "N/A")
            if isinstance(created, str) and "T" in created:
                created = created.split("T")[0]

            table.add_row(
                str(backup.get("id", "N/A")),
                backup.get("backup_type", "N/A"),
                _format_backup_status(backup.get("status", "unknown")),
                progress_str,
                size_str,
                backup.get("notes", "-") or "-",
                created,
            )

        console.print(table)

        # Show settings summary
        settings = response.get("settings")
        if settings:
            enabled = settings.get("enabled", False)
            console.print()
            if enabled:
                console.print(f"[green]Auto-backup enabled[/green] — Schedule: {settings.get('schedule_hour', 0):02d}:00 UTC, Retention: {settings.get('retention_days', 7)} days, Max: {settings.get('max_backups', 3)}")
            else:
                console.print("[dim]Auto-backup disabled[/dim]")


@backup_app.command("create")
def backup_create(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    notes: Optional[str] = typer.Option(None, "--notes", "-n", help="Backup notes (max 500 chars)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a manual backup"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {"notes": notes} if notes else {"notes": None}

    with with_spinner("Creating backup...") as progress:
        task = progress.add_task("Creating backup...", total=None)
        try:
            response = client.post(f"/vps/{vps_id}/backups", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Backup created for VPS {vps_id}!")
        backup_id = response.get("id")
        if backup_id:
            print_success(f"Backup ID: {backup_id}")


@backup_app.command("restore")
def backup_restore(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    backup_id: int = typer.Argument(..., help="Backup ID to restore"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Restore a VPS from a backup"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Restore VPS {vps_id} from backup {backup_id}? Current data will be overwritten!"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Restoring from backup...") as progress:
        task = progress.add_task("Restoring from backup...", total=None)
        try:
            response = client.post(f"/vps/{vps_id}/backups/{backup_id}/restore", {"confirm": True})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Restore initiated from backup {backup_id}!")


@backup_app.command("delete")
def backup_delete(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    backup_id: int = typer.Argument(..., help="Backup ID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a backup"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete backup {backup_id}? This cannot be undone."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting backup...") as progress:
        task = progress.add_task("Deleting backup...", total=None)
        try:
            response = client.delete(f"/vps/{vps_id}/backups/{backup_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Backup {backup_id} deleted!")


@backup_app.command("settings")
def backup_settings(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show backup settings for a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching backup settings...") as progress:
        task = progress.add_task("Fetching backup settings...", total=None)
        try:
            response = client.get(f"/vps/{vps_id}/backup/settings")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = Table(title="Backup Settings", show_header=False, box=box.ROUNDED, show_lines=True, title_style="bold green")
        table.add_column("Property", style="bold")
        table.add_column("Value")

        enabled = response.get("enabled", False)
        table.add_row("Enabled", "[green]Yes[/green]" if enabled else "[red]No[/red]")
        table.add_row("Schedule", f"{response.get('schedule_hour', 0):02d}:00 UTC")
        table.add_row("Retention", f"{response.get('retention_days', 7)} days")
        table.add_row("Max Backups", str(response.get("max_backups", 3)))

        console.print(table)


@backup_app.command("configure")
def backup_configure(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    enabled: bool = typer.Option(..., "--enable/--disable", help="Enable or disable auto-backups"),
    schedule_hour: int = typer.Option(3, "--hour", help="Hour to run backups (0-23 UTC)"),
    retention_days: int = typer.Option(7, "--retention", help="Days to keep backups (1-7)"),
    max_backups: int = typer.Option(3, "--max", help="Maximum number of backups (1-10)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Configure automatic backup settings"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "enabled": enabled,
        "schedule_hour": schedule_hour,
        "retention_days": retention_days,
        "max_backups": max_backups,
    }

    with with_spinner("Updating backup settings...") as progress:
        task = progress.add_task("Updating backup settings...", total=None)
        try:
            response = client.put(f"/vps/{vps_id}/backup/settings", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        if enabled:
            print_success(f"Auto-backups enabled for VPS {vps_id} — {schedule_hour:02d}:00 UTC, {retention_days} days retention, max {max_backups}")
        else:
            print_success(f"Auto-backups disabled for VPS {vps_id}")


# ── ISO commands ───────────────────────────────────────────────

@iso_app.command("list")
def iso_list(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List available ISOs for a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching ISOs...") as progress:
        task = progress.add_task("Fetching ISOs...", total=None)
        try:
            response = client.get(f"/vps/{vps_id}/isos")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        items = response.get("items", [])
        mounted_id = response.get("mounted_iso_id")

        if not items:
            print_error("No ISOs available")
            return

        console.print()
        table = create_table("Available ISOs", ["ID", "Name", "Size", "Status"])

        for iso in items:
            is_mounted = iso.get("is_mounted", False)
            status_str = "[green]Mounted[/green]" if is_mounted else "[dim]Available[/dim]"
            size = iso.get("file_size", 0)
            size_str = f"{size / (1024**3):.1f} GB" if size > 0 else "-"

            table.add_row(
                iso.get("id", "N/A"),
                iso.get("name", "N/A"),
                size_str,
                status_str,
            )

        console.print(table)


@iso_app.command("mount")
def iso_mount(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    iso_id: str = typer.Argument(..., help="ISO UUID to mount"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Mount an ISO image to a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Mounting ISO...") as progress:
        task = progress.add_task("Mounting ISO...", total=None)
        try:
            response = client.post(f"/vps/{vps_id}/iso", {"iso_id": iso_id})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        iso_name = response.get("iso_name", iso_id)
        print_success(f"ISO '{iso_name}' mounted on VPS {vps_id}!")


@iso_app.command("unmount")
def iso_unmount(
    ctx: typer.Context,
    vps_id: int = typer.Argument(..., help="VPS ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Unmount the current ISO from a VPS"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Unmounting ISO...") as progress:
        task = progress.add_task("Unmounting ISO...", total=None)
        try:
            response = client.delete(f"/vps/{vps_id}/iso")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"ISO unmounted from VPS {vps_id}!")


def _format_backup_status(status: str) -> str:
    colors = {
        "completed": "green",
        "in_progress": "yellow",
        "pending": "dim",
        "failed": "red",
        "deleted": "dim",
    }
    color = colors.get(status.lower(), "white")
    return f"[{color}]{status}[/{color}]"
