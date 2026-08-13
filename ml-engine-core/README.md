# ML Engine Core

The Hexa-Core Machine Learning Engine for the Compliance Manager data discovery system.

## Architecture
This engine implements a 6-layer pipeline to validate PII with extreme precision:

1.  **Layout Analysis**: Understands document structure (Header, Footer, Table).
2.  **NER (Named Entity Recognition)**: Uses Spacy/Transformers to extract entities.
3.  **Deterministic Validator**: Regex, Luhn Check, Library Check.
4.  **Adversarial Filter**: Entropy analysis and "Test Data" detection.
5.  **Semantic Judge (LLM)**: Designed to use Llama-3 (via API/Local) for ambiguous cases; no LLM client is wired up yet, so it currently always runs in heuristic-only mode (see `pipeline/layer5_llm.py`, which prints a warning on every use).
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
    pip install -r requirements-test.txt   # lightweight deps only (no torch/spacy)
    pytest tests/ -v
    ```
    The test suite exercises `PipelineManager` and the deterministic layers
    (layout, Luhn/SSN/email validation, adversarial filtering) directly and does
    not require the optional heavy ML dependencies (`spacy`, `sentence-transformers`)
    — those layers gracefully fall back to heuristics when the models aren't
    installed, which is also what CI runs against.

## Configuration
Adjust weights and thresholds in `config/settings.yaml`.
**Note**: Ensure `sentence-transformers` is installed for vector embeddings.
