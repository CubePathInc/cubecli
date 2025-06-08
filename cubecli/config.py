import os
import json
from pathlib import Path
from typing import Dict, Optional
from dotenv import load_dotenv

load_dotenv()

class ConfigError(Exception):
    """Configuration related errors"""
    pass

def get_config_dir() -> Path:
    """Get the configuration directory"""
    config_dir = Path.home() / ".cubecli"
    config_dir.mkdir(exist_ok=True)
    return config_dir

def get_config_file() -> Path:
    """Get the configuration file path"""
    return get_config_dir() / "config.json"

def load_config() -> Dict[str, str]:
    """Load configuration from file or environment"""
    # Check environment variable first
    api_token = os.getenv("CUBE_API_TOKEN")
    if api_token:
        return {"api_token": api_token}
    
    # Check config file
    config_file = get_config_file()
    if not config_file.exists():
        raise ConfigError("No configuration found")
    
    try:
        with open(config_file, "r") as f:
            config = json.load(f)
            if "api_token" not in config:
                raise ConfigError("Invalid configuration: missing api_token")
            return config
    except json.JSONDecodeError:
        raise ConfigError("Invalid configuration file")

def save_config(config: Dict[str, str]) -> None:
    """Save configuration to file"""
    config_file = get_config_file()
    with open(config_file, "w") as f:
        json.dump(config, f, indent=2)
    # Set restrictive permissions
    config_file.chmod(0o600)

def get_api_url() -> str:
    """Get the API base URL"""
    return os.getenv("CUBE_API_URL", "https://api-cloud.cubepath.com")