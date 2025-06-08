from pydantic import BaseModel
from typing import Optional
from datetime import datetime

class VPS(BaseModel):
    id: int
    name: str
    label: Optional[str] = None
    status: str
    main_ip: Optional[str] = None
    ipv6: Optional[str] = None
    plan_name: str
    template_name: str
    location_name: str
    project_id: int
    network_id: Optional[int] = None
    created_at: Optional[datetime] = None
    vcpus: Optional[int] = None
    memory: Optional[int] = None
    disk: Optional[int] = None