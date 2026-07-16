class LLMJudge:
    """
    Layer 5: Semantic Judge
    Uses Llama-3 (via vLLM or API) to reason about ambiguous cases.
    """
    
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
        # In production: call self.client.generate(...)
        
        # Enhanced Heuristic Logic (when LLM is not connected):
        
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
