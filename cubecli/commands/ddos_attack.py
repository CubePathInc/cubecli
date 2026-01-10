import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    get_context_value, with_spinner, console, handle_api_exception
)
import httpx

app = typer.Typer(no_args_is_help=True)

@app.command("list")
def list_attacks(ctx: typer.Context):
    """List recent DDoS attacks"""
    api_token = get_context_value(ctx, "api_token")
    json_output = get_context_value(ctx, "json", False)

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching DDoS attacks...") as progress:
        task = progress.add_task("Fetching DDoS attacks...", total=None)
        try:
            response = client.get("/ddos-attacks/attacks")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    # Check if response is a message (no attacks found)
    if isinstance(response, dict) and "message" in response:
        print_error(response["message"])
        return

    if json_output:
        print_json(response)
    else:
        if not response or len(response) == 0:
            print_error("No DDoS attacks found")
            return

        console.print()
        table = create_table(
            "DDoS Attacks",
            ["Attack ID", "IP Address", "Start Time", "Duration (s)", "Peak PPS", "Peak Bps", "Status", "Description"]
        )

        for attack in response:
            # Format the values
            attack_id = str(attack["attack_id"])
            ip_address = attack["ip_address"]
            start_time = attack["start_time"]
            duration = str(attack.get("duration", 0))
            pps_peak = f"{int(attack.get('packets_second_peak', 0)):,}"
            bps_peak = f"{int(attack.get('bytes_second_peak', 0)):,}"
            status = attack["status"]
            description = attack.get("description", "Unknown")

            table.add_row(
                attack_id,
                ip_address,
                start_time,
                duration,
                pps_peak,
                bps_peak,
                status,
                description
            )

        console.print(table)
