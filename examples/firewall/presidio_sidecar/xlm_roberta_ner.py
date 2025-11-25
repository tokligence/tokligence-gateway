#!/usr/bin/env python3
"""
XLM-RoBERTa based Multilingual NER Engine

This module provides accurate multilingual Named Entity Recognition using
Hugging Face's XLM-RoBERTa model fine-tuned for NER tasks.

Features:
- Supports 10+ languages including Chinese, English, German, French, etc.
- Accurate entity boundary detection (critical for Chinese text)
- Entity types: PER (Person), LOC (Location), ORG (Organization)
- GPU acceleration support for production use

Usage:
    from xlm_roberta_ner import XLMRobertaNER

    ner = XLMRobertaNER()
    entities = ner.extract_entities("张三今天来了，李四说他很高兴")
    # [{'text': '张三', 'type': 'PER', 'start': 0, 'end': 2, 'score': 0.99}, ...]
"""

import os
import logging
from typing import List, Dict, Any, Optional
from dataclasses import dataclass

logger = logging.getLogger(__name__)

# Lazy load transformers to avoid import errors if not installed
_pipeline = None
_tokenizer = None
_model = None


@dataclass
class NEREntity:
    """Represents a detected named entity."""
    text: str
    entity_type: str
    start: int
    end: int
    score: float

    def to_dict(self) -> Dict[str, Any]:
        return {
            "text": self.text,
            "type": self.entity_type,
            "start": self.start,
            "end": self.end,
            "score": self.score
        }


class XLMRobertaNER:
    """
    Multilingual NER using XLM-RoBERTa.

    This class provides accurate entity extraction for multiple languages,
    with proper handling of entity boundaries - critical for Chinese text
    where there are no word separators.

    Supported entity types:
    - PER: Person names
    - LOC: Locations
    - ORG: Organizations

    Environment Variables:
        XLMR_NER_MODEL: Model to use (default: Davlan/xlm-roberta-base-ner-hrl)
        XLMR_NER_DEVICE: Device to use (-1=CPU, 0=GPU:0, etc.)
        XLMR_NER_BATCH_SIZE: Batch size for inference
    """

    # Available models
    MODELS = {
        "hrl": "Davlan/xlm-roberta-base-ner-hrl",      # 10 high-resource languages
        "wikiann": "Davlan/xlm-roberta-base-wikiann-ner",  # 20 languages from WikiANN
        "multilingual": "Davlan/xlm-roberta-large-ner-hrl",  # Large model, better accuracy
    }

    # Entity type mapping (model output -> standard names)
    ENTITY_MAP = {
        "PER": "PERSON",
        "LOC": "LOCATION",
        "ORG": "ORGANIZATION",
        "MISC": "MISC",
        # Some models use B-/I- prefix
        "B-PER": "PERSON",
        "I-PER": "PERSON",
        "B-LOC": "LOCATION",
        "I-LOC": "LOCATION",
        "B-ORG": "ORGANIZATION",
        "I-ORG": "ORGANIZATION",
        "B-MISC": "MISC",
        "I-MISC": "MISC",
    }

    def __init__(
        self,
        model_name: str = None,
        device: int = None,
        batch_size: int = 8,
    ):
        """
        Initialize the XLM-RoBERTa NER engine.

        Args:
            model_name: Model identifier or key from MODELS dict
            device: Device to use (-1 for CPU, 0+ for GPU)
            batch_size: Batch size for processing multiple texts
        """
        # Get configuration from environment or parameters
        if model_name is None:
            model_name = os.getenv("XLMR_NER_MODEL", "hrl")

        if device is None:
            device = int(os.getenv("XLMR_NER_DEVICE", "-1"))

        self.batch_size = int(os.getenv("XLMR_NER_BATCH_SIZE", str(batch_size)))

        # Resolve model name
        if model_name in self.MODELS:
            self.model_id = self.MODELS[model_name]
        else:
            self.model_id = model_name

        self.device = device
        self._pipeline = None
        self._initialized = False

        logger.info(f"XLMRobertaNER configured: model={self.model_id}, device={device}")

    def _ensure_initialized(self):
        """Lazy initialization of the model."""
        if self._initialized:
            return

        try:
            from transformers import (
                AutoTokenizer,
                AutoModelForTokenClassification,
                pipeline,
            )

            logger.info(f"Loading XLM-RoBERTa NER model: {self.model_id}")

            # Load tokenizer and model
            tokenizer = AutoTokenizer.from_pretrained(self.model_id)
            model = AutoModelForTokenClassification.from_pretrained(self.model_id)

            # Create pipeline with aggregation strategy
            # "simple" merges B-XXX and I-XXX into single entities
            self._pipeline = pipeline(
                "ner",
                model=model,
                tokenizer=tokenizer,
                device=self.device,
                aggregation_strategy="simple",
            )

            self._initialized = True

            device_name = "CPU" if self.device < 0 else f"GPU:{self.device}"
            logger.info(f"XLM-RoBERTa NER initialized on {device_name}")

        except ImportError as e:
            logger.error(f"Failed to import transformers: {e}")
            logger.error("Install with: pip install transformers torch")
            raise RuntimeError("transformers library not installed") from e
        except Exception as e:
            logger.error(f"Failed to initialize XLM-RoBERTa NER: {e}")
            raise

    def extract_entities(self, text: str) -> List[NEREntity]:
        """
        Extract named entities from text.

        Args:
            text: Input text (any supported language)

        Returns:
            List of NEREntity objects with text, type, position, and confidence
        """
        if not text or not text.strip():
            return []

        self._ensure_initialized()

        try:
            # Run NER pipeline
            results = self._pipeline(text)

            # Convert to NEREntity objects
            entities = []
            for r in results:
                # Map entity type
                raw_type = r.get("entity_group", r.get("entity", "UNKNOWN"))
                entity_type = self.ENTITY_MAP.get(raw_type, raw_type)

                # Get position and text
                start = r.get("start", 0)
                end = r.get("end", 0)
                entity_text = r.get("word", text[start:end])

                # Clean up entity text (remove ## from subword tokens)
                entity_text = entity_text.replace("##", "").strip()

                # Skip empty entities
                if not entity_text:
                    continue

                entity = NEREntity(
                    text=entity_text,
                    entity_type=entity_type,
                    start=start,
                    end=end,
                    score=r.get("score", 0.0)
                )
                entities.append(entity)

            return entities

        except Exception as e:
            logger.error(f"Error extracting entities: {e}")
            return []

    def extract_entities_batch(self, texts: List[str]) -> List[List[NEREntity]]:
        """
        Extract entities from multiple texts (batch processing).

        Args:
            texts: List of input texts

        Returns:
            List of entity lists, one per input text
        """
        if not texts:
            return []

        self._ensure_initialized()

        try:
            # Run NER pipeline on batch
            all_results = self._pipeline(texts)

            # If single text, wrap in list
            if texts and len(texts) == 1:
                all_results = [all_results]

            # Convert each result set
            batch_entities = []
            for text, results in zip(texts, all_results):
                entities = []
                for r in results:
                    raw_type = r.get("entity_group", r.get("entity", "UNKNOWN"))
                    entity_type = self.ENTITY_MAP.get(raw_type, raw_type)

                    start = r.get("start", 0)
                    end = r.get("end", 0)
                    entity_text = r.get("word", text[start:end])
                    entity_text = entity_text.replace("##", "").strip()

                    if not entity_text:
                        continue

                    entity = NEREntity(
                        text=entity_text,
                        entity_type=entity_type,
                        start=start,
                        end=end,
                        score=r.get("score", 0.0)
                    )
                    entities.append(entity)

                batch_entities.append(entities)

            return batch_entities

        except Exception as e:
            logger.error(f"Error in batch entity extraction: {e}")
            return [[] for _ in texts]

    def is_available(self) -> bool:
        """Check if the NER engine is available."""
        try:
            import transformers
            return True
        except ImportError:
            return False

    def get_model_info(self) -> Dict[str, Any]:
        """Get information about the loaded model."""
        return {
            "model_id": self.model_id,
            "device": "CPU" if self.device < 0 else f"GPU:{self.device}",
            "initialized": self._initialized,
            "entity_types": list(set(self.ENTITY_MAP.values())),
            "batch_size": self.batch_size,
        }


