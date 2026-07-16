import yaml
from .layer1_layout import LayoutAnalyzer
from .layer2_ner import NERAnalyzer
from .layer3_validator import DeterministicValidator
from .layer4_adversarial import AdversarialFilter
from .layer5_llm import LLMJudge
from .layer6_synthesizer import ConfidenceSynthesizer
from ..api.schemas import PIIContext, ValidatedMatch, LayerResult

class PipelineManager:
    def __init__(self, config_path: str = "config/settings.yaml"):
        # Load Config
        with open(config_path, 'r') as f:
            self.config = yaml.safe_load(f)
            
        # Initialize Layers
        self.l1 = LayoutAnalyzer()
        self.l2 = NERAnalyzer()
        self.l3 = DeterministicValidator()
        self.l4 = AdversarialFilter()
        self.l5 = LLMJudge()
        self.l6 = ConfidenceSynthesizer(self.config.get('weights', {}))

    def process_batch(self, candidates: list[PIIContext]) -> list[ValidatedMatch]:
        results = []
        for cand in candidates:
            # Extract Actual PII Value from Context
            try:
                pii_value = cand.text_segment[cand.start_index:cand.end_index]
            except IndexError:
                # Fallback if indices are wonky
                pii_value = cand.text_segment

            # 1. Structural Analysis (Uses Context)
            s1 = self.l1.analyze(cand.text_segment, cand.metadata)
            
            # 2. NER Analysis (Uses Context)
            s2 = self.l2.analyze(cand.text_segment)
            
            # 3. Deterministic Validation (Uses PII Value)
            # Assume we rely on metadata for "pii_type" hints, default to credit_card
            pii_type = cand.metadata.get("pii_type", "credit_card")
            s3 = self.l3.validate(pii_value, pii_type)
            
            # Critical Fail: If deterministic check fails (0.0), we can short-circuit or penalize heavily
            if s3 == 0.0:
                s4, s5 = 0.0, 0.0
            else:
                # 4. Adversarial Filter (Uses Context + Entropy of PII)
                entropy = self.l4.calculate_entropy(pii_value)
                s4 = self.l4.analyze(cand.text_segment, entropy, cand.file_path)
                
                # 5. Semantic Judge (Uses Context)
                s5 = self.l5.judge(cand.text_segment, pii_value, pii_type)

            # 6. Synthesis
            scores = {
                "layer1_layout": s1,
                "layer2_ner": s2,
                "layer3_validator": s3,
                "layer4_adversarial": s4,
                "layer5_llm": s5
            }
            
            final_conf, vector = self.l6.synthesize(scores, cand.text_segment)
            
            # Prepare Result
            is_valid = final_conf >= self.config['thresholds']['min_confidence']
            
            vm = ValidatedMatch(
                original_text=cand.text_segment,
                pii_type=pii_type,
                confidence_score=final_conf,
                vector_embedding=vector,
                is_valid=is_valid,
                layer_breakdown=[
                    LayerResult(layer_name=k, score=v, decision="pass" if v > 0.5 else "fail", details={})
                    for k,v in scores.items()
                ]
            )
            results.append(vm)
            
        return results
