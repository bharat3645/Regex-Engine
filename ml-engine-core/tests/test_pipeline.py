
import sys
import os
import asyncio
from typing import Dict, List

# Add parent directory to path to import modules
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from pipeline.manager import PipelineManager
from api.schemas import PIIContext

async def test_pipeline():
    print("Initializing Pipeline...")
    manager = PipelineManager()
    await manager.initialize()
    print("Pipeline Initialized.")

    # Test Case 1: Valid Credit Card
    valid_cc = PIIContext(
        text_segment="Please charge my card 4532 0151 1283 0366 for the transaction.",
        start_index=22,
        end_index=41,
        file_path="test_invoice.txt",
        metadata={"possible_type": "credit_card"}
    )

    # Test Case 2: Test Data (Should be filtered/low score)
    test_data = PIIContext(
        text_segment="This is a test credit card number: 4111 1111 1111 1111",
        start_index=35,
        end_index=54,
        file_path="dummy_test.txt",
        metadata={"possible_type": "credit_card"}
    )
    
    print("\nRunning Scan on Valid CC...")
    result1 = await manager.scan(valid_cc)
    print(f"Result 1: Valid={result1.is_valid}, Score={result1.confidence_score:.4f}, Type={result1.pii_type}")
    print(f"Vector (First 5 dims): {result1.vector_embedding[:5]}")

    print("\nRunning Scan on Test Data...")
    result2 = await manager.scan(test_data)
    print(f"Result 2: Valid={result2.is_valid}, Score={result2.confidence_score:.4f}, Type={result2.pii_type}")
    
    # Assertions
    if result1.is_valid and result1.confidence_score > 0.7:
        print("\n[PASS] Valid CC detected correctly.")
    else:
        print("\n[FAIL] Valid CC detection failed.")

    if not result2.is_valid or result2.confidence_score < 0.5:
         print("[PASS] Test Data filtered/low score correctly.")
    else:
         print(f"[WARN] Test Data validation weak (Score: {result2.confidence_score})")

if __name__ == "__main__":
    asyncio.run(test_pipeline())
