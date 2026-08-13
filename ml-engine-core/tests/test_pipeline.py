"""
Pytest suite for the Hexa-Core PII validation pipeline.

Covers the PipelineManager end-to-end (via process_batch) as well as the
individual deterministic layers that don't require optional heavy ML
dependencies (spacy / sentence-transformers), which the pipeline is designed
to gracefully fall back without (see pipeline/layer2_ner.py and
pipeline/layer6_synthesizer.py).
"""

import os
import sys

import pytest

# Make the `pipeline` and `api` packages (siblings of this tests/ dir's
# parent) importable when running via `pytest` from any cwd.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from api.schemas import PIIContext  # noqa: E402
from pipeline.manager import PipelineManager  # noqa: E402
from pipeline.layer3_validator import DeterministicValidator  # noqa: E402
from pipeline.layer4_adversarial import AdversarialFilter  # noqa: E402

CONFIG_PATH = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "config",
    "settings.yaml",
)


@pytest.fixture(scope="module")
def manager() -> PipelineManager:
    return PipelineManager(CONFIG_PATH)


def test_valid_credit_card_is_detected_with_high_confidence(manager):
    valid_cc = PIIContext(
        text_segment="Please charge my card 4532 0151 1283 0366 for the transaction.",
        start_index=22,
        end_index=41,
        file_path="test_invoice.txt",
        metadata={"possible_type": "credit_card"},
    )

    result = manager.process_batch([valid_cc])[0]

    assert result.is_valid is True
    assert result.confidence_score > 0.7
    assert result.pii_type == "credit_card"
    assert len(result.layer_breakdown) == 5


def test_obvious_test_data_is_filtered_or_scored_low(manager):
    test_data = PIIContext(
        text_segment="This is a test credit card number: 4111 1111 1111 1111",
        start_index=35,
        end_index=54,
        file_path="dummy_test.txt",
        metadata={"possible_type": "credit_card"},
    )

    result = manager.process_batch([test_data])[0]

    assert result.is_valid is False or result.confidence_score < 0.5


def test_process_batch_handles_multiple_candidates_independently(manager):
    valid_cc = PIIContext(
        text_segment="Please charge my card 4532 0151 1283 0366 for the transaction.",
        start_index=22,
        end_index=41,
        file_path="invoice.txt",
        metadata={"possible_type": "credit_card"},
    )
    test_data = PIIContext(
        text_segment="This is a test credit card number: 4111 1111 1111 1111",
        start_index=35,
        end_index=54,
        file_path="dummy_test.txt",
        metadata={"possible_type": "credit_card"},
    )

    results = manager.process_batch([valid_cc, test_data])

    assert len(results) == 2
    assert results[0].is_valid is True
    assert results[1].is_valid is False


class TestDeterministicValidator:
    """Layer 3 is pure logic (Luhn / SSN / email regex) - no mocking needed."""

    def setup_method(self):
        self.validator = DeterministicValidator()

    def test_valid_luhn_credit_card_passes(self):
        assert self.validator.validate("4532015112830366", "credit_card") == 1.0

    def test_invalid_luhn_credit_card_fails(self):
        assert self.validator.validate("4532015112830367", "credit_card") == 0.0

    def test_wrong_length_credit_card_fails(self):
        assert self.validator.validate("4532", "credit_card") == 0.0

    def test_valid_ssn_passes(self):
        assert self.validator.validate("123-45-6789", "ssn") == 1.0

    def test_ssn_with_invalid_area_fails(self):
        assert self.validator.validate("000-45-6789", "ssn") == 0.0

    def test_valid_email_passes(self):
        assert self.validator.validate("someone@example.com", "email") == 1.0

    def test_invalid_email_fails(self):
        assert self.validator.validate("not-an-email", "email") == 0.0

    def test_unknown_type_returns_neutral_score(self):
        assert self.validator.validate("anything", "unknown_type") == 0.5

    def test_empty_value_fails(self):
        assert self.validator.validate("", "credit_card") == 0.0


class TestAdversarialFilter:
    def setup_method(self):
        self.filt = AdversarialFilter()

    def test_test_path_is_flagged_low_reliability(self):
        score = self.filt.analyze("some data", entropy_score=3.0, file_path="/test/fixtures/data.txt")
        assert score == 0.2

    def test_suspicious_keyword_is_flagged_low_reliability(self):
        score = self.filt.analyze("this is dummy data", entropy_score=3.0, file_path="")
        assert score == 0.1

    def test_ordinary_data_passes(self):
        score = self.filt.analyze("4532 0151 1283 0366", entropy_score=3.0, file_path="invoice.txt")
        assert score == 0.95

    def test_entropy_is_shannon_entropy(self):
        # A single repeated character has zero entropy.
        assert self.filt.calculate_entropy("aaaa") == 0.0
        # Entropy is non-negative and finite for mixed content.
        assert self.filt.calculate_entropy("4532015112830366") > 0.0
