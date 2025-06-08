import json
from typing import Any, Dict, List, Callable
from rich.console import Console
from rich.table import Table
from rich.panel import Panel
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn
from rich import box
import typer
import httpx

console = Console()

def print_success(message: str) -> None:
    """Print success message"""
    console.print(f"[green]✓[/green] {message}")

def print_error(message: str) -> None:
    """Print error message"""
    console.print(f"[red]✗[/red] {message}", style="red")

def print_warning(message: str) -> None:
    """Print warning message"""
    console.print(f"[yellow]⚠[/yellow] {message}", style="yellow")

def print_info(message: str) -> None:
    """Print info message"""
    console.print(f"[blue]ℹ[/blue] {message}")

def print_json(data: Any) -> None:
    """Print data as formatted JSON"""
    console.print_json(json.dumps(data, indent=2))

def create_table(title: str, columns: List[str]) -> Table:
    """Create a styled table"""
    table = Table(
        title=title,
        box=box.ROUNDED,
        show_lines=True,
        title_style="bold green",
        header_style="bold",
    )
    
    for column in columns:
        table.add_column(column, no_wrap=True)
    
    return table

def confirm_action(message: str, default: bool = False) -> bool:
    """Ask for confirmation"""
    return typer.confirm(message, default=default)

def format_status(status: str) -> str:
    """Format status with color"""
    status_colors = {
        "active": "green",
        "running": "green",
        "stopped": "dim",
        "paused": "dim",
        "pending": "dim",
        "error": "red",
        "failed": "red",
    }
    
    color = status_colors.get(status.lower(), "white")
    return f"[{color}]{status}[/{color}]"

def format_bytes(bytes_value: int) -> str:
    """Format bytes to human readable format"""
    for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
        if bytes_value < 1024.0:
            return f"{bytes_value:.2f} {unit}"
        bytes_value /= 1024.0
    return f"{bytes_value:.2f} PB"

def with_spinner(message: str):
    """Context manager for spinner"""
    return Progress(
        SpinnerColumn(spinner_name="dots", style="green"),
        TextColumn("[progress.description]{task.description}"),
        transient=True,
    )

def with_progress_bar(message: str, total: int = 100):
    """Context manager for progress bar"""
    return Progress(
        SpinnerColumn(spinner_name="dots", style="green"),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(bar_width=40, style="green", complete_style="green"),
        TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
        transient=True,
    )

def truncate_string(text: str, max_length: int = 50) -> str:
    """Truncate string with ellipsis"""
    if len(text) <= max_length:
        return text
    return text[:max_length - 3] + "..."

def format_price(price: float, currency: str = "USD") -> str:
    """Format price with currency"""
    return f"${price:.2f} {currency}"

def get_context_value(ctx: typer.Context, key: str, default: Any = None) -> Any:
    """Get value from context object"""
    if ctx.obj and key in ctx.obj:
        return ctx.obj[key]
    return default

def handle_api_error(func: Callable) -> Callable:
    """Decorator to handle API errors gracefully"""
    def wrapper(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except httpx.HTTPStatusError as e:
            # Try to get error message from response body
            error_detail = "Unknown error"
            try:
                error_data = e.response.json()
                if isinstance(error_data, dict):
                    error_detail = error_data.get("detail", error_data.get("message", str(error_data)))
                else:
                    error_detail = str(error_data)
            except:
                error_detail = e.response.text or str(e)
            
            print_error(f"API error: {e.response.status_code} - {error_detail}")
            raise typer.Exit(1)
        except httpx.RequestError as e:
            print_error(f"Network error: {str(e)}")
            raise typer.Exit(1)
        except Exception as e:
            print_error(f"Unexpected error: {str(e)}")
            raise typer.Exit(1)
    return wrapper

def handle_api_exception(e: Exception, progress=None):
    """Handle API exceptions and show appropriate error messages"""
    if progress:
        # Stop and hide the spinner/progress bar
        try:
            progress.stop()
            # Force refresh to clear the transient display
            progress.refresh()
        except:
            pass
    
    if isinstance(e, httpx.HTTPStatusError):
        # Try to get error message from response body
        error_detail = "Unknown error"
        try:
            error_data = e.response.json()
            if isinstance(error_data, dict):
                error_detail = error_data.get("detail", error_data.get("message", str(error_data)))
            else:
                error_detail = str(error_data)
        except:
            error_detail = e.response.text or str(e)
        
        # Clean up the error detail if it's a JSON string
        if error_detail.startswith('{"detail":"') and error_detail.endswith('"}'):
            try:
                import json
                error_dict = json.loads(error_detail)
                error_detail = error_dict.get("detail", error_detail)
            except:
                pass
        
        print_error(f"{error_detail}")
    elif isinstance(e, httpx.RequestError):
        print_error(f"Network error: {str(e)}")
    else:
        print_error(f"Unexpected error: {str(e)}")
    
    raise typer.Exit(1)