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
            # Use the dedicated floating IPs endpoint for more details
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
            columns = ["IP Address", "Type", "Status", "Server Type", "Assigned To", "DDoS Protection", "Location"]
        else:
            columns = ["IP Address", "Status", "Assigned To", "Location"]

        table = create_table("Floating IPs", columns)

        for ip in all_floating_ips:
            # Format protection type
            protection = ip.get("protection_type", "N/A")
            if protection and protection != "N/A":
                protection = protection.replace("AntiDDos_", "").replace("_", " ")

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
                    ip["status"],
                    server_type,
                    assigned_to,
                    protection,
                    ip.get("location_name", "N/A")
                )
            else:
                table.add_row(
                    ip["address"],
                    ip["status"],
                    assigned_to,
                    ip.get("location_name", "N/A")
                )

        console.print(table)