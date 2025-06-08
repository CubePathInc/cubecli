from pydantic import BaseModel
from typing import Optional
from datetime import datetime

class SSHKey(BaseModel):
    id: int
    name: str
    ssh_key: str
    created_at: Optional[datetime] = None