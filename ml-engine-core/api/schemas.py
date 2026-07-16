from pydantic import BaseModel, Field
from typing import List, Optional, Dict, Any

class PIIContext(BaseModel):
    text_segment: str = Field(..., description="The detected text pattern with surrounding context")
    start_index: int
    end_index: int
    file_path: str
    metadata: Dict[str, Any] = Field(default_factory=dict)

class ScanRequest(BaseModel):
    request_id: str
    candidates: List[PIIContext]

class LayerResult(BaseModel):
    layer_name: str
    score: float
    decision: str
    details: Dict[str, Any]

class ValidatedMatch(BaseModel):
    original_text: str
    pii_type: str
    confidence_score: float
    vector_embedding: List[float]
    is_valid: bool
    layer_breakdown: List[LayerResult]

class ScanResponse(BaseModel):
    request_id: str
    processed_count: int
    results: List[ValidatedMatch]
