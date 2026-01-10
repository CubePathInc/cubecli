import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, format_status,
    console, with_progress_bar, handle_api_exception
)
from rich.table import Table
from rich import box
import httpx

app = typer.Typer(no_args_is_help=True)

# Power subcommand
power_app = typer.Typer(no_args_is_help=True)
app.add_typer(power_app, name="power", help="Baremetal power management")

# Reinstall subcommand
reinstall_app = typer.Typer(no_args_is_help=True)
app.add_typer(reinstall_app, name="reinstall", help="Reinstall operating system")

# Monitoring subcommand
monitoring_app = typer.Typer(no_args_is_help=True)
app.add_typer(monitoring_app, name="monitoring", help="Server monitoring management")

# Model subcommand
model_app = typer.Typer(no_args_is_help=True)
app.add_typer(model_app, name="model", help="View available baremetal models")

@app.command()
def deploy(
    ctx: typer.Context,
    project_id: int = typer.Option(..., "--project", "-p", help="Project ID"),
    location: str = typer.Option(..., "--location", "-l", help="Location name"),
    model: str = typer.Option(..., "--model", "-m", help="Server model name"),
    hostname: str = typer.Option(..., "--hostname", "-h", help="Server hostname"),
    user: str = typer.Option("root", "--user", "-u", help="Username for server access"),
    password: str = typer.Option(..., "--password", help="Password for server access"),
    label: Optional[str] = typer.Option(None, "--label", help="Server label"),
    os_name: Optional[str] = typer.Option(None, "--os", help="Operating system name"),
    disk_layout: Optional[str] = typer.Option(None, "--disk-layout", help="Disk layout name"),
    ssh_key: Optional[str] = typer.Option(None, "--ssh-key", help="SSH key name"),
):
    """Deploy a new baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "location_name": location,
        "model_name": model,
        "hostname": hostname,
        "user": user,
        "password": password,
        "label": label or hostname
    }

    if os_name:
        data["os_name"] = os_name
    if disk_layout:
        data["disk_layout_name"] = disk_layout
    if ssh_key:
        data["ssh_key_name"] = ssh_key

    with Progress(
        SpinnerColumn(spinner_name="dots", style="green"),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(bar_width=40, style="green", complete_style="green"),
        TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
        transient=True,
    ) as progress:
        from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn
        task = progress.add_task("Deploying baremetal server...", total=100)

        try:
            progress.update(task, advance=20, description="Validating request...")
            import time
            time.sleep(0.3)

            progress.update(task, advance=20, description="Processing payment...")
            response = client.post(f"/baremetal/deploy/{project_id}", data)

            progress.update(task, advance=30, description="Provisioning server...")
            time.sleep(0.5)

            progress.update(task, advance=20, description="Configuring network...")
            time.sleep(0.3)

            progress.update(task, advance=10, description="Server deployed!")
            progress.update(task, completed=100)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Baremetal server '{hostname}' deployment initiated!")
        if response.get("payment_status") == "succeeded":
            print_success("Payment processed successfully")
        elif response.get("requires_action"):
            print_error("Payment requires additional action (3D Secure)")
        print_success("Server is being provisioned. Use 'cubecli baremetal list' to check status.")

@app.command("list")
def list_baremetal(
    ctx: typer.Context,
    project_id: Optional[int] = typer.Option(None, "--project", "-p", help="Filter by project ID"),
    location_filter: Optional[str] = typer.Option(None, "--location", "-l", help="Filter by location name")
):
    """List all baremetal servers"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching baremetal servers...") as progress:
        task = progress.add_task("Fetching baremetal servers...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    project_list = response if isinstance(response, list) else response.get("projects", [])

    # Extract all baremetal from all projects
    all_baremetal = []
    for item in project_list:
        project = item.get("project", {})
        baremetal_list = item.get("baremetals", [])
        for server in baremetal_list:
            server["project_name"] = project.get("name", "N/A")
            server["project_id"] = project.get("id", "N/A")

            # Extract main IP from floating_ips (direct list)
            floating_ips = server.get("floating_ips", [])
            if floating_ips and len(floating_ips) > 0:
                ipv4_ips = [ip for ip in floating_ips if ip.get("type") == "IPv4"]
                if ipv4_ips:
                    server["main_ip"] = ipv4_ips[0]["address"]
                else:
                    server["main_ip"] = floating_ips[0]["address"]
            else:
                server["main_ip"] = "N/A"

            # Extract location
            location = server.get("location", {})
            server["location_name"] = location.get("location_name", "N/A")

            # Extract OS
            os_info = server.get("os", {})
            server["os_name"] = os_info.get("name", "N/A") if os_info else "N/A"

            # Extract Model name
            model = server.get("baremetal_model", {})
            server["model_name"] = model.get("model_name", "N/A") if model else "N/A"

            all_baremetal.append(server)

    if project_id is not None:
        all_baremetal = [server for server in all_baremetal if server.get("project_id") == project_id]

    if location_filter is not None:
        all_baremetal = [server for server in all_baremetal if server.get("location_name") == location_filter]

    if json_output:
        print_json(all_baremetal)
    else:
        if not all_baremetal:
            print_error("No baremetal servers found")
            return

        table = create_table("Baremetal Servers", ["ID", "Hostname", "Project", "Status", "IP", "Model", "OS", "Monitoring", "Location"])

        for server in all_baremetal:
            # Format monitoring status
            monitoring = server.get("monitoring_enable", False)
            monitoring_str = "✓" if monitoring else "✗"

            table.add_row(
                str(server["id"]),
                server["hostname"],
                server["project_name"],
                format_status(server.get("status", "unknown")),
                server.get("main_ip", "N/A"),
                server.get("model_name", "N/A"),
                server.get("os_name", "N/A"),
                monitoring_str,
                server.get("location_name", "N/A")
            )

        console.print()
        console.print(table)

@app.command()
def show(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Show detailed baremetal server information"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching server details...") as progress:
        task = progress.add_task("Fetching server details...", total=None)
        try:
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    project_list = response if isinstance(response, list) else response.get("projects", [])

    # Find the server
    server_found = None
    for item in project_list:
        project = item.get("project", {})
        baremetal_list = item.get("baremetals", [])
        for server in baremetal_list:
            if server.get("id") == baremetal_id:
                server["project_name"] = project.get("name", "N/A")

                # Extract IPs (direct list)
                floating_ips = server.get("floating_ips", [])
                server["floating_ips"] = floating_ips

                if floating_ips and len(floating_ips) > 0:
                    ipv4_ips = [ip for ip in floating_ips if ip.get("type") == "IPv4"]
                    if ipv4_ips:
                        server["main_ip"] = ipv4_ips[0]["address"]
                    else:
                        server["main_ip"] = floating_ips[0]["address"]
                else:
                    server["main_ip"] = "N/A"

                location = server.get("location", {})
                server["location_name"] = location.get("location_name", "N/A")

                server_found = server
                break
        if server_found:
            break

    if not server_found:
        print_error(f"Baremetal server {baremetal_id} not found")
        raise typer.Exit(1)

    if json_output:
        print_json(server_found)
    else:
        console.print()

        # Basic info table
        info_table = Table(show_header=False, box=box.ROUNDED, show_lines=True)
        info_table.add_column("Property", style="bold")
        info_table.add_column("Value")

        info_table.add_row("Hostname", server_found['hostname'])
        info_table.add_row("ID", str(server_found['id']))
        info_table.add_row("Status", format_status(server_found.get('status', 'unknown')))
        info_table.add_row("Project", server_found.get('project_name', 'N/A'))
        info_table.add_row("Location", server_found.get('location_name', 'N/A'))

        # OS info
        os_info = server_found.get('os')
        if os_info:
            info_table.add_row("Operating System", os_info.get('name', 'N/A'))

        # Monitoring
        monitoring = server_found.get('monitoring_enable', False)
        info_table.add_row("Monitoring", "[green]Enabled[/green]" if monitoring else "[red]Disabled[/red]")

        info_table.add_row("Tags", server_found.get('tags', 'N/A') or 'N/A')

        console.print(info_table)

        # Hardware specifications
        model = server_found.get('baremetal_model')
        if model:
            console.print()
            hw_table = Table(title="Hardware Specifications", box=box.ROUNDED, show_lines=True, title_style="bold cyan")
            hw_table.add_column("Component", style="bold")
            hw_table.add_column("Details", style="white")

            hw_table.add_row("CPU", model.get('cpu', 'N/A'))
            hw_table.add_row("CPU Specs", model.get('cpu_specs', 'N/A'))
            hw_table.add_row("CPU Benchmark", str(int(model.get('cpu_bench', 0))))
            hw_table.add_row("RAM", f"{model.get('ram_size', 0)} GB {model.get('ram_type', '')}")
            hw_table.add_row("Storage", f"{model.get('disk_size', 'N/A')} {model.get('disk_type', '')}")
            hw_table.add_row("Network Port", f"{model.get('port', 0)} Gbps")
            hw_table.add_row("KVM/IPMI", model.get('kvm', 'N/A'))
            hw_table.add_row("Monthly Price", f"${model.get('price', 0):.2f}")

            console.print(hw_table)

        # Network info table
        console.print()
        net_table = Table(title="Network Information", box=box.ROUNDED, show_lines=True, title_style="bold green")
        net_table.add_column("Type", style="bold cyan")
        net_table.add_column("Details", style="white")

        floating_ips = server_found.get('floating_ips', [])
        for ip in floating_ips:
            ip_type = ip.get('type', 'Unknown')
            address = ip['address']
            protection = ip.get('protection_type', 'None')

            if ip_type == 'IPv4':
                ip_display = f"[bold green]{address}[/bold green]"
                if protection and protection != 'None':
                    ip_display += f" [dim]({protection})[/dim]"
                net_table.add_row("Public IPv4", ip_display)
            elif ip_type == 'IPv6':
                ip_display = f"[green]{address}[/green]"
                if protection and protection != 'None':
                    ip_display += f" [dim]({protection})[/dim]"
                net_table.add_row("Public IPv6", ip_display)
            else:
                net_table.add_row(f"Public {ip_type}", address)

        console.print(net_table)

        # Access information
        console.print()
        access_table = Table(title="Access Information", box=box.ROUNDED, show_lines=True, title_style="bold green")
        access_table.add_column("Property", style="bold")
        access_table.add_column("Value")

        ssh_username = server_found.get('ssh_username')
        if ssh_username:
            access_table.add_row("SSH Username", ssh_username)

        ssh_key = server_found.get('ssh_key')
        if ssh_key:
            access_table.add_row("SSH Key", ssh_key.get('name', 'N/A'))

        if ssh_username or ssh_key:
            console.print(access_table)

@app.command()
def sensors(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Get BMC sensor data (temperatures, fans, etc.)"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching BMC sensor data...") as progress:
        task = progress.add_task("Fetching BMC sensor data...", total=None)
        try:
            response = client.get(f"/baremetal/{baremetal_id}/bmc-sensors")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        console.print(f"[bold]BMC Sensor Data for Node: {response.get('node', 'N/A')}[/bold]")
        console.print()

        # Status info
        status_table = Table(show_header=False, box=box.ROUNDED)
        status_table.add_column("Property", style="bold")
        status_table.add_column("Value")

        ipmi_available = response.get('ipmi_available')
        power_on = response.get('power_on')

        status_table.add_row("IPMI Available", "[green]Yes[/green]" if ipmi_available else "[red]No[/red]")
        status_table.add_row("Power Status", "[green]On[/green]" if power_on else "[red]Off[/red]")

        console.print(status_table)

        sensors = response.get('sensors', {})

        # Temperature sensors
        temperatures = sensors.get('temperatures', [])
        if temperatures:
            console.print()
            temp_table = Table(title="Temperatures", box=box.ROUNDED, title_style="bold yellow")
            temp_table.add_column("Sensor", style="bold")
            temp_table.add_column("Value", style="yellow", justify="right")

            for sensor in temperatures:
                name = sensor.get('name', 'Unknown')
                value = sensor.get('value', 0)
                temp_table.add_row(name, f"{value}°C")

            console.print(temp_table)

        # Fan sensors
        fans = sensors.get('fans', [])
        if fans:
            console.print()
            fan_table = Table(title="Fans", box=box.ROUNDED, title_style="bold cyan")
            fan_table.add_column("Fan", style="bold")
            fan_table.add_column("Speed", style="cyan", justify="right")

            for sensor in fans:
                name = sensor.get('name', 'Unknown')
                value = sensor.get('value', 0)
                fan_table.add_row(name, f"{value} RPM")

            console.print(fan_table)

@power_app.command("start")
def power_start(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Start a baremetal server"""
    _power_action(ctx, baremetal_id, "start_metal", "Starting")

@power_app.command("stop")
def power_stop(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Stop a baremetal server"""
    _power_action(ctx, baremetal_id, "stop_metal", "Stopping")

@power_app.command("restart")
def power_restart(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Restart a baremetal server"""
    _power_action(ctx, baremetal_id, "restart_metal", "Restarting")

def _power_action(ctx: typer.Context, baremetal_id: int, action: str, verb: str):
    """Common power action handler"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    # Use progress bar for restart, spinner for start/stop
    if action == "restart_metal":
        with with_progress_bar(f"{verb} server...", total=100) as progress:
            task = progress.add_task(f"{verb} server...", total=100)
            try:
                progress.update(task, advance=30)
                response = client.post(f"/baremetal/{baremetal_id}/power/{action}")

                import time
                progress.update(task, advance=40)
                time.sleep(0.5)
                progress.update(task, advance=30)
                progress.update(task, completed=100)
            except Exception as e:
                handle_api_exception(e, progress)
    else:
        with with_spinner(f"{verb} server...") as progress:
            task = progress.add_task(f"{verb} server...", total=None)
            try:
                response = client.post(f"/baremetal/{baremetal_id}/power/{action}")
                progress.update(task, completed=True)
            except Exception as e:
                handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Baremetal server {baremetal_id} {verb.lower()} command sent!")

@reinstall_app.command("start")
def reinstall_start(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
    os_name: str = typer.Option(..., "--os", help="Operating system name"),
    hostname: str = typer.Option(..., "--hostname", help="Server hostname"),
    user: str = typer.Option("root", "--user", help="Username"),
    password: str = typer.Option(..., "--password", help="Password"),
    disk_layout: Optional[str] = typer.Option(None, "--disk-layout", help="Disk layout name"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Start OS reinstallation on a baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Reinstall OS on server {baremetal_id}? All data will be lost!"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    data = {
        "os_name": os_name,
        "hostname": hostname,
        "user": user,
        "password": password
    }

    if disk_layout:
        data["disk_layout_name"] = disk_layout

    with with_progress_bar("Reinstalling OS...", total=100) as progress:
        task = progress.add_task("Reinstalling OS...", total=100)
        try:
            progress.update(task, advance=20, description="Preparing reinstall...")
            import time
            time.sleep(0.3)

            progress.update(task, advance=20, description="Sending request...")
            response = client.post(f"/baremetal/{baremetal_id}/reinstall", data)

            progress.update(task, advance=30, description="Configuring installation...")
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
        print_success(f"OS reinstallation initiated on server {baremetal_id}!")

@reinstall_app.command("status")
def reinstall_status(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Check reinstallation status"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Checking reinstall status...") as progress:
        task = progress.add_task("Checking reinstall status...", total=None)
        try:
            response = client.get(f"/baremetal/{baremetal_id}/reinstall/status")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        if response.get("is_reinstalling"):
            status_table = Table(show_header=False, box=box.ROUNDED)
            status_table.add_column("Property", style="bold")
            status_table.add_column("Value")

            status_table.add_row("Reinstalling", "[yellow]Yes[/yellow]")
            status_table.add_row("Status", response.get("status", "N/A"))
            status_table.add_row("OS", response.get("os_name", "N/A"))

            console.print(status_table)
        else:
            print_success("No active reinstallation in progress")

@app.command()
def rescue(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Enable rescue mode on a baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Activating rescue mode...") as progress:
        task = progress.add_task("Activating rescue mode...", total=None)
        try:
            response = client.post(f"/baremetal/{baremetal_id}/rescue")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        print_success(response.get("detail", "Rescue mode activated!"))

        # Display credentials
        creds_table = Table(title="Rescue Mode Credentials", box=box.ROUNDED, title_style="bold yellow")
        creds_table.add_column("Property", style="bold")
        creds_table.add_column("Value", style="green")

        creds_table.add_row("Username", response.get("username", "N/A"))
        creds_table.add_row("Password", response.get("password", "N/A"))

        console.print()
        console.print(creds_table)
        console.print()
        console.print("[yellow]⚠ Save these credentials! They won't be shown again.[/yellow]")

@app.command("reset-bmc")
def reset_bmc(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
):
    """Reset the BMC (Baseboard Management Controller)"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Reset BMC for server {baremetal_id}? The BMC will be unavailable for 1-2 minutes."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Resetting BMC...") as progress:
        task = progress.add_task("Resetting BMC...", total=None)
        try:
            response = client.post(f"/baremetal/{baremetal_id}/reset-bmc")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("BMC reset initiated successfully!")
        console.print("[yellow]⚠ The BMC will be unavailable for 1-2 minutes.[/yellow]")

@app.command()
def update(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
    hostname: Optional[str] = typer.Option(None, "--hostname", help="New hostname"),
    tags: Optional[str] = typer.Option(None, "--tags", help="New tags"),
):
    """Update baremetal server details (hostname and/or tags)"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not hostname and not tags:
        print_error("At least one of --hostname or --tags must be provided")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {}
    if hostname:
        data["hostname"] = hostname
    if tags:
        data["tags"] = tags

    with with_spinner("Updating server...") as progress:
        task = progress.add_task("Updating server...", total=None)
        try:
            response = client.patch(f"/baremetal/update/{baremetal_id}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Server {baremetal_id} updated successfully!")

@monitoring_app.command("enable")
def monitoring_enable(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Enable monitoring for a baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Enabling monitoring...") as progress:
        task = progress.add_task("Enabling monitoring...", total=None)
        try:
            response = client.put(f"/baremetal/{baremetal_id}/monitoring?enable=true")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Monitoring enabled for server {baremetal_id}!")

@monitoring_app.command("disable")
def monitoring_disable(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Disable monitoring for a baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Disabling monitoring...") as progress:
        task = progress.add_task("Disabling monitoring...", total=None)
        try:
            response = client.put(f"/baremetal/{baremetal_id}/monitoring?enable=false")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Monitoring disabled for server {baremetal_id}!")

@monitoring_app.command("status")
def monitoring_status(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Check monitoring status for a baremetal server"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Checking monitoring status...") as progress:
        task = progress.add_task("Checking monitoring status...", total=None)
        try:
            # Get server details
            response = client.get("/projects/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    project_list = response if isinstance(response, list) else response.get("projects", [])

    # Find the server
    server_found = None
    for item in project_list:
        baremetal_list = item.get("baremetals", [])
        for server in baremetal_list:
            if server.get("id") == baremetal_id:
                server_found = server
                break
        if server_found:
            break

    if not server_found:
        print_error(f"Baremetal server {baremetal_id} not found")
        raise typer.Exit(1)

    monitoring = server_found.get('monitoring_enable', False)

    if json_output:
        print_json({"baremetal_id": baremetal_id, "monitoring_enabled": monitoring})
    else:
        console.print()
        status_table = Table(show_header=False, box=box.ROUNDED)
        status_table.add_column("Property", style="bold")
        status_table.add_column("Value")

        status_table.add_row("Server ID", str(baremetal_id))
        status_table.add_row("Hostname", server_found['hostname'])
        status_table.add_row("Monitoring", "[green]Enabled[/green]" if monitoring else "[red]Disabled[/red]")

        console.print(status_table)

@app.command()
def ipmi(
    ctx: typer.Context,
    baremetal_id: int = typer.Argument(..., help="Baremetal server ID"),
):
    """Get IPMI console access credentials"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Creating IPMI session...") as progress:
        task = progress.add_task("Creating IPMI session...", total=None)
        try:
            response = client.post(f"/ipmi-proxy/create-session/{baremetal_id}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        console.print(f"[bold green]IPMI Console Access Created[/bold green]")
        console.print()

        # Display access credentials in a table
        creds_table = Table(title="IPMI Access Credentials", box=box.ROUNDED, title_style="bold cyan")
        creds_table.add_column("Property", style="bold")
        creds_table.add_column("Value", style="green")

        credentials = response.get("credentials", {})

        creds_table.add_row("Console URL", response.get("proxy_url", "N/A"))
        creds_table.add_row("Username", credentials.get("username", "N/A"))
        creds_table.add_row("Password", credentials.get("password", "N/A"))

        console.print(creds_table)
        console.print()
        console.print("[yellow]⚠ These credentials are temporary and will expire in 4 hours.[/yellow]")
        console.print("[dim]💡 Access the IPMI console by opening the Console URL in your browser.[/dim]")

@model_app.command("list")
def list_models(
    ctx: typer.Context,
    in_stock: bool = typer.Option(False, "--in-stock", help="Show only models with stock available"),
    out_of_stock: bool = typer.Option(False, "--out-of-stock", help="Show only models out of stock"),
):
    """List available baremetal models with pricing by location"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching baremetal models...") as progress:
        task = progress.add_task("Fetching baremetal models...", total=None)
        try:
            response = client.get("/pricing")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    # Extract baremetal locations
    baremetal_data = response.get("baremetal", {})
    baremetal_locations = baremetal_data.get("locations", [])

    if json_output:
        print_json(baremetal_locations)
    else:
        if not baremetal_locations:
            print_error("No baremetal models found")
            return

        # Display models grouped by location
        for location in baremetal_locations:
            location_name = location.get("location_name", "Unknown")
            description = location.get("description", "")
            models = location.get("baremetal_models", [])

            if not models:
                continue

            console.print()
            console.print(f"[bold]{description}[/bold] ({location_name})")
            console.print()

            table = create_table("", ["Model", "CPU", "RAM", "Storage", "Network", "Price/Month", "Setup", "Stock"])

            models_added = 0
            for model in models:
                stock = model.get("stock_available", 0)

                # Apply stock filters
                if in_stock and stock == 0:
                    continue
                if out_of_stock and stock > 0:
                    continue

                # Calculate monthly price (assuming hourly is stored)
                monthly_price = float(model.get("price", 0))
                setup_fee = float(model.get("setup", 0))
                stock_str = f"{stock} available" if stock > 0 else "Out of stock"

                table.add_row(
                    model.get("model_name", "N/A"),
                    model.get("cpu", "N/A"),
                    f"{model.get('ram_size', 0)} GB {model.get('ram_type', '')}",
                    f"{model.get('disk_size', 'N/A')} {model.get('disk_type', '')}",
                    f"{model.get('port', 0)} Gbps",
                    f"${monthly_price:.2f}",
                    f"${setup_fee:.2f}",
                    stock_str
                )
                models_added += 1

            if models_added > 0:
                console.print(table)
