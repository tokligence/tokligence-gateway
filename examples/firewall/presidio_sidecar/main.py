#!/usr/bin/env python3
"""
Presidio-based Prompt Firewall Sidecar Service

This service provides PII detection and anonymization using Microsoft Presidio.
It exposes an HTTP API that integrates with Tokligence Gateway's HTTP filter.

Features:
- PII detection (email, phone, SSN, credit card, IP, etc.)
- Configurable anonymization/redaction
- Support for multiple languages
- Custom entity recognizers
"""

import time
import logging
from typing import Optional, List, Dict, Any
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from presidio_analyzer import AnalyzerEngine, RecognizerResult
from presidio_anonymizer import AnonymizerEngine
from presidio_anonymizer.entities import OperatorConfig

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Global engines (initialized once)
analyzer_engine: Optional[AnalyzerEngine] = None
anonymizer_engine: Optional[AnonymizerEngine] = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize Presidio engines on startup"""
    global analyzer_engine, anonymizer_engine

    logger.info("Initializing Presidio engines...")
    try:
        analyzer_engine = AnalyzerEngine()
        anonymizer_engine = AnonymizerEngine()
        logger.info("Presidio engines initialized successfully")
    except Exception as e:
        logger.error(f"Failed to initialize Presidio: {e}")
        raise

    yield

    logger.info("Shutting down Presidio sidecar")


app = FastAPI(
    title="Presidio Prompt Firewall",
    description="PII detection and anonymization service for Tokligence Gateway",
    version="0.1.0",
    lifespan=lifespan
)


# Request/Response Models
class FilterRequest(BaseModel):
    """Request from Tokligence Gateway HTTP filter"""
    input: Optional[str] = Field(None, description="Input text to analyze")
    output: Optional[str] = Field(None, description="Output text to analyze")
    model: Optional[str] = None
    endpoint: Optional[str] = None
    user_id: Optional[str] = None
    tenant_id: Optional[str] = None
    session_id: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class Detection(BaseModel):
    """Security detection result"""
    filter_name: str = "presidio"
    type: str
    severity: str
    message: str
    location: str
    details: Dict[str, Any]
    timestamp: float


class RedactedEntity(BaseModel):
    """Redacted PII entity"""
    type: str
    mask: str
    start: int
    end: int
    confidence: float


class FilterResponse(BaseModel):
    """Response to Tokligence Gateway HTTP filter"""
    allowed: bool = True
    block: bool = False
    block_reason: Optional[str] = None
    redacted_input: Optional[str] = None
    redacted_output: Optional[str] = None
    detections: List[Detection] = Field(default_factory=list)
    entities: List[RedactedEntity] = Field(default_factory=list)
    annotations: Dict[str, Any] = Field(default_factory=dict)


# Configuration
PII_ENTITIES = [
    "CREDIT_CARD",
    "CRYPTO",
    "EMAIL_ADDRESS",
    "IBAN_CODE",
    "IP_ADDRESS",
    "NRP",
    "LOCATION",
    "PERSON",
    "PHONE_NUMBER",
    "MEDICAL_LICENSE",
    "US_BANK_NUMBER",
    "US_DRIVER_LICENSE",
    "US_ITIN",
    "US_PASSPORT",
    "US_SSN",
]

# Map Presidio entity types to our redaction masks
ENTITY_MASKS = {
    "CREDIT_CARD": "[CREDIT_CARD]",
    "EMAIL_ADDRESS": "[EMAIL]",
    "PHONE_NUMBER": "[PHONE]",
    "US_SSN": "[SSN]",
    "IP_ADDRESS": "[IP]",
    "PERSON": "[PERSON]",
    "LOCATION": "[LOCATION]",
    "US_PASSPORT": "[PASSPORT]",
    "US_DRIVER_LICENSE": "[DL]",
    "CRYPTO": "[CRYPTO_ADDR]",
}

# Severity mapping
SEVERITY_MAP = {
    "CREDIT_CARD": "critical",
    "US_SSN": "critical",
    "US_PASSPORT": "critical",
    "CRYPTO": "high",
    "EMAIL_ADDRESS": "medium",
    "PHONE_NUMBER": "medium",
    "PERSON": "low",
    "LOCATION": "low",
    "IP_ADDRESS": "low",
}


def analyze_text(
    text: str,
    language: str = "en",
    entities: List[str] = None,
    threshold: float = 0.5
) -> List[RecognizerResult]:
    """Analyze text for PII entities using Presidio"""
    if not text or not analyzer_engine:
        return []

    if entities is None:
        entities = PII_ENTITIES

    try:
        results = analyzer_engine.analyze(
            text=text,
            language=language,
            entities=entities,
            score_threshold=threshold
        )
        return results
    except Exception as e:
        logger.error(f"Error analyzing text: {e}")
        return []


def anonymize_text(
    text: str,
    analyzer_results: List[RecognizerResult],
    redact: bool = True
) -> str:
    """Anonymize/redact PII in text"""
    if not text or not analyzer_results or not anonymizer_engine:
        return text

    try:
        # Build operators for each entity type
        operators = {}
        for result in analyzer_results:
            entity_type = result.entity_type
            if redact:
                # Use our custom masks
                mask = ENTITY_MASKS.get(entity_type, f"[{entity_type}]")
                operators[entity_type] = OperatorConfig("replace", {"new_value": mask})
            else:
                # Just mask with asterisks
                operators[entity_type] = OperatorConfig("mask", {"masking_char": "*", "chars_to_mask": 100})

        anonymized = anonymizer_engine.anonymize(
            text=text,
            analyzer_results=analyzer_results,
            operators=operators
        )
        return anonymized.text
    except Exception as e:
        logger.error(f"Error anonymizing text: {e}")
        return text


def convert_to_detections(
    results: List[RecognizerResult],
    location: str
) -> List[Detection]:
    """Convert Presidio results to Detection objects"""
    detections = []
    for result in results:
        severity = SEVERITY_MAP.get(result.entity_type, "medium")
        detection = Detection(
            filter_name="presidio",
            type="pii",
            severity=severity,
            message=f"Detected {result.entity_type} in {location}",
            location=location,
            details={
                "pii_type": result.entity_type,
                "confidence": result.score,
                "start": result.start,
                "end": result.end,
            },
            timestamp=time.time()
        )
        detections.append(detection)
    return detections


def convert_to_entities(
    results: List[RecognizerResult],
    text: str
) -> List[RedactedEntity]:
    """Convert Presidio results to RedactedEntity objects"""
    entities = []
    for result in results:
        mask = ENTITY_MASKS.get(result.entity_type, f"[{result.entity_type}]")
        entity = RedactedEntity(
            type=result.entity_type,
            mask=mask,
            start=result.start,
            end=result.end,
            confidence=result.score
        )
        entities.append(entity)
    return entities


@app.post("/v1/filter/input", response_model=FilterResponse)
async def filter_input(request: FilterRequest) -> FilterResponse:
    """Filter and analyze input text"""
    if not request.input:
        return FilterResponse()

    start_time = time.time()

    # Analyze for PII
    results = analyze_text(request.input)

    response = FilterResponse()

    if results:
        # Convert results
        response.detections = convert_to_detections(results, "input")
        response.entities = convert_to_entities(results, request.input)

        # Anonymize text
        response.redacted_input = anonymize_text(request.input, results, redact=True)

        # Add annotations
        response.annotations["pii_count"] = len(results)
        response.annotations["pii_types"] = list(set(r.entity_type for r in results))
        response.annotations["processing_time_ms"] = int((time.time() - start_time) * 1000)

        # Determine if we should block
        # Block if we find critical PII (SSN, credit card, etc.)
        critical_entities = [r for r in results if SEVERITY_MAP.get(r.entity_type) == "critical"]
        if critical_entities:
            response.block = True
            response.allowed = False
            response.block_reason = f"Critical PII detected: {', '.join(r.entity_type for r in critical_entities)}"

    logger.info(f"Input filter: {len(results)} PII entities detected, blocked={response.block}")

    return response


@app.post("/v1/filter/output", response_model=FilterResponse)
async def filter_output(request: FilterRequest) -> FilterResponse:
    """Filter and analyze output text"""
    if not request.output:
        return FilterResponse()

    start_time = time.time()

    # Analyze for PII
    results = analyze_text(request.output)

    response = FilterResponse()

    if results:
        # Convert results
        response.detections = convert_to_detections(results, "output")
        response.entities = convert_to_entities(results, request.output)

        # Anonymize text
        response.redacted_output = anonymize_text(request.output, results, redact=True)

        # Add annotations
        response.annotations["pii_output_count"] = len(results)
        response.annotations["pii_output_types"] = list(set(r.entity_type for r in results))
        response.annotations["processing_time_ms"] = int((time.time() - start_time) * 1000)

        # For output, we're more aggressive - redact any PII
        if results:
            response.block = False  # Don't block, just redact
            response.allowed = True

    logger.info(f"Output filter: {len(results)} PII entities detected")

    return response


@app.post("/v1/filter", response_model=FilterResponse)
async def filter_combined(request: FilterRequest) -> FilterResponse:
    """Filter both input and output (if provided)"""
    response = FilterResponse()

    # Process input if provided
    if request.input:
        input_resp = await filter_input(request)
        response.redacted_input = input_resp.redacted_input
        response.detections.extend(input_resp.detections)
        response.entities.extend(input_resp.entities)
        response.annotations.update(input_resp.annotations)
        if input_resp.block:
            response.block = True
            response.allowed = False
            response.block_reason = input_resp.block_reason

    # Process output if provided
    if request.output:
        output_resp = await filter_output(request)
        response.redacted_output = output_resp.redacted_output
        response.detections.extend(output_resp.detections)
        response.entities.extend(output_resp.entities)
        response.annotations.update(output_resp.annotations)

    return response


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "analyzer_ready": analyzer_engine is not None,
        "anonymizer_ready": anonymizer_engine is not None,
        "supported_entities": PII_ENTITIES,
    }


if __name__ == "__main__":
    import uvicorn
    import os

    # Configuration from environment
    host = os.getenv("PRESIDIO_HOST", "0.0.0.0")
    port = int(os.getenv("PRESIDIO_PORT", "7317"))  # Default: 7317 (avoid conflicts)
    workers = int(os.getenv("PRESIDIO_WORKERS", "1"))  # Multi-process workers
    reload = os.getenv("PRESIDIO_RELOAD", "false").lower() == "true"
    log_level = os.getenv("PRESIDIO_LOG_LEVEL", "info")

    logger.info(f"Starting Presidio with {workers} workers on {host}:{port}")

    uvicorn.run(
        "main:app",
        host=host,
        port=port,
        workers=workers,  # Enable multi-process for high concurrency
        reload=reload,
        log_level=log_level,
        # Performance tuning
        loop="uvloop",  # Faster event loop (if available)
        limit_concurrency=1000,  # Max concurrent connections
        backlog=2048,  # Connection backlog
    )
