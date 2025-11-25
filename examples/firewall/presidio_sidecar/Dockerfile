FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Download spaCy multilingual model (for fallback)
RUN python -m spacy download xx_ent_wiki_sm

# Pre-download XLM-RoBERTa model (optional, will download at runtime if not present)
# This speeds up container startup but increases image size (~1GB)
RUN python -c "from transformers import pipeline; pipeline('ner', model='Davlan/xlm-roberta-base-ner-hrl')" || true

# Copy application files
COPY main.py .
COPY xlm_roberta_ner.py .
COPY xlm_roberta_recognizer.py .

# Expose port (matches local development)
EXPOSE 7317

# Environment variables for XLM-RoBERTa (default)
ENV PRESIDIO_NER_ENGINE=xlmr
ENV XLMR_NER_DEVICE=-1
ENV XLMR_NER_MODEL=hrl
ENV PRESIDIO_PORT=7317
ENV PRESIDIO_WORKERS=1
ENV PRESIDIO_LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=120s --retries=3 \
    CMD curl -f http://localhost:7317/health || exit 1

# Run the application
CMD ["python", "main.py"]
