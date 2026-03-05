import typer
from typing import Optional, List
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, console, handle_api_exception
)

app = typer.Typer(no_args_is_help=True)


def _format_bytes(b):
    if b >= 1_073_741_824:
        return f"{b / 1_073_741_824:.2f} GB"
    elif b >= 1_048_576:
        return f"{b / 1_048_576:.2f} MB"
    elif b >= 1024:
        return f"{b / 1024:.2f} KB"
    return f"{b} B"


# Sub-command groups
zone_app = typer.Typer(no_args_is_help=True)
app.add_typer(zone_app, name="zone", help="Manage CDN zones")

origin_app = typer.Typer(no_args_is_help=True)
app.add_typer(origin_app, name="origin", help="Manage CDN origins")

rule_app = typer.Typer(no_args_is_help=True)
app.add_typer(rule_app, name="rule", help="Manage CDN edge rules")

waf_app = typer.Typer(no_args_is_help=True)
app.add_typer(waf_app, name="waf", help="Manage CDN WAF rules")

metrics_app = typer.Typer(no_args_is_help=True)
app.add_typer(metrics_app, name="metrics", help="View CDN metrics and analytics")

plan_app = typer.Typer(no_args_is_help=True)
app.add_typer(plan_app, name="plan", help="Manage CDN plans")

# ─── PLANS ───────────────────────────────────────────────────────────────────

@plan_app.command("list")
def list_plans(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List available CDN plans"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching CDN plans...") as progress:
        task = progress.add_task("Fetching CDN plans...", total=None)
        try:
            response = client.get("/cdn/plans")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        plans = response if isinstance(response, list) else response.get("plans", [])
        if not plans:
            print_error("No CDN plans found")
            return

        console.print()
        table = create_table("CDN Plans", ["Name", "Base Price/hr", "Max Zones", "Max Origins", "Custom SSL"])
        for plan in plans:
            table.add_row(
                plan.get("name", "N/A"),
                f"${plan.get('base_price_per_hour', 0):.4f}",
                str(plan.get("max_zones", "N/A")),
                str(plan.get("max_origins_per_zone", "N/A")),
                "Yes" if plan.get("custom_ssl_allowed") else "No",
            )
        console.print(table)

# ─── ZONES ───────────────────────────────────────────────────────────────────

@zone_app.command("list")
def zone_list(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List all CDN zones"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching CDN zones...") as progress:
        task = progress.add_task("Fetching CDN zones...", total=None)
        try:
            response = client.get("/cdn/zones")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        zones = response if isinstance(response, list) else response.get("zones", [])
        if not zones:
            print_error("No CDN zones found")
            return

        console.print()
        table = create_table("CDN Zones", ["UUID", "Name", "Domain", "Custom Domain", "Status", "Plan"])
        for zone in zones:
            table.add_row(
                zone.get("uuid", "N/A"),
                zone.get("name", "N/A"),
                zone.get("domain", "N/A"),
                zone.get("custom_domain", "-"),
                zone.get("status", "N/A"),
                zone.get("plan_name", "N/A"),
            )
        console.print(table)

