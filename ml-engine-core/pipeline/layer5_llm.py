class LLMJudge:
    """
    Layer 5: Semantic Judge
    Aspirationally designed to call Llama-3 (via vLLM or API) to reason about
    ambiguous cases. No LLM client is wired up yet, so this layer always runs
    in heuristic-only mode -- unlike Layer 2 (NER) and Layer 6 (embeddings),
    which attempt a real model and only fall back on missing dependencies,
    this layer never attempts a real call at all, so the warning below fires
    unconditionally on every instantiation rather than being gated behind a
    try/except.
    """

    def __init__(self):
        print("Warning: Semantic Judge: no LLM configured, running in heuristic-only mode.")

    PROMPT_TEMPLATE = """
    Analyze the following text segment for Personally Identifiable Information (PII).
    Context: {context}
    Candidate: {candidate}
    
    Q: Is this a real {pii_type} or a false positive? 
    Reason based on the surrounding context (e.g., variable names, sentence structure).
    Return strict JSON: {{"is_real": boolean, "confidence": float, "reason": string}}
    """

    def judge(self, context: str, candidate: str, pii_type: str) -> float:
        """
        Returns confidence score 0.0-1.0 from LLM.
        """
        # Not yet implemented: no LLM client exists to call here (e.g.
        # self.client.generate(...) against vLLM/an API). See the class
        # docstring and __init__ warning -- this always falls through to
        # heuristics below, never a real model.

        # Heuristic Logic (this layer is heuristic-only, see above):
        
        context_lower = context.lower()
        
        # Generic strong indicators
        if any(w in context_lower for w in ["example", "test", "mock", "dummy"]):
             return 0.1 # Likely false positive

        if pii_type == "credit_card":
            if any(w in context_lower for w in ["visa", "mastercard", "amex", "cc", "card", "payment", "billing"]):
                return 0.95
            if "id" in context_lower: # "Client ID" often looks like a number
                return 0.4

        elif pii_type == "ssn":
            if any(w in context_lower for w in ["social", "security", "tax", "ssn", "tin"]):
                return 0.95
            if "phone" in context_lower or "fax" in context_lower:
                return 0.2

        elif pii_type == "email":
            if "mailto" in context_lower or "contact" in context_lower:
                return 0.95
        
        # Default moderate confidence if no strong signals
        return 0.6
