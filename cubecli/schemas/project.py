from pydantic import BaseModel
from typing import Optional, List
from datetime import datetime

class Project(BaseModel):
    id: int
    name: str
    description: Optional[str] = None
    created_at: Optional[datetime] = None
    vps: Optional[List[dict]] = []
    networks: Optional[List[dict]] = []
    baremetals: Optional[List[dict]] = []