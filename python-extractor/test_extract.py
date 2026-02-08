#!/usr/bin/env python3
"""
Test script to extract items from Blinkit PDF receipts
"""

import sys
import json
from app import extract_from_zepto, extract_pdf_interal

def main():
    if len(sys.argv) < 2:
        print("Usage: python test_extract.py <pdf_file>")
        sys.exit(1)
    
    pdf_path = sys.argv[1]
    
    print(f"Extracting from: {pdf_path}\n")
    
    result = extract_pdf_interal(pdf_path)
    
    print(f"Provider: {result.provider}")
    print(f"Total items found: {len(result.items)}\n")
    
    print("Items:")
    print("-" * 80)
    for i, item in enumerate(result.items, 1):
        print(f"{i}. {item.name}")
        print(f"   Count: {item.count}, Unit: {item.unit_value} {item.unit}")
        print()
    
    # Also output as JSON
    print("\n" + "=" * 80)
    print("JSON Output:")
    print("=" * 80)
    print(json.dumps(result.dict(), indent=2))

if __name__ == "__main__":
    main()