@zone_app.command("show")
def zone_show(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show CDN zone details"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching zone details...") as progress:
        task = progress.add_task("Fetching zone details...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table("CDN Zone Details", ["Property", "Value"])
        table.add_row("UUID", response.get("uuid", "N/A"))
        table.add_row("Name", response.get("name", "N/A"))
        table.add_row("Domain", response.get("domain", "N/A"))
        table.add_row("Custom Domain", response.get("custom_domain", "-"))
        table.add_row("Status", response.get("status", "N/A"))
        table.add_row("Plan", response.get("plan_name", "N/A"))
        table.add_row("SSL Type", response.get("ssl_type", "N/A"))
        table.add_row("Created", response.get("created_at", "N/A"))
        table.add_row("Updated", response.get("updated_at", "N/A"))

        origins = response.get("origins", [])
        table.add_row("Origins", str(len(origins)))

        rules = response.get("rules", [])
        table.add_row("Rules", str(len(rules)))

        console.print(table)

@zone_app.command("create")
def zone_create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="Zone name (3-32 chars, lowercase alphanumeric + hyphens)"),
    plan: str = typer.Option(..., "--plan", "-p", help="CDN plan name"),
    custom_domain: Optional[str] = typer.Option(None, "--domain", "-d", help="Custom domain"),
    project_id: Optional[int] = typer.Option(None, "--project", help="Project ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {"name": name, "plan_name": plan}
    if custom_domain:
        data["custom_domain"] = custom_domain
    if project_id:
        data["project_id"] = project_id

    with with_spinner("Creating CDN zone...") as progress:
        task = progress.add_task("Creating CDN zone...", total=None)
        try:
            response = client.post("/cdn/zones", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"CDN zone created: {response.get('name', name)} ({response.get('uuid', 'N/A')})")
        domain = response.get("domain")
        if domain:
            console.print(f"  Domain: {domain}")

@zone_app.command("update")
def zone_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="New zone name"),
    custom_domain: Optional[str] = typer.Option(None, "--domain", "-d", help="Custom domain"),
    ssl_type: Optional[str] = typer.Option(None, "--ssl-type", help="SSL type: automatic or custom"),
    certificate_uuid: Optional[str] = typer.Option(None, "--certificate", help="Custom certificate UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {}
    if name:
        data["name"] = name
    if custom_domain:
        data["custom_domain"] = custom_domain
    if ssl_type:
        data["ssl_type"] = ssl_type
    if certificate_uuid:
        data["certificate_uuid"] = certificate_uuid

    if not data:
        print_error("No update parameters provided")
        raise typer.Exit(1)

    with with_spinner("Updating CDN zone...") as progress:
        task = progress.add_task("Updating CDN zone...", total=None)
        try:
            response = client.patch(f"/cdn/zones/{zone_uuid}", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"CDN zone {zone_uuid} updated")

@zone_app.command("delete")
def zone_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete CDN zone {zone_uuid}? This action cannot be undone."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting CDN zone...") as progress:
        task = progress.add_task("Deleting CDN zone...", total=None)
        try:
            response = client.delete(f"/cdn/zones/{zone_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"CDN zone {zone_uuid} deleted")

@zone_app.command("pricing")
def zone_pricing(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show pricing information for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching pricing...") as progress:
        task = progress.add_task("Fetching pricing...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/pricing")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        print_json(response)

# ─── ORIGINS ─────────────────────────────────────────────────────────────────

@origin_app.command("list")
def origin_list(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List origins for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching origins...") as progress:
        task = progress.add_task("Fetching origins...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/origins")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        origins = response if isinstance(response, list) else response.get("origins", [])
        if not origins:
            print_error("No origins found")
            return

        console.print()
        table = create_table("CDN Origins", ["UUID", "Name", "Address", "Port", "Protocol", "Weight", "Priority", "Backup", "Health", "Enabled"])
        for origin in origins:
            health = origin.get("health_status", "N/A")
            if health == "healthy":
                health = "[green]healthy[/green]"
            elif health == "unhealthy":
                health = "[red]unhealthy[/red]"

            table.add_row(
                origin.get("uuid", "N/A"),
                origin.get("name", "N/A"),
                origin.get("address", "N/A"),
                str(origin.get("port", "N/A")),
                origin.get("protocol", "N/A"),
                str(origin.get("weight", "N/A")),
                str(origin.get("priority", "N/A")),
                "Yes" if origin.get("is_backup") else "No",
                health,
                "Yes" if origin.get("enabled", True) else "No",
            )
        console.print(table)

@origin_app.command("create")
def origin_create(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    name: str = typer.Option(..., "--name", "-n", help="Origin name"),
    origin_url: Optional[str] = typer.Option(None, "--url", "-u", help="Origin URL (auto-parsed, e.g. https://s3.example.com/bucket/)"),
    address: Optional[str] = typer.Option(None, "--address", "-a", help="Origin IP or hostname"),
    port: Optional[int] = typer.Option(None, "--port", "-p", help="Origin port (1-65535)"),
    protocol: Optional[str] = typer.Option(None, "--protocol", help="Protocol: http or https"),
    weight: int = typer.Option(100, "--weight", "-w", help="Load balancing weight (1-1000)"),
    priority: int = typer.Option(1, "--priority", help="Priority (1-100)"),
    is_backup: bool = typer.Option(False, "--backup", help="Mark as backup origin"),
    health_check_path: str = typer.Option("/health", "--health-path", help="Health check path"),
    no_health_check: bool = typer.Option(False, "--no-health-check", help="Disable health checks"),
    no_verify_ssl: bool = typer.Option(False, "--no-verify-ssl", help="Disable SSL verification"),
    host_header: Optional[str] = typer.Option(None, "--host-header", help="Custom Host header"),
    base_path: Optional[str] = typer.Option(None, "--base-path", help="Base path prefix"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new origin for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not origin_url and not address:
        print_error("You must specify either --url or --address")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {
        "name": name,
        "weight": weight,
        "priority": priority,
        "is_backup": is_backup,
        "health_check_enabled": not no_health_check,
        "health_check_path": health_check_path,
        "verify_ssl": not no_verify_ssl,
        "enabled": True,
    }
    if origin_url:
        data["origin_url"] = origin_url
    if address:
        data["address"] = address
    if port:
        data["port"] = port
    if protocol:
        data["protocol"] = protocol
    if host_header:
        data["host_header"] = host_header
    if base_path:
        data["base_path"] = base_path

    with with_spinner("Creating origin...") as progress:
        task = progress.add_task("Creating origin...", total=None)
        try:
            response = client.post(f"/cdn/zones/{zone_uuid}/origins", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Origin created: {response.get('name', name)} ({response.get('uuid', 'N/A')})")

@origin_app.command("update")
def origin_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    origin_uuid: str = typer.Argument(..., help="Origin UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="Origin name"),
    address: Optional[str] = typer.Option(None, "--address", "-a", help="Origin IP or hostname"),
    port: Optional[int] = typer.Option(None, "--port", "-p", help="Origin port"),
    protocol: Optional[str] = typer.Option(None, "--protocol", help="Protocol: http or https"),
    weight: Optional[int] = typer.Option(None, "--weight", "-w", help="Load balancing weight"),
    priority: Optional[int] = typer.Option(None, "--priority", help="Priority"),
    host_header: Optional[str] = typer.Option(None, "--host-header", help="Custom Host header"),
    base_path: Optional[str] = typer.Option(None, "--base-path", help="Base path prefix"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a CDN origin"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {}
    if name:
        data["name"] = name
    if address:
        data["address"] = address
    if port:
        data["port"] = port
    if protocol:
        data["protocol"] = protocol
    if weight is not None:
        data["weight"] = weight
    if priority is not None:
        data["priority"] = priority
    if host_header:
        data["host_header"] = host_header
    if base_path:
        data["base_path"] = base_path

    if not data:
        print_error("No update parameters provided")
        raise typer.Exit(1)

    with with_spinner("Updating origin...") as progress:
        task = progress.add_task("Updating origin...", total=None)
        try:
            response = client.patch(f"/cdn/zones/{zone_uuid}/origins/{origin_uuid}", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Origin {origin_uuid} updated")

@origin_app.command("delete")
def origin_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    origin_uuid: str = typer.Argument(..., help="Origin UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a CDN origin"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete origin {origin_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting origin...") as progress:
        task = progress.add_task("Deleting origin...", total=None)
        try:
            response = client.delete(f"/cdn/zones/{zone_uuid}/origins/{origin_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Origin {origin_uuid} deleted")

# ─── EDGE RULES ──────────────────────────────────────────────────────────────

@rule_app.command("list")
def rule_list(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List edge rules for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching edge rules...") as progress:
        task = progress.add_task("Fetching edge rules...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/rules")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        rules = response if isinstance(response, list) else response.get("rules", [])
        if not rules:
            print_error("No edge rules found")
            return

        console.print()
        table = create_table("CDN Edge Rules", ["UUID", "Name", "Type", "Priority", "Enabled"])
        for rule in rules:
            enabled = "[green]Yes[/green]" if rule.get("enabled", True) else "[red]No[/red]"
            table.add_row(
                rule.get("uuid", "N/A"),
                rule.get("name", "N/A"),
                rule.get("rule_type", "N/A"),
                str(rule.get("priority", "N/A")),
                enabled,
            )
        console.print(table)

@rule_app.command("show")
def rule_show(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="Rule UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show edge rule details"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching rule details...") as progress:
        task = progress.add_task("Fetching rule details...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/rules/{rule_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table("Edge Rule Details", ["Property", "Value"])
        table.add_row("UUID", response.get("uuid", "N/A"))
        table.add_row("Name", response.get("name", "N/A"))
        table.add_row("Type", response.get("rule_type", "N/A"))
        table.add_row("Priority", str(response.get("priority", "N/A")))
        table.add_row("Enabled", "Yes" if response.get("enabled", True) else "No")
        expires = response.get("expires_at")
        table.add_row("Expires", expires if expires else "Never")
        console.print(table)

        match_conditions = response.get("match_conditions")
        if match_conditions:
            console.print("\n[bold]Match Conditions:[/bold]")
            print_json(match_conditions)

        action_config = response.get("action_config")
        if action_config:
            console.print("\n[bold]Action Config:[/bold]")
            print_json(action_config)

@rule_app.command("create")
def rule_create(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    name: str = typer.Option(..., "--name", "-n", help="Rule name"),
    rule_type: str = typer.Option(..., "--type", "-t", help="Rule type: cache, cache_bypass, redirect, header_request, header_response"),
    priority: int = typer.Option(100, "--priority", "-p", help="Priority (1-10000)"),
    action_config: str = typer.Option(..., "--action", "-a", help="Action config as JSON string"),
    match_conditions: Optional[str] = typer.Option(None, "--match", "-m", help="Match conditions as JSON string"),
    disabled: bool = typer.Option(False, "--disabled", help="Create rule as disabled"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create an edge rule for a CDN zone"""
    import json as json_lib

    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    valid_types = ["cache", "cache_bypass", "redirect", "header_request", "header_response"]
    if rule_type not in valid_types:
        print_error(f"Invalid rule type. Valid types: {', '.join(valid_types)}")
        raise typer.Exit(1)

    try:
        action_config_parsed = json_lib.loads(action_config)
    except json_lib.JSONDecodeError:
        print_error("Invalid JSON for --action")
        raise typer.Exit(1)

    data = {
        "name": name,
        "rule_type": rule_type,
        "priority": priority,
        "action_config": action_config_parsed,
        "enabled": not disabled,
    }

    if match_conditions:
        try:
            data["match_conditions"] = json_lib.loads(match_conditions)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --match")
            raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Creating edge rule...") as progress:
        task = progress.add_task("Creating edge rule...", total=None)
        try:
            response = client.post(f"/cdn/zones/{zone_uuid}/rules", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Edge rule created: {response.get('name', name)} ({response.get('uuid', 'N/A')})")

@rule_app.command("update")
def rule_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="Rule UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="Rule name"),
    priority: Optional[int] = typer.Option(None, "--priority", "-p", help="Priority"),
    action_config: Optional[str] = typer.Option(None, "--action", "-a", help="Action config as JSON"),
    match_conditions: Optional[str] = typer.Option(None, "--match", "-m", help="Match conditions as JSON"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update an edge rule"""
    import json as json_lib

    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {}
    if name:
        data["name"] = name
    if priority is not None:
        data["priority"] = priority
    if action_config:
        try:
            data["action_config"] = json_lib.loads(action_config)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --action")
            raise typer.Exit(1)
    if match_conditions:
        try:
            data["match_conditions"] = json_lib.loads(match_conditions)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --match")
            raise typer.Exit(1)

    if not data:
        print_error("No update parameters provided")
        raise typer.Exit(1)

    with with_spinner("Updating edge rule...") as progress:
        task = progress.add_task("Updating edge rule...", total=None)
        try:
            response = client.patch(f"/cdn/zones/{zone_uuid}/rules/{rule_uuid}", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Edge rule {rule_uuid} updated")

@rule_app.command("delete")
def rule_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="Rule UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete an edge rule"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete edge rule {rule_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting edge rule...") as progress:
        task = progress.add_task("Deleting edge rule...", total=None)
        try:
            response = client.delete(f"/cdn/zones/{zone_uuid}/rules/{rule_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Edge rule {rule_uuid} deleted")

# ─── WAF RULES ───────────────────────────────────────────────────────────────

@waf_app.command("list")
def waf_list(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List WAF rules for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching WAF rules...") as progress:
        task = progress.add_task("Fetching WAF rules...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/waf-rules")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        rules = response if isinstance(response, list) else response.get("rules", [])
        if not rules:
            print_error("No WAF rules found")
            return

        console.print()
        table = create_table("CDN WAF Rules", ["UUID", "Name", "Type", "Priority", "Enabled"])
        for rule in rules:
            enabled = "[green]Yes[/green]" if rule.get("enabled", True) else "[red]No[/red]"
            table.add_row(
                rule.get("uuid", "N/A"),
                rule.get("name", "N/A"),
                rule.get("rule_type", "N/A"),
                str(rule.get("priority", "N/A")),
                enabled,
            )
        console.print(table)

@waf_app.command("show")
def waf_show(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="WAF rule UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show WAF rule details"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching WAF rule details...") as progress:
        task = progress.add_task("Fetching WAF rule details...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/waf-rules/{rule_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table("WAF Rule Details", ["Property", "Value"])
        table.add_row("UUID", response.get("uuid", "N/A"))
        table.add_row("Name", response.get("name", "N/A"))
        table.add_row("Type", response.get("rule_type", "N/A"))
        table.add_row("Priority", str(response.get("priority", "N/A")))
        table.add_row("Enabled", "Yes" if response.get("enabled", True) else "No")
        expires = response.get("expires_at")
        table.add_row("Expires", expires if expires else "Never")
        console.print(table)

        match_conditions = response.get("match_conditions")
        if match_conditions:
            console.print("\n[bold]Match Conditions:[/bold]")
            print_json(match_conditions)

        action_config = response.get("action_config")
        if action_config:
            console.print("\n[bold]Action Config:[/bold]")
            print_json(action_config)

@waf_app.command("create")
def waf_create(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    name: str = typer.Option(..., "--name", "-n", help="Rule name"),
    rule_type: str = typer.Option(..., "--type", "-t", help="WAF rule type: firewall_ip, firewall_country, firewall_ua, rate_limit, js_challenge, limit_download_speed, limit_requests, limit_connections, limit_bandwidth"),
    priority: int = typer.Option(100, "--priority", "-p", help="Priority (1-10000)"),
    action_config: str = typer.Option(..., "--action", "-a", help="Action config as JSON string"),
    match_conditions: Optional[str] = typer.Option(None, "--match", "-m", help="Match conditions as JSON string"),
    disabled: bool = typer.Option(False, "--disabled", help="Create rule as disabled"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a WAF rule for a CDN zone"""
    import json as json_lib

    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    valid_types = ["firewall_ip", "firewall_country", "firewall_ua", "rate_limit", "js_challenge",
                   "limit_download_speed", "limit_requests", "limit_connections", "limit_bandwidth"]
    if rule_type not in valid_types:
        print_error(f"Invalid WAF rule type. Valid types: {', '.join(valid_types)}")
        raise typer.Exit(1)

    try:
        action_config_parsed = json_lib.loads(action_config)
    except json_lib.JSONDecodeError:
        print_error("Invalid JSON for --action")
        raise typer.Exit(1)

    data = {
        "name": name,
        "rule_type": rule_type,
        "priority": priority,
        "action_config": action_config_parsed,
        "enabled": not disabled,
    }

    if match_conditions:
        try:
            data["match_conditions"] = json_lib.loads(match_conditions)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --match")
            raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Creating WAF rule...") as progress:
        task = progress.add_task("Creating WAF rule...", total=None)
        try:
            response = client.post(f"/cdn/zones/{zone_uuid}/waf-rules", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"WAF rule created: {response.get('name', name)} ({response.get('uuid', 'N/A')})")

@waf_app.command("update")
def waf_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="WAF rule UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="Rule name"),
    priority: Optional[int] = typer.Option(None, "--priority", "-p", help="Priority"),
    action_config: Optional[str] = typer.Option(None, "--action", "-a", help="Action config as JSON"),
    match_conditions: Optional[str] = typer.Option(None, "--match", "-m", help="Match conditions as JSON"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a WAF rule"""
    import json as json_lib

    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)
    data = {}
    if name:
        data["name"] = name
    if priority is not None:
        data["priority"] = priority
    if action_config:
        try:
            data["action_config"] = json_lib.loads(action_config)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --action")
            raise typer.Exit(1)
    if match_conditions:
        try:
            data["match_conditions"] = json_lib.loads(match_conditions)
        except json_lib.JSONDecodeError:
            print_error("Invalid JSON for --match")
            raise typer.Exit(1)

    if not data:
        print_error("No update parameters provided")
        raise typer.Exit(1)

    with with_spinner("Updating WAF rule...") as progress:
        task = progress.add_task("Updating WAF rule...", total=None)
        try:
            response = client.patch(f"/cdn/zones/{zone_uuid}/waf-rules/{rule_uuid}", data=data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"WAF rule {rule_uuid} updated")

@waf_app.command("delete")
def waf_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    rule_uuid: str = typer.Argument(..., help="WAF rule UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a WAF rule"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete WAF rule {rule_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting WAF rule...") as progress:
        task = progress.add_task("Deleting WAF rule...", total=None)
        try:
            response = client.delete(f"/cdn/zones/{zone_uuid}/waf-rules/{rule_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"WAF rule {rule_uuid} deleted")

# ─── METRICS ─────────────────────────────────────────────────────────────────

@metrics_app.command("summary")
def metrics_summary(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get metrics summary for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching metrics...") as progress:
        task = progress.add_task("Fetching metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/summary", params={"minutes": minutes})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table(f"CDN Metrics Summary (last {minutes} min)", ["Metric", "Value"])
        table.add_row("Domain", response.get("domain", "N/A"))
        table.add_row("Total Requests", f"{response.get('total_requests', 0):,}")

        table.add_row("Total Bandwidth", _format_bytes(response.get("total_bandwidth", 0)))

        cache_hit = response.get("cache_hit_rate", 0)
        table.add_row("Cache Hit Rate", f"{cache_hit:.2f}%")
        error_rate = response.get("error_rate", 0)
        table.add_row("Error Rate", f"{error_rate:.2f}%")
        console.print(table)

@metrics_app.command("requests")
def metrics_requests(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    interval: int = typer.Option(60, "--interval", help="Interval in seconds"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get requests over time for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching request metrics...") as progress:
        task = progress.add_task("Fetching request metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/requests", params={"minutes": minutes, "interval_seconds": interval})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No request data found")
            return
        console.print()
        table = create_table(f"Requests Over Time (last {minutes} min)", ["Timestamp", "Req/s"])
        for item in data:
            table.add_row(item.get("timestamp", "N/A"), f"{item.get('value', 0):.1f}")
        console.print(table)

@metrics_app.command("bandwidth")
def metrics_bandwidth(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    group_by: str = typer.Option("time", "--group-by", "-g", help="Group by: time or region"),
    interval: int = typer.Option(60, "--interval", help="Interval in seconds (for time grouping)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get bandwidth metrics for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching bandwidth metrics...") as progress:
        task = progress.add_task("Fetching bandwidth metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/bandwidth", params={"minutes": minutes, "group_by": group_by, "interval_seconds": interval})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No bandwidth data found")
            return
        console.print()
        if group_by == "region":
            table = create_table(f"Bandwidth by Region (last {minutes} min)", ["Region", "Bandwidth", "Requests"])
            for item in data:
                table.add_row(item.get("region", "N/A"), _format_bytes(item.get("bandwidth", 0)), f"{item.get('requests', 0):,}")
        else:
            table = create_table(f"Bandwidth Over Time (last {minutes} min)", ["Timestamp", "Bytes/s"])
            for item in data:
                table.add_row(item.get("timestamp", "N/A"), f"{item.get('value', 0):.1f}")
        console.print(table)

@metrics_app.command("cache")
def metrics_cache(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get cache hit/miss metrics for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching cache metrics...") as progress:
        task = progress.add_task("Fetching cache metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/cache", params={"minutes": minutes})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table(f"CDN Cache Metrics (last {minutes} min)", ["Metric", "Value"])
        table.add_row("Hits", str(response.get("hits", 0)))
        table.add_row("Misses", str(response.get("misses", 0)))
        hit_rate = response.get("hit_rate", 0)
        table.add_row("Hit Rate", f"{hit_rate:.1f}%" if isinstance(hit_rate, (int, float)) else str(hit_rate))
        console.print(table)

@metrics_app.command("status-codes")
def metrics_status_codes(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get HTTP status code distribution for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching status code metrics...") as progress:
        task = progress.add_task("Fetching status code metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/status-codes", params={"minutes": minutes})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = create_table(f"HTTP Status Codes (last {minutes} min)", ["Status", "Count"])
        table.add_row("[green]2xx[/green]", str(response.get("status_2xx", 0)))
        table.add_row("[cyan]3xx[/cyan]", str(response.get("status_3xx", 0)))
        table.add_row("[yellow]4xx[/yellow]", str(response.get("status_4xx", 0)))
        table.add_row("[red]5xx[/red]", str(response.get("status_5xx", 0)))
        console.print(table)

@metrics_app.command("top-urls")
def metrics_top_urls(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    limit: int = typer.Option(20, "--limit", "-l", help="Number of results"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get top URLs for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching top URLs...") as progress:
        task = progress.add_task("Fetching top URLs...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/top-urls", params={"minutes": minutes, "limit": limit})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No URL data found")
            return
        console.print()
        table = create_table(f"Top URLs (last {minutes} min)", ["URL", "Requests", "Bandwidth"])
        for item in data:
            table.add_row(item.get("url", "N/A"), f"{item.get('requests', 0):,}", _format_bytes(item.get("bandwidth", 0)))
        console.print(table)

@metrics_app.command("top-countries")
def metrics_top_countries(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    limit: int = typer.Option(20, "--limit", "-l", help="Number of results"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get top countries for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching top countries...") as progress:
        task = progress.add_task("Fetching top countries...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/top-countries", params={"minutes": minutes, "limit": limit})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No country data found")
            return
        console.print()
        table = create_table(f"Top Countries (last {minutes} min)", ["Code", "Country", "Requests", "Bandwidth"])
        for item in data:
            table.add_row(item.get("country_code", "N/A"), item.get("country_name", "N/A"), f"{item.get('requests', 0):,}", _format_bytes(item.get("bandwidth", 0)))
        console.print(table)

@metrics_app.command("top-asn")
def metrics_top_asn(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    limit: int = typer.Option(20, "--limit", "-l", help="Number of results"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get top ASN (Autonomous System Numbers) for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching top ASN...") as progress:
        task = progress.add_task("Fetching top ASN...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/top-asn", params={"minutes": minutes, "limit": limit})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No ASN data found")
            return
        console.print()
        table = create_table(f"Top ASN (last {minutes} min)", ["ASN", "Name", "Requests", "Bandwidth"])
        for item in data:
            table.add_row(str(item.get("asn", "N/A")), item.get("asn_name", "N/A"), f"{item.get('requests', 0):,}", _format_bytes(item.get("bandwidth", 0)))
        console.print(table)

@metrics_app.command("top-user-agents")
def metrics_top_user_agents(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    limit: int = typer.Option(20, "--limit", "-l", help="Number of results"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get top user agents for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching top user agents...") as progress:
        task = progress.add_task("Fetching top user agents...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/top-user-agents", params={"minutes": minutes, "limit": limit})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No user agent data found")
            return
        console.print()
        table = create_table(f"Top User Agents (last {minutes} min)", ["User Agent", "Requests"])
        for item in data:
            table.add_row(item.get("user_agent", "N/A"), f"{item.get('requests', 0):,}")
        console.print(table)

@metrics_app.command("blocked")
def metrics_blocked(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get blocked requests by reason for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching blocked requests...") as progress:
        task = progress.add_task("Fetching blocked requests...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/blocked", params={"minutes": minutes})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No blocked request data found")
            return
        console.print()
        table = create_table(f"Blocked Requests (last {minutes} min)", ["Reason", "Requests"])
        for item in data:
            table.add_row(item.get("reason", "N/A"), f"{item.get('requests', 0):,}")
        console.print(table)

@metrics_app.command("pops")
def metrics_pops(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get requests by PoP (Point of Presence) for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching PoP metrics...") as progress:
        task = progress.add_task("Fetching PoP metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/pops", params={"minutes": minutes})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No PoP data found")
            return
        console.print()
        table = create_table(f"Requests by PoP (last {minutes} min)", ["PoP", "Requests", "Cache Hit %"])
        for item in data:
            cache_pct = item.get("cache_hit_pct", 0)
            table.add_row(item.get("pop", "N/A"), f"{item.get('requests', 0):,}", f"{cache_pct:.1f}%")
        console.print(table)

@metrics_app.command("file-extensions")
def metrics_file_extensions(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="CDN zone UUID"),
    minutes: int = typer.Option(60, "--minutes", "-m", help="Time window in minutes"),
    limit: int = typer.Option(20, "--limit", "-l", help="Number of results"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Get top file extensions for a CDN zone"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching file extension metrics...") as progress:
        task = progress.add_task("Fetching file extension metrics...", total=None)
        try:
            response = client.get(f"/cdn/zones/{zone_uuid}/metrics/file-extensions", params={"minutes": minutes, "limit": limit})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        data = response.get("data", [])
        if not data:
            print_error("No file extension data found")
            return
        console.print()
        table = create_table(f"Top File Extensions (last {minutes} min)", ["Extension", "Requests", "Bandwidth"])
        for item in data:
            table.add_row(item.get("extension", "N/A"), f"{item.get('requests', 0):,}", _format_bytes(item.get("bandwidth", 0)))
        console.print(table)
