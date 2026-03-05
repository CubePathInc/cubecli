import typer
from typing import Optional
from cubecli.api_client import APIClient
from cubecli.utils import (
    print_success, print_error, print_json, create_table,
    confirm_action, get_context_value, with_spinner, format_status,
    console, handle_api_exception
)
from rich.table import Table
from rich import box

app = typer.Typer(no_args_is_help=True)

# Subcommands
listener_app = typer.Typer(no_args_is_help=True)
app.add_typer(listener_app, name="listener", help="Manage listeners")

target_app = typer.Typer(no_args_is_help=True)
app.add_typer(target_app, name="target", help="Manage targets")

healthcheck_app = typer.Typer(no_args_is_help=True)
app.add_typer(healthcheck_app, name="health-check", help="Manage health checks")

plan_app = typer.Typer(no_args_is_help=True)
app.add_typer(plan_app, name="plan", help="View available plans")


# ── LB CRUD ────────────────────────────────────────────────────

@app.command("list")
def lb_list(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List all load balancers"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching load balancers...") as progress:
        task = progress.add_task("Fetching load balancers...", total=None)
        try:
            response = client.get("/loadbalancer/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    lbs = response if isinstance(response, list) else []

    if json_output:
        print_json(lbs)
    else:
        if not lbs:
            print_error("No load balancers found")
            return

        console.print()
        table = create_table("Load Balancers", ["UUID", "Name", "Status", "Plan", "IP", "Listeners", "Location"])

        for lb in lbs:
            ip = lb.get("floating_ip_address") or (lb.get("floating_ip", {}) or {}).get("address", "-")
            listeners = str(lb.get("listeners_count", len(lb.get("listeners", []))))
            plan = lb.get("plan_name") or (lb.get("plan", {}) or {}).get("name", "N/A")

            table.add_row(
                lb.get("uuid", "N/A"),
                lb.get("name", "N/A"),
                format_status(lb.get("status", "unknown")),
                plan,
                ip,
                listeners,
                lb.get("location_name", "N/A"),
            )

        console.print(table)


@app.command()
def show(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Show load balancer details"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching load balancer...") as progress:
        task = progress.add_task("Fetching load balancer...", total=None)
        try:
            # Get full list and find the one we need
            lbs = client.get("/loadbalancer/")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    lb = None
    if isinstance(lbs, list):
        for item in lbs:
            if item.get("uuid") == lb_uuid:
                lb = item
                break

    if not lb:
        print_error(f"Load Balancer {lb_uuid} not found")
        raise typer.Exit(1)

    if json_output:
        print_json(lb)
    else:
        console.print()

        # Basic info
        info_table = Table(show_header=False, box=box.ROUNDED, show_lines=True)
        info_table.add_column("Property", style="bold")
        info_table.add_column("Value")

        info_table.add_row("Name", lb.get("name", "N/A"))
        info_table.add_row("UUID", lb.get("uuid", "N/A"))
        info_table.add_row("Status", format_status(lb.get("status", "unknown")))
        info_table.add_row("Location", lb.get("location_name", "N/A"))

        plan = lb.get("plan", {})
        if plan:
            info_table.add_row("Plan", plan.get("name", "N/A"))
            price = float(plan.get("price_per_hour", 0))
            info_table.add_row("Price", f"${price:.3f}/hr (${price * 720:.2f}/mo)")
            info_table.add_row("Max Listeners", str(plan.get("max_listeners", "N/A")))
            info_table.add_row("Max Targets", str(plan.get("max_targets", "N/A")))

        fip = lb.get("floating_ip", {})
        if fip:
            info_table.add_row("IP Address", f"[green]{fip.get('address', 'N/A')}[/green]")

        console.print(info_table)

        # Listeners
        listeners = lb.get("listeners", [])
        if listeners:
            console.print()
            lt = create_table("Listeners", ["UUID", "Name", "Protocol", "Port", "Target Port", "Algorithm", "Enabled", "Targets"])
            for l in listeners:
                lt.add_row(
                    l.get("uuid", "N/A"),
                    l.get("name", "N/A"),
                    l.get("protocol", "N/A"),
                    str(l.get("source_port", "N/A")),
                    str(l.get("target_port", "N/A")),
                    l.get("algorithm", "N/A"),
                    "[green]Yes[/green]" if l.get("enabled") else "[red]No[/red]",
                    str(l.get("targets_count", len(l.get("targets", [])))),
                )
            console.print(lt)

            # Targets per listener
            for l in listeners:
                targets = l.get("targets", [])
                if targets:
                    console.print()
                    tt = create_table(f"Targets — {l.get('name', 'N/A')}", ["UUID", "Type", "Name", "IP", "Port", "Weight", "Health", "Enabled"])
                    for t in targets:
                        health = t.get("health_status", "unknown")
                        health_colors = {"healthy": "green", "unhealthy": "red", "draining": "yellow"}
                        health_color = health_colors.get(health, "dim")
                        tt.add_row(
                            t.get("uuid", "N/A")[:8],
                            t.get("target_type", "N/A"),
                            t.get("target_name", "N/A"),
                            t.get("target_ip", "-"),
                            str(t.get("port", "-")),
                            str(t.get("weight", 100)),
                            f"[{health_color}]{health}[/{health_color}]",
                            "[green]Yes[/green]" if t.get("enabled") else "[red]No[/red]",
                        )
                    console.print(tt)


@app.command()
def create(
    ctx: typer.Context,
    name: str = typer.Option(..., "--name", "-n", help="Load balancer name"),
    plan: str = typer.Option(..., "--plan", "-p", help="Plan name (e.g., lb.small)"),
    location: str = typer.Option(..., "--location", "-l", help="Location name"),
    project_id: Optional[int] = typer.Option(None, "--project", help="Project ID"),
    label: Optional[str] = typer.Option(None, "--label", help="Optional label"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new load balancer"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "name": name,
        "plan_name": plan,
        "location_name": location,
    }
    if project_id is not None:
        data["project_id"] = project_id
    if label:
        data["label"] = label

    with with_spinner("Creating load balancer...") as progress:
        task = progress.add_task("Creating load balancer...", total=None)
        try:
            response = client.post("/loadbalancer/", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Load Balancer '{name}' created!")
        uuid = response.get("uuid")
        if uuid:
            print_success(f"UUID: {uuid}")


@app.command()
def update(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="New name"),
    label: Optional[str] = typer.Option(None, "--label", help="New label"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a load balancer"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not name and not label:
        print_error("At least one of --name or --label must be provided")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {}
    if name:
        data["name"] = name
    if label:
        data["label"] = label

    with with_spinner("Updating load balancer...") as progress:
        task = progress.add_task("Updating load balancer...", total=None)
        try:
            response = client.patch(f"/loadbalancer/{lb_uuid}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Load Balancer {lb_uuid} updated!")


@app.command()
def delete(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a load balancer"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete Load Balancer {lb_uuid}? This cannot be undone."):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting load balancer...") as progress:
        task = progress.add_task("Deleting load balancer...", total=None)
        try:
            response = client.delete(f"/loadbalancer/{lb_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Load Balancer deletion initiated!")


@app.command()
def resize(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    plan: str = typer.Option(..., "--plan", "-p", help="New plan name"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Resize a load balancer to a different plan"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Resizing load balancer...") as progress:
        task = progress.add_task("Resizing load balancer...", total=None)
        try:
            response = client.post(f"/loadbalancer/{lb_uuid}/resize", {"plan_name": plan})
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Load Balancer resize to {plan} initiated!")


# ── Plans ──────────────────────────────────────────────────────

@plan_app.command("list")
def plan_list(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """List available load balancer plans"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Fetching plans...") as progress:
        task = progress.add_task("Fetching plans...", total=None)
        try:
            response = client.get("/loadbalancer/plans")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    locations = response if isinstance(response, list) else []

    if json_output:
        print_json(locations)
    else:
        if not locations:
            print_error("No plans found")
            return

        for loc in locations:
            console.print()
            console.print(f"[bold]{loc.get('description', loc.get('location_name', 'Unknown'))}[/bold] ({loc.get('location_name', '')})")
            console.print()

            plans = loc.get("plans", [])
            if plans:
                table = create_table("", ["Plan", "Price/Hour", "Price/Month", "Max Listeners", "Max Targets", "Conn/sec"])
                for p in plans:
                    price = float(p.get("price_per_hour", 0))
                    table.add_row(
                        p.get("name", "N/A"),
                        f"${price:.3f}",
                        f"${price * 720:.2f}",
                        str(p.get("max_listeners", "N/A")),
                        str(p.get("max_targets", "N/A")),
                        str(p.get("connections_per_second", "N/A")),
                    )
                console.print(table)


# ── Listeners ──────────────────────────────────────────────────

@listener_app.command("create")
def listener_create(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    name: str = typer.Option(..., "--name", "-n", help="Listener name"),
    protocol: str = typer.Option("http", "--protocol", "-p", help="Protocol (http, https, tcp, tls)"),
    source_port: int = typer.Option(..., "--port", help="Port to listen on"),
    target_port: int = typer.Option(..., "--target-port", help="Port to forward to targets"),
    algorithm: str = typer.Option("round_robin", "--algorithm", "-a", help="Algorithm (round_robin, least_conn, source)"),
    sticky: bool = typer.Option(False, "--sticky/--no-sticky", help="Enable sticky sessions"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Create a new listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "name": name,
        "protocol": protocol.lower(),
        "source_port": source_port,
        "target_port": target_port,
        "algorithm": algorithm,
        "sticky_sessions": sticky,
    }

    with with_spinner("Creating listener...") as progress:
        task = progress.add_task("Creating listener...", total=None)
        try:
            response = client.post(f"/loadbalancer/{lb_uuid}/listeners", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success(f"Listener '{name}' created on port {source_port}!")


@listener_app.command("update")
def listener_update(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    name: Optional[str] = typer.Option(None, "--name", "-n", help="New name"),
    target_port: Optional[int] = typer.Option(None, "--target-port", help="New target port"),
    algorithm: Optional[str] = typer.Option(None, "--algorithm", "-a", help="New algorithm"),
    enabled: Optional[bool] = typer.Option(None, "--enable/--disable", help="Enable or disable"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    data = {}
    if name is not None:
        data["name"] = name
    if target_port is not None:
        data["target_port"] = target_port
    if algorithm is not None:
        data["algorithm"] = algorithm
    if enabled is not None:
        data["enabled"] = enabled

    if not data:
        print_error("At least one field must be provided")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Updating listener...") as progress:
        task = progress.add_task("Updating listener...", total=None)
        try:
            response = client.patch(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Listener updated!")


@listener_app.command("delete")
def listener_delete(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Delete listener {listener_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting listener...") as progress:
        task = progress.add_task("Deleting listener...", total=None)
        try:
            response = client.delete(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Listener deleted!")


# ── Targets ────────────────────────────────────────────────────

@target_app.command("add")
def target_add(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    target_type: str = typer.Option(..., "--type", "-t", help="Target type (vps, baremetal, availability_group)"),
    target_uuid: str = typer.Option(..., "--target", help="Target ID/UUID"),
    port: Optional[int] = typer.Option(None, "--port", "-p", help="Override target port"),
    weight: int = typer.Option(100, "--weight", "-w", help="Target weight (1-100)"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Add a target to a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "target_type": target_type,
        "target_uuid": target_uuid,
        "weight": weight,
    }
    if port is not None:
        data["port"] = port

    with with_spinner("Adding target...") as progress:
        task = progress.add_task("Adding target...", total=None)
        try:
            response = client.post(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/targets", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        target_name = response.get("target_name", target_uuid)
        print_success(f"Target '{target_name}' added!")


@target_app.command("update")
def target_update(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    target_uuid: str = typer.Argument(..., help="Target UUID"),
    port: Optional[int] = typer.Option(None, "--port", "-p", help="New port"),
    weight: Optional[int] = typer.Option(None, "--weight", "-w", help="New weight (1-100)"),
    enabled: Optional[bool] = typer.Option(None, "--enable/--disable", help="Enable or disable"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Update a target"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    data = {}
    if port is not None:
        data["port"] = port
    if weight is not None:
        data["weight"] = weight
    if enabled is not None:
        data["enabled"] = enabled

    if not data:
        print_error("At least one field must be provided")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Updating target...") as progress:
        task = progress.add_task("Updating target...", total=None)
        try:
            response = client.patch(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/targets/{target_uuid}", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Target updated!")


@target_app.command("remove")
def target_remove(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    target_uuid: str = typer.Argument(..., help="Target UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Remove a target from a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action(f"Remove target {target_uuid}?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Removing target...") as progress:
        task = progress.add_task("Removing target...", total=None)
        try:
            response = client.delete(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/targets/{target_uuid}")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Target removed!")


@target_app.command("drain")
def target_drain(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    target_uuid: str = typer.Argument(..., help="Target UUID"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Set a target to draining mode (graceful removal)"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    with with_spinner("Draining target...") as progress:
        task = progress.add_task("Draining target...", total=None)
        try:
            response = client.post(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/targets/{target_uuid}/drain")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Target set to draining mode!")


# ── Health Checks ──────────────────────────────────────────────

@healthcheck_app.command("configure")
def healthcheck_configure(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    protocol: str = typer.Option("http", "--protocol", "-p", help="Protocol (http, https, tcp)"),
    path: str = typer.Option("/", "--path", help="Health check path (HTTP/HTTPS)"),
    interval: int = typer.Option(30, "--interval", help="Check interval in seconds (5-300)"),
    timeout: int = typer.Option(5, "--timeout", help="Timeout in seconds (1-60)"),
    healthy: int = typer.Option(2, "--healthy", help="Healthy threshold (1-10)"),
    unhealthy: int = typer.Option(3, "--unhealthy", help="Unhealthy threshold (1-10)"),
    expected_codes: str = typer.Option("200-399", "--codes", help="Expected HTTP status codes"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Configure health check for a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    client = APIClient(api_token)

    data = {
        "protocol": protocol.lower(),
        "path": path,
        "interval_seconds": interval,
        "timeout_seconds": timeout,
        "healthy_threshold": healthy,
        "unhealthy_threshold": unhealthy,
        "expected_codes": expected_codes,
    }

    with with_spinner("Configuring health check...") as progress:
        task = progress.add_task("Configuring health check...", total=None)
        try:
            response = client.put(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/health-check", data)
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Health check configured!")


@healthcheck_app.command("delete")
def healthcheck_delete(
    ctx: typer.Context,
    lb_uuid: str = typer.Argument(..., help="Load Balancer UUID"),
    listener_uuid: str = typer.Argument(..., help="Listener UUID"),
    force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
    json_output: bool = typer.Option(False, "--json", help="Output in JSON format"),
):
    """Delete health check from a listener"""
    api_token = get_context_value(ctx, "api_token")
    if not api_token:
        print_error("No API token configured")
        raise typer.Exit(1)

    if not force:
        if not confirm_action("Delete health check?"):
            print_error("Operation cancelled")
            return

    client = APIClient(api_token)

    with with_spinner("Deleting health check...") as progress:
        task = progress.add_task("Deleting health check...", total=None)
        try:
            response = client.delete(f"/loadbalancer/{lb_uuid}/listeners/{listener_uuid}/health-check")
            progress.update(task, completed=True)
        except Exception as e:
            handle_api_exception(e, progress)

    if json_output:
        print_json(response)
    else:
        print_success("Health check deleted!")
