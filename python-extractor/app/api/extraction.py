import re
import logging
import sys
import tempfile
import os
from typing import List, Optional
from pathlib import Path
from fastapi import APIRouter, File, UploadFile, HTTPException
from pydantic import BaseModel
import pdfplumber

# Importing image_extractor from the root package
# Assuming the main app runs with python-extractor as root or app directory
try:
    from image_extractor import extract_items
except ImportError:
    # Handle path when running in different contexts
    sys.path.append(os.getcwd())
    from image_extractor import extract_items

logger = logging.getLogger(__name__)

router = APIRouter()

class ExtractedItem(BaseModel):
    name: str
    count: float
    unit_value: float
    unit: str

class ExtractionResult(BaseModel):
    provider: str
    items: List[ExtractedItem]

BLINKIT_EXCLUDED_ITEMS = ['handling charge', 'total']

def parse_unit_and_value(text: str) -> tuple[float, str]:
    text = text.lower()
    match = re.search(r'\((\d+(?:\.\d+)?)\s*(g|kg|ml|l|pc|pcs)\)', text)
    if match:
        val = float(match.group(1))
        unit = match.group(2)
        if unit == "pcs": unit = "pc"
        if unit == "kg": return val * 1000, "g"
        if unit == "l": return val * 1000, "ml"
        return val, unit
    
    match = re.search(r'(\d+(?:\.\d+)?)\s*(g|kg|ml|l|pc|pcs)', text)
    if match:
        val = float(match.group(1))
        unit = match.group(2)
        if unit == "pcs": unit = "pc"
        if unit == "kg": return val * 1000, "g"
        if unit == "l": return val * 1000, "ml"
        return val, unit
    return 1.0, "pcs"

def clean_item_name(name: str, unit_value: float, unit: str) -> str:
    name = re.sub(r'\([^)]*?(kg|g|ml|l|pc|pcs)[^)]*?\)', '', name, flags=re.IGNORECASE)
    name = re.sub(r'\d+\.?\d*\s*(g|kg|ml|l|pc|pcs)', '', name, flags=re.IGNORECASE)
    name = re.sub(r'\d+\.?\d*\s*()', '', name, flags=re.IGNORECASE)
    name = re.sub(r"\s*(?:\([^)]*\)\s*)+$", "", name, flags=re.IGNORECASE).strip()
    name = re.sub(r'\s+', ' ', name)
    return name.strip()

def detect_provider(pdf_path: str) -> str:
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages[:3]:
            text = page.extract_text()
            if text:
                text_lower = text.lower()
                if "zepto" in text_lower or "geddit convenience" in text_lower:
                    return "zepto"
                elif "blinkit" in text_lower or "grofers" in text_lower:
                    return "blinkit"
                elif "trolleypop" in text_lower or "first club" in text_lower:
                    return "first_club"
                else:
                    return "swiggy"
    return "unknown"

def extract_from_zepto(pdf_path: str) -> ExtractionResult:
    items = []
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            for table in tables:
                if not table or len(table) < 2: continue
                header_idx = None
                qty_col_idx = None
                desc_col_idx = None
                for i, row in enumerate(table):
                    if not row: continue
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    if 'description' in row_text or 'item' in row_text:
                        header_idx = i
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower: desc_col_idx = j
                                if 'qty' in cell_lower or 'quantity' in cell_lower: qty_col_idx = j
                        break
                if header_idx is None or desc_col_idx is None: continue
                for row in table[header_idx + 1:]:
                    if not row or not any(row): continue
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'subtotal' in row_text: break
                    description = str(row[desc_col_idx]) if desc_col_idx < len(row) and row[desc_col_idx] else None
                    if not description or len(description.strip()) < 2: continue
                    qty = 1.0
                    if qty_col_idx is not None and qty_col_idx < len(row) and row[qty_col_idx]:
                        try: qty = float(str(row[qty_col_idx]).strip())
                        except ValueError: qty = 1.0
                    unit_value, unit = parse_unit_and_value(description)
                    clean_name = clean_item_name(description, unit_value, unit)
                    if clean_name:
                        items.append(ExtractedItem(name=clean_name, count=qty, unit_value=unit_value, unit=unit))
    return ExtractionResult(provider="zepto", items=items)

def extract_from_first_club(pdf_path: str) -> ExtractionResult:
    items = []
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            for table in tables:
                if not table or len(table) < 2: continue
                header_idx = None
                particulars_col_idx = None
                for i, row in enumerate(table):
                    if not row: continue
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    if 'particulars' in row_text:
                        header_idx = i
                        for j, cell in enumerate(row):
                            if cell:
                                if 'particulars' in str(cell).lower(): particulars_col_idx = j
                        break
                if header_idx is None or particulars_col_idx is None: continue
                start_row = header_idx + 1
                if start_row < len(table) and table[start_row]:
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in table[start_row])
                    if 'rate' in row_text and 'amt' in row_text: start_row += 1
                for row in table[start_row:]:
                    if not row or not any(row): continue
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'item total' in row_text: break
                    particulars = str(row[particulars_col_idx]) if particulars_col_idx < len(row) and row[particulars_col_idx] else None
                    if not particulars or len(particulars.strip()) < 2 or particulars.strip().isdigit(): continue
                    qty = 1.0
                    qty_match = re.search(r'\s+x\s+(\d+(?:\.\d+)?)\s*$', particulars, re.IGNORECASE)
                    if qty_match:
                        qty = float(qty_match.group(1))
                        particulars = re.sub(r'\s+x\s+\d+(?:\.\d+)?\s*$', '', particulars, flags=re.IGNORECASE)
                    unit_value, unit = parse_unit_and_value(particulars)
                    clean_name = clean_item_name(particulars, unit_value, unit)
                    if clean_name:
                        items.append(ExtractedItem(name=clean_name, count=qty, unit_value=unit_value, unit=unit))
    return ExtractionResult(provider="first_club", items=items)

