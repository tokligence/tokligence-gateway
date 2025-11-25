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
import hashlib
from typing import Optional, List, Dict, Any, Tuple
from contextlib import asynccontextmanager

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from presidio_analyzer import AnalyzerEngine, RecognizerResult, Pattern, PatternRecognizer, EntityRecognizer
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
analyzer_language: str = "en"  # Primary language for analysis


class ChineseNameRecognizer(EntityRecognizer):
    """
    Custom recognizer for Chinese names (中文人名).

    Uses common Chinese surname patterns combined with given name patterns.
    Supports both simplified and traditional Chinese characters.

    Key feature: Boundary-aware matching
    - Correctly detects name boundaries in continuous Chinese text
    - Prevents matching "张三今" when text is "张三今天来了" (should match "张三" only)
    - Same name appearing multiple times gets same token for consistent redaction
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

    # Common words that start with surnames but are NOT names
    COMMON_WORDS = {
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
    }

    # CJK character range for boundary checking
    CJK_RANGE = re.compile(r'[\u4e00-\u9fff]')

    def __init__(self, supported_language: str = "en"):
        # Build surname sets for fast lookup
        self.single_surnames = set(s for s in self.COMMON_SURNAMES if len(s) == 1)
        self.double_surnames = set(s for s in self.COMMON_SURNAMES if len(s) == 2)
        self.all_surnames = self.single_surnames | self.double_surnames

        super().__init__(
            supported_entities=["PERSON"],
            supported_language=supported_language,
            name="ChineseNameRecognizer",
        )

    def load(self) -> None:
        """No external resources to load."""
        pass

    def analyze(self, text: str, entities: List[str], nlp_artifacts=None) -> List[RecognizerResult]:
        """
        Analyze text for Chinese names with proper boundary detection.

        Strategy:
        1. Find all potential name matches (surname + 1-2 chars)
        2. Check if the match is at a word boundary (not followed by another CJK char)
        3. Prefer shorter matches when followed by CJK chars (e.g., "张三" not "张三今")
        """
        results = []

        # Only analyze if PERSON is in entities list
        if "PERSON" not in entities:
            return results

        # Find all surnames in text
        for i, char in enumerate(text):
            # Check for single surname
            if char in self.single_surnames:
                result = self._extract_name_at_position(text, i, 1)
                if result:
                    results.append(result)

            # Check for double surname
            if i + 1 < len(text):
                double = text[i:i+2]
                if double in self.double_surnames:
                    result = self._extract_name_at_position(text, i, 2)
                    if result:
                        results.append(result)

        # Remove duplicates (same start position, keep longer match)
        results = self._deduplicate_results(results)

        return results

    def _extract_name_at_position(self, text: str, start: int, surname_len: int) -> Optional[RecognizerResult]:
        """
        Extract a Chinese name starting at the given position.

        Args:
            text: Full text
            start: Start position of surname
            surname_len: Length of surname (1 for single, 2 for double)

        Returns:
            RecognizerResult if valid name found, None otherwise
        """
        # Note: We don't check for preceding CJK chars because Chinese names
        # can appear anywhere in continuous text (e.g., "和李四" contains name "李四")
        # The deduplication step handles overlapping matches

        surname = text[start:start + surname_len]

        # Try to match given name (1-2 chars after surname)
        given_start = start + surname_len

        # Must have at least one char after surname
        if given_start >= len(text):
            return None

        # First char after surname must be CJK
        if not self.CJK_RANGE.match(text[given_start]):
            return None

        # Determine name length based on what follows
        # Default: surname + 1 char (most common: 张三)
        name_end = given_start + 1

        # Check if there's a second given name char
        if given_start + 1 < len(text) and self.CJK_RANGE.match(text[given_start + 1]):
            # There's a second CJK char - but is it part of the name?
            # Check what comes after it
            if given_start + 2 >= len(text) or not self.CJK_RANGE.match(text[given_start + 2]):
                # No third CJK char, so second char is likely part of name
                name_end = given_start + 2
            else:
                # Third CJK char exists - likely we're in middle of a sentence
                # Stick with 2-char name (surname + 1 char)
                name_end = given_start + 1

        # Extract the full name
        full_name = text[start:name_end]

        # Validate the name
        if not self._validate_name(full_name):
            return None

        return RecognizerResult(
            entity_type="PERSON",
            start=start,
            end=name_end,
            score=0.85,
            analysis_explanation=None,
            recognition_metadata={
                "recognizer_name": self.name,
            }
        )

    def _validate_name(self, name: str) -> bool:
        """Validate that the extracted text is likely a name."""
        # Must be 2-4 chars
        if len(name) < 2 or len(name) > 4:
            return False

        # Filter out common words
        if name in self.COMMON_WORDS:
            return False

        return True

    def _deduplicate_results(self, results: List[RecognizerResult]) -> List[RecognizerResult]:
        """Remove duplicate results at the same position."""
        if not results:
            return results

        # Group by start position
        by_start = {}
        for r in results:
            if r.start not in by_start:
                by_start[r.start] = r
            else:
                # Keep the one with higher score, or longer match if scores equal
                existing = by_start[r.start]
                if r.score > existing.score or (r.score == existing.score and r.end > existing.end):
                    by_start[r.start] = r

        # Remove overlapping results (keep earlier starts)
        sorted_results = sorted(by_start.values(), key=lambda x: x.start)
        final_results = []
        last_end = -1

        for r in sorted_results:
            if r.start >= last_end:
                final_results.append(r)
                last_end = r.end

        return final_results


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


class InternationalPhoneRecognizer(PatternRecognizer):
    """Custom recognizer for international phone numbers (US, UK, Singapore, etc.)"""

    def __init__(self):
        patterns = [
            # US formats: +1-xxx-xxx-xxxx, (xxx) xxx-xxxx, xxx-xxx-xxxx
            Pattern(
                name="us_phone_intl",
                regex=r"\+1[-.\s]?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}",
                score=0.85
            ),
            Pattern(
                name="us_phone_parens",
                regex=r"\(\d{3}\)\s*\d{3}[-.\s]?\d{4}",
                score=0.85
            ),
            Pattern(
                name="us_phone_dashes",
                regex=r"\b\d{3}[-.\s]\d{3}[-.\s]\d{4}\b",
                score=0.75
            ),
            # UK formats: +44 xx xxxx xxxx, 0xx xxxx xxxx
            Pattern(
                name="uk_phone",
                regex=r"\+44[-.\s]?\d{2,4}[-.\s]?\d{3,4}[-.\s]?\d{3,4}",
                score=0.85
            ),
            Pattern(
                name="uk_phone_local",
                regex=r"\b0\d{2,4}[-.\s]?\d{3,4}[-.\s]?\d{3,4}\b",
                score=0.70
            ),
            # Singapore formats: +65 xxxx xxxx, 9xxx xxxx, 8xxx xxxx
            Pattern(
                name="sg_phone",
                regex=r"\+65[-.\s]?\d{4}[-.\s]?\d{4}",
                score=0.85
            ),
            Pattern(
                name="sg_phone_local",
                regex=r"\b[89]\d{3}[-.\s]?\d{4}\b",
                score=0.70
            ),
            # Generic international: +xx xxx xxx xxxx
            Pattern(
                name="intl_phone_generic",
                regex=r"\+\d{1,3}[-.\s]?\d{2,4}[-.\s]?\d{3,4}[-.\s]?\d{3,4}",
                score=0.75
            ),
        ]

        super().__init__(
            supported_entity="PHONE_NUMBER",
            patterns=patterns,
            supported_language="en",
            name="InternationalPhoneRecognizer",
        )


class URLRecognizer(PatternRecognizer):
    """Custom recognizer for URLs"""

    def __init__(self):
        patterns = [
            Pattern(
                name="url_https",
                regex=r"https?://[^\s<>\"']+",
                score=0.85
            ),
            Pattern(
                name="url_www",
                regex=r"www\.[a-zA-Z0-9][a-zA-Z0-9-]*\.[a-zA-Z]{2,}[^\s<>\"']*",
                score=0.80
            ),
        ]

        super().__init__(
            supported_entity="URL",
            patterns=patterns,
            supported_language="en",
            name="URLRecognizer",
        )


class USSSNRecognizer(PatternRecognizer):
    """Custom recognizer for US Social Security Numbers"""

    def __init__(self):
        patterns = [
            # Standard format: xxx-xx-xxxx
            Pattern(
                name="ssn_dashes",
                regex=r"\b\d{3}-\d{2}-\d{4}\b",
                score=0.85
            ),
            # No separators: xxxxxxxxx (9 digits)
            Pattern(
                name="ssn_no_sep",
                regex=r"\b\d{9}\b",
                score=0.50  # Lower score - could be other numbers
            ),
        ]

        super().__init__(
            supported_entity="US_SSN",
            patterns=patterns,
            supported_language="en",
            name="USSSNRecognizer",
        )


class VehiclePlateRecognizer(PatternRecognizer):
    """Custom recognizer for vehicle license plates (China only for now)

    Note: US plate patterns are disabled due to high false positive rate
    (conflicts with SSN, IP addresses, etc.)
    """

    def __init__(self):
        patterns = [
            # China plates only: 京A12345, 粤B·12345, 沪C123AB
            # These are distinctive enough to avoid false positives
            Pattern(
                name="cn_plate",
                regex=r"[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼][A-Z][·.]?[A-Z0-9]{5,6}",
                score=0.90
            ),
            # US plates disabled - too many false positives
            # Pattern(name="us_plate", regex=r"\b[A-Z]{2,3}[-\s]?\d{3,4}\b", score=0.60),
        ]

        super().__init__(
            supported_entity="VEHICLE_PLATE",
            patterns=patterns,
            supported_language="en",
            name="VehiclePlateRecognizer",
        )


class PassportRecognizer(PatternRecognizer):
    """Custom recognizer for passport numbers from 20+ major countries.

    Supported countries (sorted by region):

    Americas (4):
    - US: 9 digits (with context keyword)
    - CA: 2 letters + 6 digits
    - MX: Letter + 8 digits
    - BR: 2 letters + 6 digits

    Europe (6):
    - UK: 9 digits (with context keyword)
    - DE: 9 alphanumeric (no vowels)
    - FR: 2 digits + 2 letters + 5 digits
    - IT: 2 letters + 7 digits
    - ES: 3 letters + 6 digits
    - PL: 2 letters + 7 digits
    - RU: 2 digits + space + 7 digits

    Asia (8):
    - CN: E/G + 8 digits
    - JP: 2 letters + 7 digits
    - KR: 1-2 letters + 7-8 digits
    - IN: 1 letter + 7 digits
    - SG: 1 letter + 7 digits + 1 letter
    - MY: 1-2 letters + 7 digits
    - TH: 1 letter + 8 digits
    - PH: 2 letters + 7 digits

    Oceania (2):
    - AU: 1-2 letters + 7 digits
    - NZ: 2 letters + 6 digits

    Middle East (2):
    - AE (UAE): 9 digits (with context)
    - SA: 1 letter + 8 digits

    Total: 22 countries

    Note: Pure digit patterns (US/UK/UAE) require context keywords to avoid
    false positives with other similar numbers.

    Not supported (high false positive rate):
    - NL: 9 alphanumeric (too generic, matches random IDs)
    """

    def __init__(self):
        patterns = [
            # ============== ASIA ==============
            # China passport: E/G + 8 digits (very distinctive)
            Pattern(
                name="cn_passport",
                regex=r"\b[EeGg][0-9]{8}\b",
                score=0.90
            ),
            # Japan passport: 2 letters + 7 digits (e.g., TZ1234567)
            Pattern(
                name="jp_passport",
                regex=r"\b[A-Z]{2}[0-9]{7}\b",
                score=0.70
            ),
            # South Korea passport: 1-2 letters + 7-8 digits (e.g., M12345678)
            Pattern(
                name="kr_passport",
                regex=r"\b[A-Z]{1,2}[0-9]{7,8}\b",
                score=0.65
            ),
            # India passport: 1 letter + 7 digits (e.g., J1234567)
            Pattern(
                name="in_passport",
                regex=r"\b[A-Z][0-9]{7}\b",
                score=0.60
            ),
            # Singapore passport: 1 letter + 7 digits + 1 letter (e.g., S1234567A)
            Pattern(
                name="sg_passport",
                regex=r"\b[A-Z][0-9]{7}[A-Z]\b",
                score=0.85
            ),
            # Malaysia passport: 1-2 letters + 7 digits (e.g., A12345678)
            Pattern(
                name="my_passport",
                regex=r"\b[A-Z]{1,2}[0-9]{7,8}\b",
                score=0.60
            ),
            # Thailand passport: 1 letter + 8 digits (e.g., AA1234567)
            Pattern(
                name="th_passport",
                regex=r"\b[A-Z]{1,2}[0-9]{7,8}\b",
                score=0.60
            ),
            # Philippines passport: 2 letters + 7 digits (e.g., EC1234567)
            Pattern(
                name="ph_passport",
                regex=r"\b[A-Z]{2}[0-9]{7}\b",
                score=0.65
            ),

            # ============== EUROPE ==============
            # Germany passport: 9 alphanumeric (excludes vowels, B,D,Q,S)
            # Must contain at least one letter to avoid matching pure digit strings
            Pattern(
                name="de_passport",
                regex=r"\b(?=[A-Z0-9]*[CFGHJKLMNPRTVWXYZ])[CFGHJKLMNPRTVWXYZ0-9]{9}\b",
                score=0.65
            ),
            # France passport: 2 digits + 2 letters + 5 digits (e.g., 15AB12345)
            Pattern(
                name="fr_passport",
                regex=r"\b[0-9]{2}[A-Z]{2}[0-9]{5}\b",
                score=0.80
            ),
            # Italy passport: 2 letters + 7 digits (e.g., AA1234567)
            Pattern(
                name="it_passport",
                regex=r"\b[A-Z]{2}[0-9]{7}\b",
                score=0.70
            ),
            # Spain passport: 3 letters + 6 digits (e.g., AAA123456)
            Pattern(
                name="es_passport",
                regex=r"\b[A-Z]{3}[0-9]{6}\b",
                score=0.75
            ),
            # Netherlands passport: 9 alphanumeric - REMOVED due to high false positive rate
            # (matches any 9-character alphanumeric string including random IDs)
            # To detect NL passports, use context-based patterns below

            # Poland passport: 2 letters + 7 digits (e.g., AY1234567)
            Pattern(
                name="pl_passport",
                regex=r"\b[A-Z]{2}[0-9]{7}\b",
                score=0.70
            ),
            # Russia passport: 2 digits + space + 7 digits (e.g., 70 1234567)
            # Requires space to avoid matching random 9-digit numbers
            Pattern(
                name="ru_passport",
                regex=r"\b[0-9]{2}\s[0-9]{7}\b",
                score=0.75
            ),

            # ============== AMERICAS ==============
            # Canada passport: 2 letters + 6 digits (e.g., AB123456)
            Pattern(
                name="ca_passport",
                regex=r"\b[A-Z]{2}[0-9]{6}\b",
                score=0.70
            ),
            # Mexico passport: Letter + 8 digits (e.g., G12345678)
            Pattern(
                name="mx_passport",
                regex=r"\b[A-Z][0-9]{8}\b",
                score=0.70
            ),
            # Brazil passport: 2 letters + 6 digits (e.g., FH123456)
            Pattern(
                name="br_passport",
                regex=r"\b[A-Z]{2}[0-9]{6}\b",
                score=0.70
            ),

            # ============== OCEANIA ==============
            # Australia passport: 1-2 letters + 7 digits (e.g., PA1234567)
            Pattern(
                name="au_passport",
                regex=r"\b[A-Z]{1,2}[0-9]{7}\b",
                score=0.65
            ),
            # New Zealand passport: 2 letters + 6 digits (e.g., LN123456)
            Pattern(
                name="nz_passport",
                regex=r"\b[A-Z]{2}[0-9]{6}\b",
                score=0.70
            ),

            # ============== MIDDLE EAST ==============
            # Saudi Arabia passport: 1 letter + 8 digits (e.g., A12345678)
            Pattern(
                name="sa_passport",
                regex=r"\b[A-Z][0-9]{8}\b",
                score=0.65
            ),

            # ============== CONTEXT-BASED (9 digits) ==============
            # US/UK/UAE passport with context: "passport" keyword + 9 digits
            # This avoids matching random 9-digit numbers (SSN, phone, etc.)
            Pattern(
                name="us_uk_uae_passport_with_context",
                regex=r"(?i)(?:passport|護照|护照|パスポート|여권|جواز\s*(?:السفر)?)[:\s#№]*([0-9]{9})\b",
                score=0.90
            ),
            # Alternative: passport number followed by digits
            Pattern(
                name="passport_number_context",
                regex=r"(?i)(?:passport\s*(?:no\.?|number|#|num)?|護照號碼|护照号)[:\s]*([0-9]{8,9})\b",
                score=0.85
            ),
        ]

        super().__init__(
            supported_entity="PASSPORT",
            patterns=patterns,
            supported_language="en",
            name="PassportRecognizer",
        )


def create_analyzer_engine(enable_chinese: bool = True) -> AnalyzerEngine:
    """
    Create Presidio AnalyzerEngine with optional Chinese language support.

    Args:
        enable_chinese: Whether to enable Chinese NER and custom recognizers

    Returns:
        Configured AnalyzerEngine

    Environment Variables:
        PRESIDIO_NER_ENGINE: NER engine to use (default: spacy)
            Options:
            - spacy: Use spaCy models (traditional, faster on CPU)
            - xlmr: Use XLM-RoBERTa (more accurate, requires transformers)

        PRESIDIO_SPACY_MODEL: spaCy model to use for English NER (default: xx_ent_wiki_sm)
            Options:
            - en_core_web_sm (12MB) - Fast, lower accuracy
            - en_core_web_md (40MB) - Balanced
            - en_core_web_lg (600MB) - Good accuracy
            - en_core_web_trf (450MB) - Best accuracy, requires GPU for speed
            - xx_ent_wiki_sm - Multilingual (default)

        PRESIDIO_MULTILINGUAL: Enable multilingual mode (default: true)

        XLMR_NER_MODEL: XLM-RoBERTa model (when using xlmr engine)
            Options:
            - hrl: High Resource Languages (10 langs, recommended)
            - wikiann: WikiANN (20 langs, broader coverage)

        XLMR_NER_DEVICE: Device for XLM-RoBERTa (-1=CPU, 0+=GPU)

    Note on XLM-RoBERTa:
        XLM-RoBERTa provides more accurate entity boundary detection,
        especially for Chinese text where there are no word separators.
        - GPU recommended for production (50-100ms/request)
        - CPU is slower but works (~500ms-1s/request)
    """
    import spacy
    from presidio_analyzer.nlp_engine import SpacyNlpEngine, NlpEngineProvider

    # Check which NER engine to use
    ner_engine = os.getenv("PRESIDIO_NER_ENGINE", "spacy").lower()

    # Get spaCy model from environment
    spacy_model = os.getenv("PRESIDIO_SPACY_MODEL", "xx_ent_wiki_sm")
    enable_multilingual = os.getenv("PRESIDIO_MULTILINGUAL", "true").lower() == "true"

    # Check if requested spaCy model is available
    if not spacy.util.is_package(spacy_model):
        logger.warning(f"Requested spaCy model '{spacy_model}' not found.")
        logger.warning(f"Run: python -m spacy download {spacy_model}")
        logger.warning("Falling back to default model...")
        # Try fallback models
        for fallback in ["en_core_web_lg", "en_core_web_md", "en_core_web_sm"]:
            if spacy.util.is_package(fallback):
                spacy_model = fallback
                logger.info(f"Using fallback model: {spacy_model}")
                break
        else:
            raise RuntimeError("No spaCy English model available. Run: python -m spacy download en_core_web_sm")

    # Warn about transformer model on CPU
    if spacy_model == "en_core_web_trf":
        try:
            import torch
            if not torch.cuda.is_available():
                logger.warning("=" * 60)
                logger.warning("⚠️  PERFORMANCE WARNING: Using transformer model without GPU")
                logger.warning("   en_core_web_trf is very slow on CPU (~1-2s per request)")
                logger.warning("   Consider using en_core_web_lg for CPU deployments")
                logger.warning("=" * 60)
            else:
                logger.info(f"GPU detected: {torch.cuda.get_device_name(0)}")
        except ImportError:
            pass

    logger.info(f"Loading spaCy model: {spacy_model}")

    # Build models list based on configuration
    models = []
    supported_languages = []

    # Determine primary model based on spacy_model setting
    if spacy_model == "xx_ent_wiki_sm":
        models.append({"lang_code": "xx", "model_name": "xx_ent_wiki_sm"})
        supported_languages.append("xx")
        logger.info("Primary model: xx_ent_wiki_sm (multilingual, 100+ languages)")
    else:
        models.append({"lang_code": "en", "model_name": spacy_model})
        supported_languages.append("en")
        logger.info(f"Primary model: {spacy_model} (English)")

    # Optionally add additional models for better coverage
    if enable_multilingual and spacy_model != "xx_ent_wiki_sm":
        if spacy.util.is_package("xx_ent_wiki_sm"):
            models.append({"lang_code": "xx", "model_name": "xx_ent_wiki_sm"})
            supported_languages.append("xx")
            logger.info("Added multilingual model: xx_ent_wiki_sm")

    # Create NLP engine with the specified model(s)
    nlp_configuration = {
        "nlp_engine_name": "spacy",
        "models": models,
    }

    nlp_engine = NlpEngineProvider(nlp_configuration=nlp_configuration).create_engine()
    analyzer = AnalyzerEngine(nlp_engine=nlp_engine, supported_languages=supported_languages)

    # Store primary language for later use in analyze_text
    global analyzer_language
    analyzer_language = supported_languages[0]
    logger.info(f"Primary analysis language: {analyzer_language}")

    # Add XLM-RoBERTa recognizer if requested
    if ner_engine == "xlmr":
        try:
            from xlm_roberta_recognizer import XLMRobertaRecognizer
            xlmr_device = int(os.getenv("XLMR_NER_DEVICE", "-1"))
            xlmr_model = os.getenv("XLMR_NER_MODEL", "hrl")

            xlmr_recognizer = XLMRobertaRecognizer(
                supported_language=analyzer_language,
                model_name=xlmr_model,
                device=xlmr_device,
            )
            analyzer.registry.add_recognizer(xlmr_recognizer)
            logger.info(f"✅ XLM-RoBERTa NER enabled (model={xlmr_model}, device={xlmr_device})")
            logger.info("   XLM-RoBERTa provides accurate multilingual entity detection")

        except ImportError as e:
            logger.warning(f"XLM-RoBERTa NER not available: {e}")
            logger.warning("Install with: pip install transformers torch")
            logger.warning("Falling back to pattern-based Chinese recognizers")
            ner_engine = "spacy"  # Fall back

    # Add custom pattern-based recognizers (supplement XLM-RoBERTa or primary for spacy mode)
    if enable_chinese:
        # Add pattern-based recognizers for formats XLM-RoBERTa doesn't detect
        # (phone numbers, ID cards, etc.)
        chinese_phone = ChinesePhoneRecognizer()
        chinese_phone.supported_language = analyzer_language
        chinese_id = ChineseIDCardRecognizer()
        chinese_id.supported_language = analyzer_language

        analyzer.registry.add_recognizer(chinese_phone)
        analyzer.registry.add_recognizer(chinese_id)
        logger.info(f"Added pattern-based recognizers (phone, ID card)")

        # Only add pattern-based name recognizer if NOT using XLM-RoBERTa
        # (XLM-RoBERTa handles names better)
        if ner_engine != "xlmr":
            chinese_name = ChineseNameRecognizer(supported_language=analyzer_language)
            analyzer.registry.add_recognizer(chinese_name)
            logger.info("Added pattern-based Chinese name recognizer (fallback mode)")

    # Add international recognizers (always enabled)
    intl_phone = InternationalPhoneRecognizer()
    intl_phone.supported_language = analyzer_language
    analyzer.registry.add_recognizer(intl_phone)

    url_recognizer = URLRecognizer()
    url_recognizer.supported_language = analyzer_language
    analyzer.registry.add_recognizer(url_recognizer)

    ssn_recognizer = USSSNRecognizer()
    ssn_recognizer.supported_language = analyzer_language
    analyzer.registry.add_recognizer(ssn_recognizer)

    plate_recognizer = VehiclePlateRecognizer()
    plate_recognizer.supported_language = analyzer_language
    analyzer.registry.add_recognizer(plate_recognizer)

    passport_recognizer = PassportRecognizer()
    passport_recognizer.supported_language = analyzer_language
    analyzer.registry.add_recognizer(passport_recognizer)

    # Add API key recognizer (detects 30+ provider API keys)
    try:
        from api_key_recognizer import APIKeyRecognizer
        api_key_recognizer = APIKeyRecognizer(supported_language=analyzer_language)
        analyzer.registry.add_recognizer(api_key_recognizer)
        logger.info("✅ API Key Recognizer enabled (30+ providers: OpenAI, AWS, GitHub, etc.)")
    except ImportError as e:
        logger.warning(f"API Key Recognizer not available: {e}")

    logger.info("Added recognizers: international phone, URL, US SSN, vehicle plates, passport, API keys")

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
    # Custom entities
    "URL",
    "VEHICLE_PLATE",
    "CN_ID_CARD",
    "PASSPORT",
    "API_KEY",  # Developer API keys (OpenAI, AWS, GitHub, etc.)
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
    "URL": "[URL]",
    "VEHICLE_PLATE": "[PLATE]",
    "PASSPORT": "[PASSPORT]",
    # Chinese specific
    "CN_ID_CARD": "[CNID]",
    # Developer API keys
    "API_KEY": "[API_KEY]",
}

# Severity mapping
SEVERITY_MAP = {
    "CREDIT_CARD": "critical",
    "US_SSN": "critical",
    "US_PASSPORT": "critical",
    "PASSPORT": "critical",  # International passport numbers
    "CN_ID_CARD": "critical",  # Chinese ID card is critical PII
    "API_KEY": "critical",  # API keys are critical secrets
    "CRYPTO": "high",
    "EMAIL_ADDRESS": "medium",
    "PHONE_NUMBER": "medium",
    "URL": "low",
    "VEHICLE_PLATE": "medium",
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
    language: str = None,
    entities: List[str] = None,
    threshold: float = 0.5
) -> List[RecognizerResult]:
    """
    Analyze text for PII entities using Presidio.

    Supports multilingual detection:
    - Uses the configured language (xx for multilingual, en for English-only)
    - Pattern-based recognizers work on any text regardless of language
    - Chinese names, phones, and ID cards are detected via custom recognizers
    """
    if not text or not analyzer_engine:
        return []

    # Use global analyzer_language if not specified
    if language is None:
        language = analyzer_language

    if entities is None:
        entities = PII_ENTITIES + ["CN_ID_CARD"]  # Add Chinese ID card

    try:
        # Run all recognizers using the configured language
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


def generate_unique_token(entity_type: str, original_text: str, start: int = 0, end: int = 0) -> str:
    """
    Generate a unique token for a PII entity.

    The token format is: [TYPE_hash] where hash is a 6-character hex string
    derived from the original text only (not position).

    Examples:
        - 张三 → [PERSON_a7f3e2] (same token every time "张三" appears)
        - 李四 → [PERSON_b8c4d1]
        - test@example.com → [EMAIL_c9d5f3]

    This ensures:
    1. Same PII value always gets the same token (deterministic, position-independent)
    2. Different PII values get different tokens (unique)
    3. Tokens are human-readable and indicate the PII type
    4. Multiple occurrences of the same name use the same token
    """
    # Create hash from original text only - same value = same token
    hash_input = f"{entity_type}:{original_text}"
    hash_str = hashlib.md5(hash_input.encode('utf-8')).hexdigest()[:6]

    # Map entity types to shorter display names
    type_names = {
        "CREDIT_CARD": "CC",
        "EMAIL_ADDRESS": "EMAIL",
        "PHONE_NUMBER": "PHONE",
        "US_SSN": "SSN",
        "IP_ADDRESS": "IP",
        "PERSON": "PERSON",
        "LOCATION": "LOC",
        "US_PASSPORT": "PASSPORT",
        "PASSPORT": "PASSPORT",  # International passport numbers
        "US_DRIVER_LICENSE": "DL",
        "CRYPTO": "CRYPTO",
        "CN_ID_CARD": "CNID",
        "URL": "URL",
        "VEHICLE_PLATE": "PLATE",
        "IBAN_CODE": "IBAN",
        "API_KEY": "APIKEY",  # Developer API keys
    }

    type_name = type_names.get(entity_type, entity_type)
    return f"[{type_name}_{hash_str}]"


def anonymize_text_with_unique_tokens(
    text: str,
    analyzer_results: List[RecognizerResult],
) -> Tuple[str, Dict[str, str]]:
    """
    Anonymize text using unique tokens for each PII entity.

    Returns:
        Tuple of (anonymized_text, token_mapping)
        - anonymized_text: Text with PII replaced by unique tokens
        - token_mapping: Dict mapping tokens to original values

    Example:
        Input: "张三和李四的邮箱分别是a@test.com和b@test.com"
        Output: (
            "[PERSON_a7f3e2]和[PERSON_b8c4d1]的邮箱分别是[EMAIL_c9d5f3]和[EMAIL_d0e6g4]",
            {
                "[PERSON_a7f3e2]": "张三",
                "[PERSON_b8c4d1]": "李四",
                "[EMAIL_c9d5f3]": "a@test.com",
                "[EMAIL_d0e6g4]": "b@test.com"
            }
        )
    """
    if not text or not analyzer_results:
        return text, {}

    # Remove overlapping entities - keep the one with higher confidence or longer span
    # Sort by start position, then by length (longer first), then by confidence (higher first)
    sorted_results = sorted(
        analyzer_results,
        key=lambda x: (x.start, -(x.end - x.start), -x.score)
    )

    # Filter out overlapping entities
    filtered_results = []
    last_end = -1
    for result in sorted_results:
        if result.start >= last_end:
            filtered_results.append(result)
            last_end = result.end
        # Skip overlapping entities

    # Now sort by start position in reverse order for replacement
    filtered_results = sorted(filtered_results, key=lambda x: x.start, reverse=True)

    token_mapping = {}
    result_text = text

    for result in filtered_results:
        original_value = text[result.start:result.end]
        token = generate_unique_token(
            result.entity_type,
            original_value,
            result.start,
            result.end
        )

        # Store mapping for potential restoration
        token_mapping[token] = original_value

        # Replace in text
        result_text = result_text[:result.start] + token + result_text[result.end:]

    return result_text, token_mapping


def anonymize_text(
    text: str,
    analyzer_results: List[RecognizerResult],
    redact: bool = True
) -> str:
    """
    Anonymize/redact PII in text.

    When redact=True, uses unique tokens like [PERSON_a7f3e2] for each PII entity.
    This allows different PII values of the same type to be distinguished and
    potentially restored later.
    """
    if not text or not analyzer_results or not anonymizer_engine:
        return text

    if redact:
        # Use unique token replacement
        anonymized_text, _ = anonymize_text_with_unique_tokens(text, analyzer_results)
        return anonymized_text

    try:
        # Non-redact mode: mask with asterisks using Presidio
        operators = {}
        for result in analyzer_results:
            entity_type = result.entity_type
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
    """Convert Presidio results to RedactedEntity objects with unique tokens"""
    entities = []
    for result in results:
        original_value = text[result.start:result.end]
        # Generate unique token for this entity
        mask = generate_unique_token(
            result.entity_type,
            original_value,
            result.start,
            result.end
        )
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
