from pydantic import BaseModel
from typing import Optional
from datetime import datetime

class SSHKey(BaseModel):
    id: int
    name: str
    ssh_key: str
    fingerprint: Optional[str] = None
    key_type: Optional[str] = None
    created_at: Optional[datetime] = None