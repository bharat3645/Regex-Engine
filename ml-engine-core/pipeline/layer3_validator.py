import re
import math

class DeterministicValidator:
    """
    Layer 3: Deterministic Validator
    Production-grade validation for PII entities using mathematical checksums and strict regex patterns.
    """

    # Compiled patterns for performance
    CREDIT_CARD_PATTERN = re.compile(r'[^0-9]')
    SSN_PATTERN = re.compile(r'^(\d{3})-?(\d{2})-?(\d{4})$') # Supports 123456789 or 123-45-6789

    def validate(self, value: str, pii_type: str = "credit_card") -> float:
        """
        Validates the format and checksum of PII.
        Returns 1.0 (Pass), 0.0 (Fail), or 0.5 (Unknown Type).
        """
        if not value:
            return 0.0

        if pii_type == "credit_card":
            return 1.0 if self._validate_credit_card(value) else 0.0
            
        if pii_type == "ssn":
            return 1.0 if self._validate_ssn(value) else 0.0
            
        # Add email validation
        if pii_type == "email":
             # Basic RFC 5322 regex approximation
             if re.match(r"^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$", value):
                 return 1.0
             return 0.0

        return 0.5  # Unknown type handled gracefully

    def _validate_credit_card(self, card_number: str) -> bool:
        """
        Validates credit card numbers using the Luhn Algorithm (Mod 10).
        Strips non-digit characters before validation.
        """
        digits_only = self.CREDIT_CARD_PATTERN.sub('', card_number)
        
        # Length check (13 to 19 digits)
        if not 13 <= len(digits_only) <= 19:
            return False
            
        # Luhn Algorithm
        total = 0
        reverse_digits = digits_only[::-1]
        for i, digit in enumerate(reverse_digits):
            n = int(digit)
            if i % 2 == 1:
                n *= 2
                if n > 9:
                    n -= 9
            total += n
            
        return total % 10 == 0

    def _validate_ssn(self, ssn: str) -> bool:
        """
        Validates US Social Security Numbers.
        Checks format and known invalid area/group numbers.
        """
        match = self.SSN_PATTERN.match(ssn)
        if not match:
            return False
            
        area, group, serial = match.groups()
        
        # Invalid Areas
        if area == '000' or area == '666' or (900 <= int(area) <= 999):
            return False
            
        # Invalid Group/Serial
        if group == '00' or serial == '0000':
            return False
            
        return True
