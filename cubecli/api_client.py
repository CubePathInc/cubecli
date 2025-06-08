import httpx
from typing import Dict, Any, Optional, List
from cubecli.config import get_api_url

class APIClient:
    """HTTP client for CubePath API"""
    
    def __init__(self, api_token: str):
        self.api_token = api_token
        self.base_url = get_api_url()
        self.headers = {
            "Authorization": f"Bearer {api_token}",
            "Content-Type": "application/json",
        }
    
    def _handle_response(self, response: httpx.Response) -> Dict[str, Any]:
        """Handle API response and errors"""
        try:
            response.raise_for_status()
            return response.json()
        except (httpx.HTTPStatusError, httpx.RequestError):
            # Let the command handlers deal with error formatting
            raise
    
    def get(self, path: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """GET request"""
        with httpx.Client() as client:
            response = client.get(
                f"{self.base_url}{path}",
                headers=self.headers,
                params=params,
                timeout=30.0
            )
            return self._handle_response(response)
    
    def post(self, path: str, data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """POST request"""
        with httpx.Client() as client:
            response = client.post(
                f"{self.base_url}{path}",
                headers=self.headers,
                json=data or {},
                timeout=30.0
            )
            return self._handle_response(response)
    
    def put(self, path: str, data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """PUT request"""
        with httpx.Client() as client:
            response = client.put(
                f"{self.base_url}{path}",
                headers=self.headers,
                json=data or {},
                timeout=30.0
            )
            return self._handle_response(response)
    
    def patch(self, path: str, data: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        """PATCH request"""
        with httpx.Client() as client:
            response = client.patch(
                f"{self.base_url}{path}",
                headers=self.headers,
                json=data or {},
                timeout=30.0
            )
            return self._handle_response(response)
    
    def delete(self, path: str) -> Dict[str, Any]:
        """DELETE request"""
        with httpx.Client() as client:
            response = client.delete(
                f"{self.base_url}{path}",
                headers=self.headers,
                timeout=30.0
            )
            return self._handle_response(response)