#!/usr/bin/env python3
"""
Presidio-based Prompt Firewall Sidecar Service

This service provides PII detection and anonymization using Microsoft Presidio.
It exposes an HTTP API that integrates with Tokligence Gateway's HTTP filter.

Features:
- PII detection (email, phone, SSN, credit card, IP, etc.)
- Configurable anonymization/redaction
- Support for multiple languages (English + Chinese)
- Custom entity recognizers for Chinese names
"""

import os
import re
import time
import logging
from typing import Optional, List, Dict, Any
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from presidio_analyzer import AnalyzerEngine, RecognizerResult, Pattern, PatternRecognizer
from presidio_analyzer.nlp_engine import NlpEngineProvider
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


class ChineseNameRecognizer(PatternRecognizer):
    """
    Custom recognizer for Chinese names (中文人名).

    Uses common Chinese surname patterns combined with given name patterns.
    Supports both simplified and traditional Chinese characters.
    """

    # Common Chinese surnames (百家姓 - top 100 most common)
    COMMON_SURNAMES = [
        # Single character surnames (单姓)
        "王", "李", "张", "刘", "陈", "杨", "黄", "赵", "吴", "周",
        "徐", "孙", "马", "朱", "胡", "郭", "何", "林", "高", "罗",
        "郑", "梁", "谢", "宋", "唐", "许", "韩", "邓", "冯", "曹",
        "彭", "曾", "萧", "田", "董", "潘", "袁", "蔡", "蒋", "余",
        "于", "杜", "叶", "程", "魏", "苏", "吕", "丁", "任", "沈",
        "姚", "卢", "傅", "钟", "姜", "崔", "谭", "廖", "范", "汪",
        "陆", "金", "石", "戴", "贾", "韦", "夏", "邱", "方", "侯",
        "邹", "熊", "孟", "秦", "白", "江", "阎", "薛", "尹", "段",
        "雷", "龙", "黎", "史", "陶", "毛", "贺", "顾", "龚", "郝",
        "邵", "万", "覃", "武", "钱", "严", "莫", "孔", "向", "常",
        # Double character surnames (复姓)
        "欧阳", "上官", "司马", "诸葛", "皇甫", "令狐", "司徒", "南宫",
        "东方", "西门", "慕容", "公孙", "独孤", "长孙", "宇文", "尉迟",
    ]

    def __init__(self):
        # Build surname pattern
        single_surnames = [s for s in self.COMMON_SURNAMES if len(s) == 1]
        double_surnames = [s for s in self.COMMON_SURNAMES if len(s) == 2]

        # Chinese name pattern: surname + 1-2 Chinese characters (given name)
        # Given name characters are in CJK Unified Ideographs range
        surname_pattern = f"({'|'.join(double_surnames)}|[{''.join(single_surnames)}])"
        given_name_pattern = r"[\u4e00-\u9fff]{1,2}"

        patterns = [
            Pattern(
                name="chinese_name_full",
                regex=f"{surname_pattern}{given_name_pattern}",
                score=0.7
            ),
        ]

        # Support both 'en' and 'zh' so it works without Chinese NLP model
        super().__init__(
            supported_entity="PERSON",
            patterns=patterns,
            supported_language="en",  # Use 'en' to work with default NLP engine
            name="ChineseNameRecognizer",
        )

    def validate_result(self, pattern_text: str) -> Optional[bool]:
        """Additional validation for Chinese names"""
        # Filter out common non-name patterns
        # Avoid matching common words that start with surnames
        common_words = [
            # 常见词语 (Common words)
            "张开", "王牌", "李子", "刘海", "陈旧", "杨柳", "黄金", "赵钱",
            "周末", "吴语", "郑重", "马上", "朱红", "胡说", "郭然", "何必",
            "林木", "高低", "罗列", "梁上", "谢谢", "宋词", "唐朝", "许多",
            "韩语", "邓肯", "冯唐", "曹操", "彭德", "曾经", "萧条", "田野",
            "董事", "潘多", "袁世", "蔡元", "蒋介", "余下", "于是", "杜绝",
            "叶子", "程序", "魏晋", "苏州", "吕布", "丁香", "任何", "沈阳",
            "姚明", "卢沟", "傅雷", "钟表", "姜汁", "崔健", "谭嗣", "廖化",
            "范围", "汪洋", "陆地", "金色", "石头", "戴上", "贾宝", "韦小",
            "夏天", "邱吉", "方向", "侯门", "邹忌", "熊猫", "孟子", "秦始",
            "白色", "江河", "阎王", "薛定", "尹天", "段落", "雷电", "龙虎",
            "黎明", "史记", "陶瓷", "毛泽", "贺岁", "顾客", "龚自", "郝运",
            "邵阳", "万一", "覃思", "武汉", "钱币", "严格", "莫非", "孔子",
            "向往", "常见",
        ]
        if pattern_text in common_words:
            return False
        # Names should typically be 2-4 characters
        if len(pattern_text) < 2 or len(pattern_text) > 4:
            return False
        return True


