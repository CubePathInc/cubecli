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
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Enable verbose output"),
):
    """List all floating IPs"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
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

    if json_output:
        print_json(all_floating_ips)
    else:
        if not all_floating_ips:
            print_error("No floating IPs found")
            return

        console.print()
        table = create_table(
            "Floating IPs",
            ["IP Address", "Type", "Status", "Server Type", "Assigned To", "DDoS Protection", "Location"]
        )

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

            table.add_row(
                ip["address"],
                ip["ip_type"],
                ip["status"],
                server_type,
                assigned_to,
                protection,
                ip.get("location_name", "N/A")
            )

        console.print(table)