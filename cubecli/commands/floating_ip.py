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
def list_floating_ips(
    ctx: typer.Context,
    server_type: Optional[str] = typer.Option(None, "--type", "-t", help="Filter by server type (vps or baremetal)"),
    location: Optional[str] = typer.Option(None, "--location", "-l", help="Filter by location"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all floating IPs"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    # Validate server_type if provided
    if server_type and server_type.lower() not in ["vps", "baremetal"]:
        print_error("Invalid server type. Use 'vps' or 'baremetal'")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching floating IPs...") as progress:
        task = progress.add_task("Fetching floating IPs...", total=None)
        try:
            response = client.get("/floating_ips/organization")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    # Extract single IPs and subnet IPs
    single_ips = response.get("single_ips", [])
    subnets = response.get("subnets", [])

    # Combine all IPs into one list with type indicator
    all_floating_ips = []

    # Add single IPs
    for ip in single_ips:
        ip_info = ip.copy()
        ip_info["ip_type"] = "Single"
        all_floating_ips.append(ip_info)

    # Add subnet IPs (expand them)
    for subnet in subnets:
        subnet_ips = subnet.get("ip_addresses", [])
        for ip in subnet_ips:
            ip_info = ip.copy()
            ip_info["ip_type"] = f"Subnet /{subnet.get('prefix', 'N/A')}"
            ip_info["protection_type"] = subnet.get("protection_type", "N/A")
            all_floating_ips.append(ip_info)

    # Apply filters
    filtered_ips = all_floating_ips

    if server_type:
        server_type_normalized = server_type.lower()
        filtered_ips = []
        for ip in all_floating_ips:
            vps_name = ip.get("vps_name")
            baremetal_name = ip.get("baremetal_name")

            if server_type_normalized == "vps" and vps_name:
                filtered_ips.append(ip)
            elif server_type_normalized == "baremetal" and baremetal_name:
                filtered_ips.append(ip)

    if location:
        location_normalized = location.lower()
        filtered_ips = [
            ip for ip in filtered_ips
            if ip.get("location_name", "").lower() == location_normalized
        ]

    all_floating_ips = filtered_ips

    if json_output:
        print_json(all_floating_ips)
    else:
        if not all_floating_ips:
            print_error("No floating IPs found")
            return

        console.print()

        # Define columns based on verbose flag
        if verbose:
            columns = ["IP Address", "Type", "Role", "Status", "Server Type", "Assigned To", "DDoS Protection", "Location"]
        else:
            columns = ["IP Address", "Role", "Status", "Assigned To", "Location"]

        table = create_table("Floating IPs", columns)

        for ip in all_floating_ips:
            # Format protection type
            protection = ip.get("protection_type", "N/A")
            if protection and protection != "N/A":
                protection = protection.replace("AntiDDos_", "").replace("_", " ")

            # Primary or Secondary
            is_primary = ip.get("is_primary")
            if is_primary is True:
                role = "[green]Primary[/green]"
            elif is_primary is False:
                role = "[dim]Secondary[/dim]"
            else:
                role = "-"

            # Get assigned server (VPS or Baremetal)
            assigned_to = "-"
            server_type = "-"
            vps_name = ip.get("vps_name")
            baremetal_name = ip.get("baremetal_name")

            if vps_name:
                assigned_to = vps_name
                server_type = "VPS"
            elif baremetal_name:
                assigned_to = baremetal_name
                server_type = "Baremetal"

            if verbose:
                table.add_row(
                    ip["address"],
                    ip["ip_type"],
                    role,
                    ip["status"],
                    server_type,
                    assigned_to,
                    protection,
                    ip.get("location_name", "N/A")
                )
            else:
                table.add_row(
                    ip["address"],
                    role,
                    ip["status"],
                    assigned_to,
                    ip.get("location_name", "N/A")
                )

        console.print(table)

@app.command()
def acquire(
    ctx: typer.Context,
    ip_type: str = typer.Option("IPv4", "--type", "-t", help="IP type: IPv4 or IPv6"),
    location: str = typer.Option(..., "--location", "-l", help="Location name (e.g., us-mia-1)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Acquire a new floating IP from the pool"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    # Normalize to API expected format
    ip_type_map = {"ipv4": "IPv4", "ipv6": "IPv6", "ipv4": "IPv4", "ipv6": "IPv6"}
    normalized = ip_type_map.get(ip_type.lower())
    if not normalized:
        print_error("Invalid IP type. Use 'IPv4' or 'IPv6'")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Acquiring floating IP...") as progress:
        task = progress.add_task("Acquiring floating IP...", total=None)
        try:
            response = client.post(
                "/floating_ips/acquire",
                params={"ip_type": normalized, "location_name": location}
            )
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        address = response.get("address", "N/A")
        print_success(f"Floating IP acquired: {address}")

@app.command()
def release(
    ctx: typer.Context,
    address: str = typer.Argument(..., help="IP address to release"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Release a floating IP back to the pool"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Release floating IP {address}? It will be returned to the pool."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Releasing floating IP...") as progress:
        task = progress.add_task("Releasing floating IP...", total=None)
        try:
            response = client.post(f"/floating_ips/release/{address}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Floating IP {address} released successfully!")

@app.command()
def assign(
    ctx: typer.Context,
    address: str = typer.Argument(..., help="Floating IP address to assign"),
    vps_id: Optional[int] = typer.Option(None, "--vps", help="VPS ID to assign the IP to"),
    baremetal_id: Optional[int] = typer.Option(None, "--baremetal", help="Baremetal server ID to assign the IP to"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Assign a floating IP to a VPS or baremetal server"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not vps_id and not baremetal_id:
        print_error("You must specify either --vps or --baremetal")
        raise typer.Exit(1)

    if vps_id and baremetal_id:
        print_error("You can only assign to one target: --vps or --baremetal")
        raise typer.Exit(1)

    client = APIClient(api_token)

    if vps_id:
        endpoint = f"/floating_ips/assign/vps/{vps_id}"
        target = f"VPS {vps_id}"
    else:
        endpoint = f"/floating_ips/assign/baremetal/{baremetal_id}"
        target = f"Baremetal {baremetal_id}"

    with with_spinner(f"Assigning {address} to {target}...") as progress:
        task = progress.add_task(f"Assigning {address} to {target}...", total=None)
        try:
            response = client.post(endpoint, params={"address": address})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Floating IP {address} assigned to {target}!")

@app.command()
def unassign(
    ctx: typer.Context,
    address: str = typer.Argument(..., help="Floating IP address to unassign"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Unassign a floating IP from its current server"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Unassign floating IP {address} from its current server?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Unassigning floating IP...") as progress:
        task = progress.add_task("Unassigning floating IP...", total=None)
        try:
            response = client.post(f"/floating_ips/unassign/{address}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Floating IP {address} unassigned successfully!")

@app.command("reverse-dns")
def reverse_dns(
    ctx: typer.Context,
    ip: str = typer.Argument(..., help="Floating IP address"),
    hostname: str = typer.Option("", "--hostname", "-r", help="Reverse DNS hostname (leave empty to delete)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Configure reverse DNS (PTR record) for a floating IP"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Configuring reverse DNS...") as progress:
        task = progress.add_task("Configuring reverse DNS...", total=None)
        try:
            response = client.post(
                "/floating_ips/reverse_dns/configure",
                params={"ip": ip, "reverse_dns": hostname}
            )
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        if hostname:
            print_success(f"Reverse DNS for {ip} set to {hostname}")
        else:
            print_success(f"Reverse DNS for {ip} removed")