def extract_from_blinkit(pdf_path: str) -> ExtractionResult:
    items = []
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            for table in tables:
                if not table or len(table) < 2: continue
                header_idx = None
                qty_col_idx = None
                desc_col_idx = None
                for i, row in enumerate(table):
                    if not row: continue
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    if 'description' in row_text or 'item' in row_text or 'product' in row_text:
                        header_idx = i
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower or 'product' in cell_lower: desc_col_idx = j
                                if 'qty' in cell_lower or 'quantity' in cell_lower: qty_col_idx = j
                        break
                if header_idx is None or desc_col_idx is None: continue
                for row in table[header_idx + 1:]:
                    if not row or not any(row): continue
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'subtotal' in row_text: break
                    description = str(row[desc_col_idx]) if desc_col_idx < len(row) and row[desc_col_idx] else None
                    if not description or len(description.strip()) < 2: continue
                    qty = 1.0
                    if qty_col_idx is not None and qty_col_idx < len(row) and row[qty_col_idx]:
                        try: qty = float(str(row[qty_col_idx]).strip())
                        except ValueError: qty = 1.0
                    unit_value, unit = parse_unit_and_value(description)
                    clean_name = clean_item_name(description, unit_value, unit)
                    if clean_name.lower() not in BLINKIT_EXCLUDED_ITEMS:
                        items.append(ExtractedItem(name=clean_name, count=qty, unit_value=unit_value, unit=unit))
    return ExtractionResult(provider="blinkit", items=items)

def extract_from_swiggy(pdf_path: str) -> ExtractionResult:
    items = []
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            tables = page.extract_tables()
            for table in tables:
                if not table or len(table) < 2: continue
                header_idx = None
                qty_col_idx = None
                desc_col_idx = None
                for i, row in enumerate(table):
                    if not row: continue
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    if 'description' in row_text or 'item' in row_text or 'product' in row_text:
                        header_idx = i
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower or 'product' in cell_lower: desc_col_idx = j
                                if 'qty' in cell_lower or 'quantity' in cell_lower: qty_col_idx = j
                        break
                if header_idx is None or desc_col_idx is None: continue
                for row in table[header_idx + 1:]:
                    if not row or not any(row): continue
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'subtotal' in row_text: break
                    description = str(row[desc_col_idx]) if desc_col_idx < len(row) and row[desc_col_idx] else None
                    if not description or len(description.strip()) < 2: continue
                    qty = 1.0
                    if qty_col_idx is not None and qty_col_idx < len(row) and row[qty_col_idx]:
                        try: qty = float(str(row[qty_col_idx]).strip())
                        except ValueError: qty = 1.0
                    unit_value, unit = parse_unit_and_value(description)
                    clean_name = clean_item_name(description, unit_value, unit)
                    if clean_name:
                        items.append(ExtractedItem(name=clean_name, count=qty, unit_value=unit_value, unit=unit))
    return ExtractionResult(provider="swiggy", items=items)

def get_extraction_function(provider: str):
    strategies = {
        "zepto": extract_from_zepto,
        "blinkit": extract_from_blinkit,
        "swiggy": extract_from_swiggy,
        "first_club": extract_from_first_club,
    }
    func = strategies.get(provider)
    if not func: raise HTTPException(status_code=400, detail=f"Unsupported provider: {provider}")
    return func

@router.post("/extract", response_model=ExtractionResult)
async def extract_receipt(file: UploadFile = File(...)):
    filename = file.filename.lower()
    is_pdf = filename.endswith('.pdf') or file.content_type == 'application/pdf'
    is_image = any(filename.endswith(ext) for ext in ['.jpg', '.jpeg', '.png', '.webp']) or (file.content_type and file.content_type.startswith('image/'))
    
    if not is_pdf and not is_image:
        raise HTTPException(status_code=400, detail="File must be a PDF or an Image (JPG, PNG, WEBP)")
    
    suffix = '.pdf' if is_pdf else os.path.splitext(filename)[1]
    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        content = await file.read()
        tmp.write(content)
        tmp_path = tmp.name
    
    try:
        if is_pdf:
            provider = detect_provider(tmp_path)
            func = get_extraction_function(provider)
            return func(tmp_path)
        else:
            items_raw = extract_items(tmp_path)
            items = [ExtractedItem(name=item["name"], count=item["count"], unit_value=item["unit_value"], unit=item["unit"]) for item in items_raw]
            return ExtractionResult(provider="image_ocr", items=items)
    except Exception as e:
        logger.exception(f"Extraction failed: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")
    finally:
        if os.path.exists(tmp_path): os.unlink(tmp_path)
