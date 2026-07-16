# ML Engine Core

The Hexa-Core Machine Learning Engine for the Regex Data Discovery System.

## Architecture
This engine implements a 6-layer pipeline to validate PII with extreme precision:

1.  **Layout Analysis**: Understands document structure (Header, Footer, Table).
2.  **NER (Named Entity Recognition)**: Uses Spacy/Transformers to extract entities.
3.  **Deterministic Validator**: Regex, Luhn Check, Library Check.
4.  **Adversarial Filter**: Entropy analysis and "Test Data" detection.
5.  **Semantic Judge (LLM)**: Uses Llama-3 (via API/Local) for ambiguous cases.
6.  **Confidence Synthesizer**: Aggregates scores and generates Vector Embeddings.

## Setup

1.  **Install Dependencies**:
    ```bash
    pip install -r requirements.txt
    ```

2.  **Run Server**:
    ```bash
    python -m api.main
    ```
    The server listens on `http://localhost:8000`.

3.  **Run Tests**:
    ```bash
    python tests/test_pipeline.py
    ```

## Configuration
Adjust weights and thresholds in `config/settings.yaml`.
**Note**: Ensure `sentence-transformers` is installed for vector embeddings.
