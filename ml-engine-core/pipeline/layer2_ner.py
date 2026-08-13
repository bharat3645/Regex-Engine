try:
    import spacy
except ImportError:
    spacy = None
except Exception:
    spacy = None # Catch DLL load errors

from typing import Dict, Any, Optional

class NERAnalyzer:
    """
    Layer 2: Candidate Generator
    Wrapper for Spacy (TRF) and Transformer-based NER.
    Gracefully falls back to heuristics if models are missing.
    """
    def __init__(self, model_name: str = "en_core_web_trf"):
        self.nlp = None
        if spacy is None:
            print("Warning: spacy is not installed. Running in Heuristic Fallback Mode.")
            return
        try:
            self.nlp = spacy.load(model_name)
        except OSError:
            print(f"Warning: Spacy model '{model_name}' not found. Running in Heuristic Fallback Mode.")
            pass # Fallback logic handled in analyze

    def analyze(self, text: str) -> float:
        """
        Returns score 0.0-1.0 if text is a recognized entity.
        """
        if not text:
            return 0.0

        if self.nlp:
            # Real NER Inference
            try:
                doc = self.nlp(text)
                if len(doc.ents) > 0:
                    # Check if the entity type is relevant (ORG, PERSON, etc)
                    if doc.ents[0].label_ in ["PERSON", "ORG", "GPE", "CARDINAL"]:
                        return 0.9
            except Exception:
                pass # Fallback on error

        # Robust Heuristic Fallback
        # 1. Capitalization Check (for Names/Orgs)
        if text[0].isupper() and len(text) > 3:
             return 0.6
        
        # 2. Key-Value Heuristic (handled partly by Layer 1, but reinforcing here)
        # If the input is just the PII value, NER might struggle without context.
        # So we keep this layer conservative.
        
        return 0.4
