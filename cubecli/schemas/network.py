from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime

class Network(BaseModel):
    id: int
    name: str
    label: Optional[str] = None
    ip_range: str
    prefix: int
    location_name: str
    project_id: int
    created_at: Optional[datetime] = None
    vps: Optional[List[dict]] = []