class ChinesePhoneRecognizer(PatternRecognizer):
    """Custom recognizer for Chinese phone numbers (中国手机号)"""

    def __init__(self):
        patterns = [
            Pattern(
                name="chinese_mobile",
                regex=r"1[3-9]\d{9}",
                score=0.85
            ),
            Pattern(
                name="chinese_mobile_formatted",
                regex=r"1[3-9]\d{1}[-\s]?\d{4}[-\s]?\d{4}",
                score=0.85
            ),
        ]

        super().__init__(
            supported_entity="PHONE_NUMBER",
            patterns=patterns,
            supported_language="en",  # Use 'en' to work with default NLP engine
            name="ChinesePhoneRecognizer",
        )


class ChineseIDCardRecognizer(PatternRecognizer):
    """Custom recognizer for Chinese ID card numbers (身份证号)"""

    def __init__(self):
        patterns = [
            Pattern(
                name="chinese_id_card",
                regex=r"[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[0-9Xx]",
                score=0.95
            ),
        ]

        super().__init__(
            supported_entity="CN_ID_CARD",
            patterns=patterns,
            supported_language="en",  # Use 'en' to work with default NLP engine
            name="ChineseIDCardRecognizer",
        )


def create_analyzer_engine(enable_chinese: bool = True) -> AnalyzerEngine:
    """
    Create Presidio AnalyzerEngine with optional Chinese language support.

    Args:
        enable_chinese: Whether to enable Chinese NER and custom recognizers

    Returns:
        Configured AnalyzerEngine
    """
    # Check if Chinese model is available
    chinese_model_available = False
    if enable_chinese:
        try:
            import spacy
            if spacy.util.is_package("zh_core_web_sm"):
                chinese_model_available = True
                logger.info("Chinese spaCy model (zh_core_web_sm) found")
            else:
                logger.warning("Chinese spaCy model not found. Run: python -m spacy download zh_core_web_sm")
        except Exception as e:
            logger.warning(f"Could not check for Chinese model: {e}")

    # Create engine with English model (always available)
    analyzer = AnalyzerEngine()

    # Add custom Chinese recognizers (work even without Chinese NLP model)
    if enable_chinese:
        analyzer.registry.add_recognizer(ChineseNameRecognizer())
        analyzer.registry.add_recognizer(ChinesePhoneRecognizer())
        analyzer.registry.add_recognizer(ChineseIDCardRecognizer())
        logger.info("Added custom Chinese recognizers (PERSON, PHONE_NUMBER, CN_ID_CARD)")

    return analyzer


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize Presidio engines on startup"""
    global analyzer_engine, anonymizer_engine

    logger.info("Initializing Presidio engines...")
    try:
        enable_chinese = os.getenv("PRESIDIO_ENABLE_CHINESE", "true").lower() == "true"
        analyzer_engine = create_analyzer_engine(enable_chinese=enable_chinese)
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
    # Chinese specific
    "CN_ID_CARD": "[身份证]",
}

# Severity mapping
SEVERITY_MAP = {
    "CREDIT_CARD": "critical",
    "US_SSN": "critical",
    "US_PASSPORT": "critical",
    "CN_ID_CARD": "critical",  # Chinese ID card is critical PII
    "CRYPTO": "high",
    "EMAIL_ADDRESS": "medium",
    "PHONE_NUMBER": "medium",
    "PERSON": "low",
    "LOCATION": "low",
    "IP_ADDRESS": "low",
}


def detect_language(text: str) -> str:
    """
    Simple language detection based on character analysis.
    Returns 'zh' if Chinese characters are detected, otherwise 'en'.
    """
    # Count Chinese characters
    chinese_count = len(re.findall(r'[\u4e00-\u9fff]', text))
    total_chars = len(text.replace(' ', ''))

    if total_chars > 0 and chinese_count / total_chars > 0.3:
        return "zh"
    return "en"


def analyze_text(
    text: str,
    language: str = "en",
    entities: List[str] = None,
    threshold: float = 0.5
) -> List[RecognizerResult]:
    """
    Analyze text for PII entities using Presidio.

    Supports multilingual detection:
    - All recognizers (including Chinese) are registered under 'en' language
    - Pattern-based recognizers work on any text regardless of language
    - Chinese names, phones, and ID cards are detected via regex patterns
    """
    if not text or not analyzer_engine:
        return []

    if entities is None:
        entities = PII_ENTITIES + ["CN_ID_CARD"]  # Add Chinese ID card

    try:
        # Run all recognizers (English NLP + Chinese regex patterns)
        results = analyzer_engine.analyze(
            text=text,
            language="en",  # All recognizers registered under 'en'
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
