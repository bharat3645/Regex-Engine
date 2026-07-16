from typing import List, Dict
import numpy as np

class ConfidenceSynthesizer:
    """
    Layer 6: Confidence Synthesizer
    Aggregates votes from all layers and generates vector embeddings.
    """
    def __init__(self, weights: Dict[str, float]):
        self.weights = weights
        try:
            from sentence_transformers import SentenceTransformer
            self.embedder = SentenceTransformer('all-MiniLM-L6-v2')
        except ImportError:
            self.embedder = None
            print("Warning: sentence-transformers not installed. Using dummy embeddings.")

    def synthesize(self, layer_scores: Dict[str, float], text_context: str) -> (float, List[float]):
        """
        Returns (Final Confidence Score, Vector Embedding)
        """
        final_score = 0.0
        total_weight = 0.0
        
        # Calculate weighted average
        for layer, score in layer_scores.items():
            w = self.weights.get(layer, 0.1)
            final_score += score * w
            total_weight += w
            
        normalized_score = final_score / total_weight if total_weight > 0 else 0.0
        
        # Determine Embedding
        embedding = []
        if self.embedder:
            try:
                # encode returns ndarray
                embedding = self.embedder.encode(text_context).tolist()
            except Exception as e:
                print(f"Embedding error: {e}")
                embedding = np.random.rand(384).tolist()
        else:
             embedding = np.random.rand(384).tolist()
        
        return min(max(normalized_score, 0.0), 1.0), embedding
