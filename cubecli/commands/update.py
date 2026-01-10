import typer
import subprocess
from cubecli.utils import print_success, print_error, print_info

def update_cubecli():
    """Update cubecli to the latest version"""
    try:
        print_info("Updating cubecli from GitHub...")

        result = subprocess.run(
            ["pipx", "install", "--force", "git+https://github.com/CubePathInc/cubecli.git"],
            capture_output=True,
            text=True
        )

        if result.returncode != 0:
            print_error(f"Update failed: {result.stderr}")
            raise typer.Exit(1)

        print_success("cubecli updated successfully!")

    except Exception as e:
        print_error(f"Update failed: {str(e)}")
        raise typer.Exit(1)
