import typer
import subprocess
import sys
from pathlib import Path
from cubecli.utils import print_success, print_error, print_info, console

app = typer.Typer(no_args_is_help=False)

@app.command()
def update():
    """Update cubecli to the latest version"""

    try:
        cubecli_path = Path(__file__).parent.parent.parent

        print_info("Updating cubecli...")
        console.print()

        result = subprocess.run(
            ["git", "pull"],
            cwd=cubecli_path,
            capture_output=True,
            text=True
        )

        if result.returncode != 0:
            print_error(f"Git pull failed: {result.stderr}")
            raise typer.Exit(1)

        console.print(result.stdout)

        if "Already up to date" in result.stdout:
            print_success("cubecli is already up to date!")
            return

        print_info("Reinstalling cubecli...")

        install_result = subprocess.run(
            [sys.executable, "-m", "pip", "install", "-e", "."],
            cwd=cubecli_path,
            capture_output=True,
            text=True
        )

        if install_result.returncode != 0:
            print_error(f"Installation failed: {install_result.stderr}")
            raise typer.Exit(1)

        print_success("cubecli updated successfully!")

    except Exception as e:
        print_error(f"Update failed: {str(e)}")
        raise typer.Exit(1)
