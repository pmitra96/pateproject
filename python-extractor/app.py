"""
PDF Extraction Microservice

FastAPI service that extracts structured data from grocery receipt PDFs
using pdfplumber. Supports multiple providers (Zepto, Blinkit, etc.).
"""

import re
from typing import List, Optional
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
import pdfplumber
from pydantic import BaseModel
import tempfile
import os


app = FastAPI(title="PDF Extraction Service", version="1.0.0")


class ExtractedItem(BaseModel):
    name: str
    count: float
    unit_value: float
    unit: str


class ExtractionResult(BaseModel):
    provider: str
    items: List[ExtractedItem]


def parse_unit_and_value(text: str) -> tuple[float, str]:
    """
    Extract unit value and unit from text.
    
    Examples:
        "(1kg)" -> (1000, "g")
        "500g" -> (500, "g")
        "1 pc" -> (1, "pc")
    """
    text = text.lower()
    
    # Look for parenthetical units like (1kg), (500g)
    match = re.search(r'\((\d+(?:\.\d+)?)\s*(g|kg|ml|l|pc|pcs)\)', text)
    if match:
        val = float(match.group(1))
        unit = match.group(2)
        
        # Normalize units
        if unit == "pcs":
            unit = "pc"
        if unit == "kg":
            return val * 1000, "g"
        if unit == "l":
            return val * 1000, "ml"
        
        return val, unit
    
    # Look for regular patterns like 500g, 1 kg
    match = re.search(r'(\d+(?:\.\d+)?)\s*(g|kg|ml|l|pc|pcs)', text)
    if match:
        val = float(match.group(1))
        unit = match.group(2)
        
        # Normalize units
        if unit == "pcs":
            unit = "pc"
        if unit == "kg":
            return val * 1000, "g"
        if unit == "l":
            return val * 1000, "ml"
        
        return val, unit
    
    return 1, "pcs"


def clean_item_name(name: str, unit_value: float, unit: str) -> str:
    """Clean item name by removing unit information and extra text."""
    # Remove parenthetical units
    name = re.sub(r'\([^)]*?(kg|g|ml|l|pc|pcs)[^)]*?\)', '', name, flags=re.IGNORECASE)
    
    # Remove standalone units
    name = re.sub(r'\d+\.?\d*\s*(g|kg|ml|l|pc|pcs)', '', name, flags=re.IGNORECASE)
    
    # Remove extra whitespace and newlines
    name = re.sub(r'\s+', ' ', name)
    name = name.strip()
    
    return name


def detect_provider(text: str) -> str:
    """Detect the provider from PDF text."""
    text_lower = text.lower()
    
    if "zepto" in text_lower or "geddit convenience" in text_lower:
        return "zepto"
    elif "blinkit" in text_lower or "grofers" in text_lower:
        return "blinkit"
    elif "swiggy" in text_lower or "instamart" in text_lower:
        return "swiggy"
    
    return "unknown"


def extract_from_zepto(pdf_path: str) -> ExtractionResult:
    """Extract items from Zepto PDF."""
    items = []
    provider = "zepto"
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            # Detect provider from text
            text = page.extract_text()
            if text:
                provider = detect_provider(text)
            
            # Extract tables
            tables = page.extract_tables()
            
            for table in tables:
                if not table or len(table) < 2:
                    continue
                
                # Find header row
                header_idx = None
                qty_col_idx = None
                desc_col_idx = None
                
                for i, row in enumerate(table):
                    if not row:
                        continue
                    
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    
                    # Look for header with "description" and "qty"
                    if 'description' in row_text or 'item' in row_text:
                        header_idx = i
                        
                        # Find column indices
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower:
                                    desc_col_idx = j
                                if 'qty' in cell_lower or 'quantity' in cell_lower:
                                    qty_col_idx = j
                        break
                
                if header_idx is None or desc_col_idx is None:
                    continue
                
                # Extract items
                for row in table[header_idx + 1:]:
                    if not row or not any(row):
                        continue
                    
                    # Skip total rows
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'subtotal' in row_text:
                        break
                    
                    # Get description
                    description = str(row[desc_col_idx]) if desc_col_idx < len(row) and row[desc_col_idx] else None
                    if not description or len(description.strip()) < 2:
                        continue
                    
                    # Get quantity
                    qty = 1.0
                    if qty_col_idx is not None and qty_col_idx < len(row) and row[qty_col_idx]:
                        try:
                            qty = float(str(row[qty_col_idx]).strip())
                        except ValueError:
                            qty = 1.0
                    
                    # Parse unit info from description
                    unit_value, unit = parse_unit_and_value(description)
                    
                    # Clean name
                    clean_name = clean_item_name(description, unit_value, unit)
                    
                    if clean_name:
                        items.append(ExtractedItem(
                            name=clean_name,
                            count=qty,
                            unit_value=unit_value,
                            unit=unit
                        ))
    
    return ExtractionResult(provider=provider, items=items)


@app.post("/extract", response_model=ExtractionResult)
async def extract_pdf(file: UploadFile = File(...)):
    """
    Extract items from a PDF receipt.
    
    Accepts a PDF file and returns structured item data.
    """
    if not file.filename.endswith('.pdf'):
        raise HTTPException(status_code=400, detail="File must be a PDF")
    
    # Save to temp file
    with tempfile.NamedTemporaryFile(delete=False, suffix='.pdf') as tmp:
        content = await file.read()
        tmp.write(content)
        tmp_path = tmp.name
    
    try:
        # Extract items
        result = extract_from_zepto(tmp_path)
        return result
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")
    finally:
        # Clean up temp file
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)


@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "service": "pdf-extractor"}


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8081)
