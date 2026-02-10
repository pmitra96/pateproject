#!/usr/bin/env python3
"""
Test PDF extraction using Python libraries to compare with Go implementation.
"""

import sys
import json
import re

try:
    import pdfplumber
except ImportError:
    print("Installing pdfplumber...")
    import subprocess
    subprocess.check_call([sys.executable, "-m", "pip", "install", "pdfplumber"])
    import pdfplumber


def extract_with_pdfplumber(pdf_path):
    """Extract text and tables from PDF using pdfplumber."""
    print(f"\n{'='*60}")
    print(f"Extracting: {pdf_path}")
    print(f"{'='*60}\n")
    
    with pdfplumber.open(pdf_path) as pdf:
        print(f"Total pages: {len(pdf.pages)}\n")
        
        for page_num, page in enumerate(pdf.pages, 1):
            print(f"--- Page {page_num} ---\n")
            
            # Extract text
            text = page.extract_text()
            if text:
                print("TEXT CONTENT:")
                print(text[:500])  # First 500 chars
                print("\n")
            
            # Extract tables
            tables = page.extract_tables()
            if tables:
                print(f"TABLES FOUND: {len(tables)}\n")
                for i, table in enumerate(tables, 1):
                    print(f"Table {i}:")
                    for row in table[:5]:  # First 5 rows
                        print(row)
                    print()
            
            # Try to extract structured data
            print("WORDS (first 20):")
            words = page.extract_words()
            for word in words[:20]:
                print(f"  '{word['text']}' at x={word['x0']:.1f}, y={word['top']:.1f}")
            print()


def parse_zepto_items(pdf_path):
    """Attempt to parse Zepto image items."""
    items = []
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            
            for table in tables:
                if not table:
                    continue
                
                # Try to find header row
                header_idx = None
                for i, row in enumerate(table):
                    if row and any(cell and 'description' in str(cell).lower() for cell in row):
                        header_idx = i
                        break
                
                if header_idx is None:
                    continue
                
                # Extract items
                for row in table[header_idx + 1:]:
                    if not row or not any(row):
                        continue
                    
                    # Skip total rows
                    row_text = ' '.join(str(cell) for cell in row if cell)
                    if 'total' in row_text.lower():
                        break
                    
                    # Try to extract item info
                    item = {
                        'raw_row': row,
                        'description': None,
                        'quantity': None,
                        'unit': None
                    }
                    
                    # Find description (usually first non-empty cell)
                    for cell in row:
                        if cell and len(str(cell).strip()) > 2:
                            item['description'] = str(cell).strip()
                            break
                    
                    # Look for quantity patterns
                    for cell in row:
                        if cell:
                            # Try to find numbers
                            match = re.search(r'(\d+(?:\.\d+)?)\s*(kg|g|ml|l|pc|pcs)?', str(cell).lower())
                            if match:
                                item['quantity'] = float(match.group(1))
                                if match.group(2):
                                    item['unit'] = match.group(2)
                    
                    if item['description']:
                        items.append(item)
    
    return items


if __name__ == "__main__":
    pdf_path = "zepto.pdf"
    
    # First, show raw extraction
    extract_with_pdfplumber(pdf_path)
    
    # Then try to parse items
    print(f"\n{'='*60}")
    print("PARSED ITEMS")
    print(f"{'='*60}\n")
    
    items = parse_zepto_items(pdf_path)
    print(json.dumps(items, indent=2))
    print(f"\nTotal items found: {len(items)}")
