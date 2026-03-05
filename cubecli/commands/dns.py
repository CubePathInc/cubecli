import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, console, handle_api_exception
)
from rich.table import Table
from rich import box

app = typer.Typer(no_args_is_help=True)

# Zone subcommands
zone_app = typer.Typer(no_args_is_help=True)
app.add_typer(zone_app, name="zone", help="Manage DNS zones")

# Record subcommands
record_app = typer.Typer(no_args_is_help=True)
app.add_typer(record_app, name="record", help="Manage DNS records")

# SOA subcommands
soa_app = typer.Typer(no_args_is_help=True)
app.add_typer(soa_app, name="soa", help="Manage SOA settings")


# ── Zone commands ──────────────────────────────────────────────

@zone_app.command("list")
def zone_list(
    ctx: typer.Context,
    project_id: Optional[int] = typer.Option(None, "--project", "-p", help="Filter by project ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List all DNS zones"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    params = {}
    if project_id is not None:
        params["project_id"] = project_id

    with with_spinner("Fetching DNS zones...") as progress:
        task = progress.add_task("Fetching DNS zones...", total=None)
        try:
            response = client.get("/dns/zones", params=params or None)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    zones = response if isinstance(response, list) else []

    if json_output:
        print_json(zones)
    else:
        if not zones:
            print_error("No DNS zones found")
            return

        console.print()
        table = create_table("DNS Zones", ["UUID", "Domain", "Status", "Records", "Nameservers"])

        for zone in zones:
            nameservers = ", ".join(zone.get("nameservers", []))
            table.add_row(
                zone.get("uuid", "N/A"),
                zone.get("domain", "N/A"),
                _format_zone_status(zone.get("status", "unknown")),
                str(zone.get("records_count", 0)),
                nameservers or "N/A",
            )

        console.print(table)


@zone_app.command("show")
def zone_show(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show DNS zone details"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching zone details...") as progress:
        task = progress.add_task("Fetching zone details...", total=None)
        try:
            zone = client.get(f"/dns/zones/{zone_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(zone)
    else:
        console.print()
        info_table = Table(show_header=False, box=box.ROUNDED, show_lines=True)
        info_table.add_column("Property", style="bold")
        info_table.add_column("Value")

        info_table.add_row("Domain", zone.get("domain", "N/A"))
        info_table.add_row("UUID", zone.get("uuid", "N/A"))
        info_table.add_row("Status", _format_zone_status(zone.get("status", "unknown")))
        info_table.add_row("Records", str(zone.get("records_count", 0)))
        info_table.add_row("DNS Tier", _format_tier(zone.get("dns_tier", 1)))
        info_table.add_row("Project ID", str(zone.get("project_id", "N/A")))

        nameservers = zone.get("nameservers", [])
        for i, ns in enumerate(nameservers):
            info_table.add_row(f"Nameserver {i+1}", f"[green]{ns}[/green]")

        verified_at = zone.get("verified_at")
        if verified_at:
            info_table.add_row("Verified At", _format_date(verified_at))

        info_table.add_row("Created", _format_date(zone.get("created_at")))
        info_table.add_row("Updated", _format_date(zone.get("updated_at")))

        console.print(info_table)


@zone_app.command("create")
def zone_create(
    ctx: typer.Context,
    domain: str = typer.Argument(..., help="Domain name (e.g., example.com)"),
    project_id: int = typer.Option(..., "--project", "-p", help="Project ID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new DNS zone"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Creating DNS zone...") as progress:
        task = progress.add_task("Creating DNS zone...", total=None)
        try:
            response = client.post("/dns/zones", {"domain": domain, "project_id": project_id})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"DNS zone '{domain}' created!")
        nameservers = response.get("nameservers", [])
        if nameservers:
            console.print()
            console.print("[bold]Point your domain to these nameservers:[/bold]")
            for ns in nameservers:
                console.print(f"  [green]{ns}[/green]")
            console.print()
            console.print("[dim]Then run 'cubecli dns zone verify <uuid>' to verify ownership.[/dim]")


@zone_app.command("delete")
def zone_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a DNS zone and all its records"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete DNS zone {zone_uuid}? All records will be lost!"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting DNS zone...") as progress:
        task = progress.add_task("Deleting DNS zone...", total=None)
        try:
            response = client.delete(f"/dns/zones/{zone_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("DNS zone deletion initiated!")


@zone_app.command("verify")
def zone_verify(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID to verify"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Verify domain ownership (NS records pointing to CubePath)"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Verifying domain...") as progress:
        task = progress.add_task("Verifying domain...", total=None)
        try:
            response = client.post(f"/dns/zones/{zone_uuid}/verify")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        verified = response.get("verified", False)
        message = response.get("message", "")
        if verified:
            print_success(f"Domain verified! {message}")
        else:
            print_error(f"Verification pending: {message}")
            next_check = response.get("next_check_at")
            if next_check:
                console.print(f"[dim]Next check at: {_format_date(next_check)}[/dim]")


@zone_app.command("scan")
def zone_scan(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    auto_import: bool = typer.Option(True, "--import/--preview", help="Auto-import found records or just preview"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Scan public DNS to discover and import existing records"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Scanning DNS records...") as progress:
        task = progress.add_task("Scanning DNS records...", total=None)
        try:
            response = client.post(
                f"/dns/zones/{zone_uuid}/scan",
                params={"auto_import": auto_import}
            )
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        imported = response.get("imported", 0)
        skipped = response.get("skipped", 0)
        errors = response.get("errors", [])

        if auto_import:
            print_success(f"Imported {imported} records, skipped {skipped}")
        else:
            print_success(f"Found {imported} records (preview mode, not imported)")

        if errors:
            for err in errors:
                print_error(err)

        records = response.get("records", [])
        if records:
            _print_records_table(records)


# ── Record commands ────────────────────────────────────────────

@record_app.command("list")
def record_list(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    record_type: Optional[str] = typer.Option(None, "--type", "-t", help="Filter by record type (A, AAAA, CNAME, MX, TXT, SRV, etc.)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List all DNS records in a zone"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    params = {}
    if record_type:
        params["record_type"] = record_type.upper()

    with with_spinner("Fetching DNS records...") as progress:
        task = progress.add_task("Fetching DNS records...", total=None)
        try:
            response = client.get(f"/dns/zones/{zone_uuid}/records", params=params or None)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    records = response if isinstance(response, list) else []

    if json_output:
        print_json(records)
    else:
        if not records:
            print_error("No DNS records found")
            return

        _print_records_table(records)


@record_app.command("create")
def record_create(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    name: str = typer.Option(..., "--name", "-n", help="Record name (@ for apex, or subdomain)"),
    record_type: str = typer.Option(..., "--type", "-t", help="Record type (A, AAAA, CNAME, MX, TXT, SRV, etc.)"),
    content: str = typer.Option(..., "--content", "-c", help="Record content (IP, domain, text, etc.)"),
    ttl: int = typer.Option(3600, "--ttl", help="Time to live in seconds (default: 3600)"),
    priority: Optional[int] = typer.Option(None, "--priority", help="Priority for MX/SRV records"),
    weight: Optional[int] = typer.Option(None, "--weight", help="Weight for SRV records"),
    port: Optional[int] = typer.Option(None, "--port", help="Port for SRV records"),
    comment: Optional[str] = typer.Option(None, "--comment", help="Optional comment"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new DNS record"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "name": name,
        "record_type": record_type.upper(),
        "content": content,
        "ttl": ttl,
    }

    if priority is not None:
        data["priority"] = priority
    if weight is not None:
        data["weight"] = weight
    if port is not None:
        data["port"] = port
    if comment is not None:
        data["comment"] = comment

    with with_spinner("Creating DNS record...") as progress:
        task = progress.add_task("Creating DNS record...", total=None)
        try:
            response = client.post(f"/dns/zones/{zone_uuid}/records", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"{record_type.upper()} record '{name}' created!")


@record_app.command("update")
def record_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    record_uuid: str = typer.Argument(..., help="DNS record UUID"),
    content: Optional[str] = typer.Option(None, "--content", "-c", help="New record content"),
    ttl: Optional[int] = typer.Option(None, "--ttl", help="New TTL in seconds"),
    priority: Optional[int] = typer.Option(None, "--priority", help="New priority (MX/SRV)"),
    weight: Optional[int] = typer.Option(None, "--weight", help="New weight (SRV)"),
    port: Optional[int] = typer.Option(None, "--port", help="New port (SRV)"),
    comment: Optional[str] = typer.Option(None, "--comment", help="New comment"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a DNS record"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    data = {}
    if content is not None:
        data["content"] = content
    if ttl is not None:
        data["ttl"] = ttl
    if priority is not None:
        data["priority"] = priority
    if weight is not None:
        data["weight"] = weight
    if port is not None:
        data["port"] = port
    if comment is not None:
        data["comment"] = comment

    if not data:
        print_error("At least one field must be provided to update")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Updating DNS record...") as progress:
        task = progress.add_task("Updating DNS record...", total=None)
        try:
            response = client.put(f"/dns/zones/{zone_uuid}/records/{record_uuid}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("DNS record updated!")


@record_app.command("delete")
def record_delete(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    record_uuid: str = typer.Argument(..., help="DNS record UUID to delete"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a DNS record"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete DNS record {record_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting DNS record...") as progress:
        task = progress.add_task("Deleting DNS record...", total=None)
        try:
            response = client.delete(f"/dns/zones/{zone_uuid}/records/{record_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("DNS record deleted!")


# ── SOA commands ───────────────────────────────────────────────

@soa_app.command("show")
def soa_show(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show SOA settings for a DNS zone"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching SOA settings...") as progress:
        task = progress.add_task("Fetching SOA settings...", total=None)
        try:
            response = client.get(f"/dns/zones/{zone_uuid}/soa")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        console.print()
        table = Table(title="SOA Settings", show_header=False, box=box.ROUNDED, show_lines=True, title_style="bold green")
        table.add_column("Property", style="bold")
        table.add_column("Value")

        table.add_row("Primary NS", response.get("primary_ns", "N/A"))
        table.add_row("Hostmaster", response.get("hostmaster", "N/A"))
        table.add_row("Serial", str(response.get("serial", "N/A")))
        table.add_row("Refresh", f"{response.get('refresh', 'N/A')}s")
        table.add_row("Retry", f"{response.get('retry', 'N/A')}s")
        table.add_row("Expire", f"{response.get('expire', 'N/A')}s")
        table.add_row("Minimum TTL", f"{response.get('minimum', 'N/A')}s")

        console.print(table)


@soa_app.command("update")
def soa_update(
    ctx: typer.Context,
    zone_uuid: str = typer.Argument(..., help="DNS zone UUID"),
    refresh: Optional[int] = typer.Option(None, "--refresh", help="Refresh interval in seconds (300-86400)"),
    retry: Optional[int] = typer.Option(None, "--retry", help="Retry interval in seconds (60-86400)"),
    expire: Optional[int] = typer.Option(None, "--expire", help="Expire time in seconds (86400-2419200)"),
    minimum: Optional[int] = typer.Option(None, "--minimum", help="Minimum/negative caching TTL in seconds (60-86400)"),
    hostmaster: Optional[str] = typer.Option(None, "--hostmaster", help="Hostmaster email (e.g., admin@example.com)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update SOA settings for a DNS zone"""
    api_token = get_context_value(ctx, "api_token")

    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    data = {}
    if refresh is not None:
        data["refresh"] = refresh
    if retry is not None:
        data["retry"] = retry
    if expire is not None:
        data["expire"] = expire
    if minimum is not None:
        data["minimum"] = minimum
    if hostmaster is not None:
        data["hostmaster"] = hostmaster

    if not data:
        print_error("At least one SOA field must be provided to update")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Updating SOA settings...") as progress:
        task = progress.add_task("Updating SOA settings...", total=None)
        try:
            response = client.put(f"/dns/zones/{zone_uuid}/soa", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("SOA settings updated!")


# ── Helpers ────────────────────────────────────────────────────

def _format_zone_status(status: str) -> str:
    colors = {
        "active": "green",
        "verified": "green",
        "pending": "yellow",
        "pending_verification": "yellow",
        "deleting": "red",
        "error": "red",
    }
    color = colors.get(status.lower(), "white")
    return f"[{color}]{status}[/{color}]"


def _format_tier(tier: int) -> str:
    names = {1: "Free", 2: "Pro", 3: "Business"}
    return names.get(tier, str(tier))


def _format_date(date_str) -> str:
    if not date_str:
        return "N/A"
    if isinstance(date_str, str) and "T" in date_str:
        return date_str.split("T")[0]
    return str(date_str)


def _print_records_table(records: list):
    console.print()
    table = create_table("DNS Records", ["Name", "Type", "Content", "TTL", "Priority", "UUID"])

    for record in records:
        priority = record.get("priority")
        priority_str = str(priority) if priority is not None else "-"

        table.add_row(
            record.get("name", "N/A"),
            record.get("record_type", "N/A"),
            record.get("content", "N/A"),
            str(record.get("ttl", "N/A")),
            priority_str,
            record.get("uuid", "N/A")[:8],
        )

    console.print(table)
