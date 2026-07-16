import math
from typing import Dict, Any

class AdversarialFilter:
    """
    Layer 4: Adversarial Filter
    Detects non-PII data that mimics PII structure (e.g., test data, encoded strings).
    Uses Shannon Entropy and keyword heuristics.
    """
    
    # Common test values often found in codebases
    SUSPICIOUS_KEYWORDS = {
        "test", "dummy", "example", "sample", "mock", 
        "fake", "placeholder", "todo", "fixme"
    }

    def analyze(self, text: str, entropy_score: float, file_path: str = "") -> float:
        """
        Analyzes the text for adversarial signs.
        Args:
            text: The text segment to analyze.
            entropy_score: The pre-calculated entropy of the text.
            file_path: The path of the file containing the text.
        Returns:
            float: Reliability score (1.0 = Real Data, 0.0 = Fake/Adversarial)
        """
        if not text:
            return 0.0

        lower_text = text.lower()

        # 1. File Path Heuristics (NEW)
        # If the file path indicates a test environment, reduce confidence significantly
        if file_path:
            lower_path = file_path.lower()
            if any(k in lower_path for k in ["/test/", "\\test\\", "_test.", ".test.", "spec.", "mock"]):
                return 0.2 # Likely test data

        # 2. Keyword Check (Heuristic)
        # If the text itself or immediate context contains suspicious words
        for keyword in self.SUSPICIOUS_KEYWORDS:
            if keyword in lower_text:
                return 0.1 # Highly likely to be fake

        # 3. Entropy Check (Statistical)
        # High entropy (> 5.5 in short strings) often indicates random generation or encryption
        # Credit cards and SSNs have relatively low entropy compared to hashes
        if entropy_score > 6.0:
            return 0.2 # Likely random string or hash
            
        # 4. Character Distribution Check
        # PII usually doesn't have homogenous structure like "AAAAA" or "12345"
        if len(set(lower_text)) == 1:
             return 0.1 # Repetitive content

        return 0.95 # Passes potential adversarial checks

    def calculate_entropy(self, text: str) -> float:
        """Calculates Shannon Entropy for a given string."""
        if not text:
            return 0.0
            
        prob = [text.count(c) / len(text) for c in set(text)]
        return -sum(p * math.log2(p) for p in prob)
