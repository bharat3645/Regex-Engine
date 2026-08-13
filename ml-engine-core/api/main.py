from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import JSONResponse
import uvicorn
import os
import structlog
from api.schemas import ScanRequest, ScanResponse
from pipeline.manager import PipelineManager

# Setup Structured Logging
structlog.configure(
    processors=[
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.JSONRenderer()
    ]
)
logger = structlog.get_logger()

app = FastAPI(
    title="Hexa-Core PII Detection Engine",
    description="Isolated Server-Side ML Pipeline for High-Precision PII Validation",
    version="1.1.0"
)

# Initialize Pipeline (Singleton)
try:
    config_path = os.path.join(os.path.dirname(__file__), "..", "config", "settings.yaml")
    pipeline = PipelineManager(config_path)
    logger.info("pipeline_initialized", config=config_path)
except Exception as e:
    logger.critical("pipeline_initialization_failed", error=str(e))
    raise RuntimeError(f"Failed to initialize pipeline: {e}")

@app.get("/health")
def health_check():
    return {"status": "active", "layers": 6, "version": "1.1.0"}

@app.post("/api/v1/scan", response_model=ScanResponse)
async def scan_pii(request: ScanRequest):
    log = logger.bind(request_id=request.request_id, count=len(request.candidates))
    log.info("scan_request_received")
    
    try:
        results = pipeline.process_batch(request.candidates)
        log.info("scan_completed", processed=len(results))
        return ScanResponse(
            request_id=request.request_id,
            processed_count=len(results),
            results=results
        )
    except Exception as e:
        log.error("scan_failed", error=str(e))
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    uvicorn.run("api.main:app", host="0.0.0.0", port=8000, reload=True)
