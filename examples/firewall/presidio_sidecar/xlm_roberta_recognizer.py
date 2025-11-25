#!/usr/bin/env python3
"""
XLM-RoBERTa based Presidio Recognizer

This module provides a Presidio-compatible recognizer that uses XLM-RoBERTa
for multilingual Named Entity Recognition. It integrates seamlessly with
the existing Presidio analyzer pipeline.

Features:
- Accurate entity boundary detection for all supported languages
- Proper handling of Chinese text (no word boundaries)
- Integration with Presidio's analyzer engine
- Support for PER, LOC, ORG entity types

Usage:
    from xlm_roberta_recognizer import XLMRobertaRecognizer

    recognizer = XLMRobertaRecognizer()
    analyzer.registry.add_recognizer(recognizer)
"""

import logging
from typing import List, Optional

from presidio_analyzer import EntityRecognizer, RecognizerResult

from xlm_roberta_ner import XLMRobertaNER, NEREntity

logger = logging.getLogger(__name__)


class XLMRobertaRecognizer(EntityRecognizer):
    """
    Presidio EntityRecognizer that uses XLM-RoBERTa for NER.

    This recognizer provides accurate multilingual NER with proper
    entity boundary detection, which is critical for Chinese text.

    Entity type mapping:
    - PER -> PERSON (Presidio standard)
    - LOC -> LOCATION
    - ORG -> ORGANIZATION
    """

    # Map XLM-RoBERTa entity types to Presidio entity types
    PRESIDIO_ENTITY_MAP = {
        "PERSON": "PERSON",
        "LOCATION": "LOCATION",
        "ORGANIZATION": "ORG",  # Presidio uses "ORG" not "ORGANIZATION"
        "MISC": "MISC",
    }

    # Supported Presidio entities
    SUPPORTED_ENTITIES = ["PERSON", "LOCATION", "ORG"]

    def __init__(
        self,
        supported_language: str = "xx",  # "xx" = multilingual
        model_name: str = None,
        device: int = None,
        min_score: float = 0.5,
    ):
        """
        Initialize the XLM-RoBERTa recognizer.

        Args:
            supported_language: Language code ("xx" for multilingual)
            model_name: XLM-RoBERTa model to use
            device: Device for inference (-1=CPU, 0+=GPU)
            min_score: Minimum confidence score to accept
        """
        # Initialize attributes BEFORE calling super().__init__
        # because super().__init__ calls load() which needs these
        self.min_score = min_score
        self._ner_engine = None
        self._model_name = model_name
        self._device = device

        super().__init__(
            supported_entities=self.SUPPORTED_ENTITIES,
            supported_language=supported_language,
            name="XLMRobertaRecognizer",
        )

        logger.info(f"XLMRobertaRecognizer initialized (language={supported_language})")

    def load(self) -> None:
        """Load the XLM-RoBERTa model (called by Presidio)."""
        if self._ner_engine is None:
            self._ner_engine = XLMRobertaNER(
                model_name=self._model_name,
                device=self._device,
            )
            # Trigger model loading
            self._ner_engine._ensure_initialized()
            logger.info("XLM-RoBERTa NER model loaded")

    def analyze(
        self,
        text: str,
        entities: List[str],
        nlp_artifacts: Optional[dict] = None,
    ) -> List[RecognizerResult]:
        """
        Analyze text for named entities.

        Args:
            text: Input text to analyze
            entities: List of entity types to look for
            nlp_artifacts: NLP artifacts from Presidio (unused)

        Returns:
            List of RecognizerResult objects
        """
        # Filter requested entities to ones we support
        requested_entities = set(entities) & set(self.SUPPORTED_ENTITIES)
        if not requested_entities:
            return []

        # Ensure model is loaded
        if self._ner_engine is None:
            self.load()

        # Extract entities using XLM-RoBERTa
        ner_results = self._ner_engine.extract_entities(text)

        # Convert to Presidio results
        results = []
        for entity in ner_results:
            # Map entity type to Presidio type
            presidio_type = self.PRESIDIO_ENTITY_MAP.get(
                entity.entity_type, entity.entity_type
            )

            # Skip if not requested or below threshold
            if presidio_type not in requested_entities:
                continue
            if entity.score < self.min_score:
                continue

            result = RecognizerResult(
                entity_type=presidio_type,
                start=entity.start,
                end=entity.end,
                score=float(entity.score),  # Convert numpy.float32 to Python float
                analysis_explanation=None,
                recognition_metadata={
                    "recognizer_name": self.name,
                    "recognizer_identifier": self.id if hasattr(self, 'id') else self.name,
                    "xlmr_entity_type": entity.entity_type,
                    "xlmr_text": entity.text,
                }
            )
            results.append(result)

        return results

    def get_supported_entities(self) -> List[str]:
        """Return list of supported entity types."""
        return self.SUPPORTED_ENTITIES

    def get_model_info(self) -> dict:
        """Get information about the underlying model."""
        if self._ner_engine:
            return self._ner_engine.get_model_info()
        return {"status": "not_loaded"}


class XLMRobertaRecognizerFactory:
    """
    Factory for creating XLM-RoBERTa recognizers with different configurations.
    """

    @staticmethod
    def create_multilingual(
        device: int = -1,
        min_score: float = 0.5,
    ) -> XLMRobertaRecognizer:
        """
        Create a multilingual recognizer (10 high-resource languages).

        Supported languages: Arabic, German, English, Spanish, French,
        Italian, Latvian, Dutch, Portuguese, Chinese
        """
        return XLMRobertaRecognizer(
            supported_language="xx",
            model_name="hrl",  # High Resource Languages
            device=device,
            min_score=min_score,
        )

    @staticmethod
    def create_wikiann(
        device: int = -1,
        min_score: float = 0.5,
    ) -> XLMRobertaRecognizer:
        """
        Create a WikiANN-based recognizer (20 languages).

        Broader language coverage but potentially lower accuracy.
        """
        return XLMRobertaRecognizer(
            supported_language="xx",
            model_name="wikiann",
            device=device,
            min_score=min_score,
        )


# Test function
if __name__ == "__main__":
    import sys

    # Configure logging
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    print("=" * 60)
    print("XLM-RoBERTa Presidio Recognizer Test")
    print("=" * 60)

    # Create recognizer
    recognizer = XLMRobertaRecognizerFactory.create_multilingual()

    # Test texts
    test_texts = [
        "张三今天来了，张三说他很高兴，李四也来了。",
        "John Smith works at Google in New York City.",
        "北京市朝阳区的腾讯公司招聘工程师。",
    ]

    for text in test_texts:
        print(f"\nInput: {text}")
        print("-" * 40)

        results = recognizer.analyze(
            text=text,
            entities=["PERSON", "LOCATION", "ORG"],
        )

        if results:
            for r in results:
                entity_text = text[r.start:r.end]
                print(f"  [{r.entity_type}] '{entity_text}' "
                      f"(pos: {r.start}-{r.end}, score: {r.score:.3f})")
        else:
            print("  No entities found")

    print("\n" + "=" * 60)
    print("Model Info:", recognizer.get_model_info())