# Singleton instance for reuse
_ner_instance: Optional[XLMRobertaNER] = None


def get_ner_engine() -> XLMRobertaNER:
    """Get the singleton NER engine instance."""
    global _ner_instance
    if _ner_instance is None:
        _ner_instance = XLMRobertaNER()
    return _ner_instance


def extract_entities(text: str) -> List[Dict[str, Any]]:
    """
    Convenience function to extract entities from text.

    Args:
        text: Input text

    Returns:
        List of entity dictionaries
    """
    ner = get_ner_engine()
    entities = ner.extract_entities(text)
    return [e.to_dict() for e in entities]


# Test function
if __name__ == "__main__":
    import sys

    # Configure logging
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    # Test texts
    test_texts = [
        # Chinese
        "张三今天来了，张三说他很高兴，李四也来了。",
        "北京市朝阳区的腾讯公司招聘工程师。",
        # English
        "John Smith works at Google in New York City.",
        "Angela Merkel visited Paris to meet Emmanuel Macron.",
        # German
        "Angela Merkel traf Emmanuel Macron in Berlin.",
        # Mixed
        "张伟 is meeting with John at 北京 tomorrow.",
    ]

    print("=" * 60)
    print("XLM-RoBERTa Multilingual NER Test")
    print("=" * 60)

    ner = XLMRobertaNER()

    for text in test_texts:
        print(f"\nInput: {text}")
        print("-" * 40)

        entities = ner.extract_entities(text)

        if entities:
            for e in entities:
                print(f"  [{e.entity_type}] '{e.text}' (pos: {e.start}-{e.end}, score: {e.score:.3f})")
        else:
            print("  No entities found")

    print("\n" + "=" * 60)
    print("Model Info:", ner.get_model_info())
