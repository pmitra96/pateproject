"""
PDF Extraction Microservice (Refactored as Router)

FastAPI router that extracts structured data from grocery receipt PDFs
using pdfplumber. Supports multiple providers (Zepto, Blinkit, etc.).
"""

import re
import logging
import sys
import tempfile
import os
from typing import List, Optional
from pathlib import Path
from fastapi import APIRouter, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
import pdfplumber
from pydantic import BaseModel

# Try to import image_extractor from the root
try:
    from image_extractor import extract_items
except ImportError:
    # If starting from 'app', we might need to look one level up
    sys.path.append(os.path.dirname(os.path.dirname(os.path.dirname(__file__))))
    from image_extractor import extract_items

# Configure logging to use the same file as the main service
# Note: we use the same logger setup as app.py had
LOG_FILE = Path(__file__).parent.parent.parent / "python-extractor.log"
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(LOG_FILE),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

router = APIRouter()

BLINKIT_EXCLUDED_ITEMS = ['handling charge', 'total']

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
    
    name = re.sub(r'\d+\.?\d*\s*()', '', name, flags=re.IGNORECASE)

    # Remove pack pouch jar
    name = re.sub(r"\s*(?:\([^)]*\)\s*)+$", "", name, flags=re.IGNORECASE).strip()

    # Remove extra whitespace and newlines
    name = re.sub(r'\s+', ' ', name)
    name = name.strip()
    
    return name


def detect_provider(pdf_path: str) -> str:
    """Detect the provider from PDF file."""
    logger.info(f"Detecting provider from PDF: {pdf_path}")
    with pdfplumber.open(pdf_path) as pdf:
        # Check first few pages for provider info
        for page in pdf.pages[:3]:  # Check first 3 pages
            text = page.extract_text()
            if text:
                text_lower = text.lower()
                
                if "zepto" in text_lower or "geddit convenience" in text_lower:
                    logger.info("Detected provider: zepto")
                    return "zepto"
                elif "blinkit" in text_lower or "grofers" in text_lower:
                    logger.info("Detected provider: blinkit")
                    return "blinkit"
                elif "trolleypop" in text_lower or "first club" in text_lower:
                    logger.info("Detected provider: first_club")
                    return "first_club"
                else:
                    return "swiggy"
    
    logger.warning("Could not detect provider, returning 'unknown'")
    return "unknown"

def extract_from_zepto(pdf_path: str) -> ExtractionResult:
    """Extract items from Zepto PDF."""
    logger.info("Starting Zepto extraction")
    items = []
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            
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
    
    logger.info(f"Zepto extraction completed. Found {len(items)} items")
    return ExtractionResult(provider="zepto", items=items)


def extract_from_first_club(pdf_path: str) -> ExtractionResult:
    """Extract items from First Club PDF Image."""
    logger.info("Starting First Club extraction")
    items = []
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
            # Extract tables
            tables = page.extract_tables()
            
            for table in tables:
                if not table or len(table) < 2:
                    continue
                
                # Find header row and column indices
                header_idx = None
                particulars_col_idx = None
                
                for i, row in enumerate(table):
                    if not row:
                        continue
                    
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in row)
                    
                    # Look for header with "Particulars"
                    if 'particulars' in row_text:
                        header_idx = i
                        
                        # Find column index for Particulars
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'particulars' in cell_lower:
                                    particulars_col_idx = j
                        break
                
                if header_idx is None or particulars_col_idx is None:
                    logger.warning("Could not find header row or Particulars column")
                    continue
                
                # Skip the sub-header row (Rate/Amt. row) if it exists
                start_row = header_idx + 1
                if start_row < len(table) and table[start_row]:
                    row_text = ' '.join(str(cell).lower() if cell else '' for cell in table[start_row])
                    if 'rate' in row_text and 'amt' in row_text:
                        start_row += 1
                
                # Extract items
                for row in table[start_row:]:
                    if not row or not any(row):
                        continue
                    
                    # Skip total rows
                    row_text = ' '.join(str(cell) if cell else '' for cell in row).lower()
                    if 'total' in row_text or 'item total' in row_text or 'delivery charges' in row_text:
                        break
                    
                    # Get particulars (item description)
                    particulars = str(row[particulars_col_idx]) if particulars_col_idx < len(row) and row[particulars_col_idx] else None
                    if not particulars or len(particulars.strip()) < 2:
                        continue
                    
                    # Skip if it's just a number (Sr. No column content)
                    if particulars.strip().isdigit():
                        continue
                    
                    # Extract quantity from "x N" pattern
                    qty = 1.0
                    qty_match = re.search(r'\s+x\s+(\d+(?:\.\d+)?)\s*$', particulars, re.IGNORECASE)
                    if qty_match:
                        qty = float(qty_match.group(1))
                        # Remove the "x N" part from the description
                        particulars = re.sub(r'\s+x\s+\d+(?:\.\d+)?\s*$', '', particulars, flags=re.IGNORECASE)
                    
                    # Parse unit info from description (e.g., "(200g)")
                    unit_value, unit = parse_unit_and_value(particulars)
                    
                    # Clean name
                    clean_name = clean_item_name(particulars, unit_value, unit)
                    
                    if clean_name and len(clean_name.strip()) > 1:
                        items.append(ExtractedItem(
                            name=clean_name,
                            count=qty,
                            unit_value=unit_value,
                            unit=unit
                        ))
                        logger.debug(f"Extracted item: {clean_name} (qty: {qty}, unit: {unit_value}{unit})")
    
    logger.info(f"First Club extraction completed. Found {len(items)} items")
    return ExtractionResult(provider="first_club", items=items)


def extract_from_blinkit(pdf_path: str) -> ExtractionResult:
    """Extract items from Blinkit PDF."""
    logger.info("Starting Blinkit extraction")
    items = []
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
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
                    if 'description' in row_text or 'item' in row_text or 'product' in row_text:
                        header_idx = i
                        
                        # Find column indices
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower or 'product' in cell_lower:
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
                    
                    if clean_name.lower() in BLINKIT_EXCLUDED_ITEMS:
                        continue
                    
                    if clean_name:
                        items.append(ExtractedItem(
                            name=clean_name,
                            count=qty,
                            unit_value=unit_value,
                            unit=unit
                        ))
    
    logger.info(f"Blinkit extraction completed. Found {len(items)} items")
    return ExtractionResult(provider="blinkit", items=items)


def extract_from_swiggy(pdf_path: str) -> ExtractionResult:
    """Extract items from Swiggy Instamart PDF."""
    logger.info("Starting Swiggy extraction")
    items = []
    
    with pdfplumber.open(pdf_path) as pdf:
        for page in pdf.pages:
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
                    if 'description' in row_text or 'item' in row_text or 'product' in row_text:
                        header_idx = i
                        
                        # Find column indices
                        for j, cell in enumerate(row):
                            if cell:
                                cell_lower = str(cell).lower()
                                if 'description' in cell_lower or 'item' in cell_lower or 'product' in cell_lower:
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
    
    logger.info(f"Swiggy extraction completed. Found {len(items)} items")
    return ExtractionResult(provider="swiggy", items=items)


def get_extraction_function(provider: str):
    """
    Strategy pattern: Return the appropriate extraction function based on provider.
    """
    extraction_strategies = {
        "zepto": extract_from_zepto,
        "blinkit": extract_from_blinkit,
        "swiggy": extract_from_swiggy,
        "first_club": extract_from_first_club,
    }
    
    extraction_func = extraction_strategies.get(provider)
    
    if extraction_func is None:
        logger.error(f"Unsupported provider: {provider}")
        raise HTTPException(
            status_code=400, 
            detail=f"Unsupported provider: {provider}. Supported providers: {', '.join(extraction_strategies.keys())}"
        )
    
    logger.info(f"Selected extraction strategy for provider: {provider}")
    return extraction_func


@router.post("/extract", response_model=ExtractionResult)
async def extract_receipt(file: UploadFile = File(...)):
    """
    Extract items from a PDF or Image receipt.
    """
    filename = file.filename.lower()
    logger.info(f"Received extraction request for file: {filename}")
    
    is_pdf = filename.endswith('.pdf') or file.content_type == 'application/pdf'
    is_image = any(filename.endswith(ext) for ext in ['.jpg', '.jpeg', '.png', '.webp']) or \
               (file.content_type and file.content_type.startswith('image/'))
    
    if not is_pdf and not is_image:
        logger.warning(f"Rejected unsupported file type: {filename} (MIME: {file.content_type})")
        raise HTTPException(status_code=400, detail="File must be a PDF or an Image (JPG, PNG, WEBP)")
    
    suffix = '.pdf' if is_pdf else os.path.splitext(filename)[1]
    
    # Save to temp file
    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        content = await file.read()
        tmp.write(content)
        tmp_path = tmp.name
    
    logger.info(f"Saved uploaded file to temporary location: {tmp_path}")
    
    try:
        if is_pdf:
            # Step 1: Detect provider from PDF
            provider = detect_provider(tmp_path)
            
            # Step 2: Get the appropriate extraction function using strategy pattern
            extraction_func = get_extraction_function(provider)
            
            # Step 3: Call the extraction function
            result = extraction_func(tmp_path)
            
            logger.info(f"Successfully extracted {len(result.items)} items from PDF: {filename}")
            return result
        else:
            # Image extraction path
            logger.info(f"Processing image extraction for: {filename}")
            items_raw = extract_items(tmp_path)
            
            # Convert raw dicts to ExtractedItem models
            items = [
                ExtractedItem(
                    name=item["name"],
                    count=item["count"],
                    unit_value=item["unit_value"],
                    unit=item["unit"]
                ) for item in items_raw
            ]
            
            logger.info(f"Successfully extracted {len(items)} items from image: {filename}")
            return ExtractionResult(provider="image_ocr", items=items)
            
    except HTTPException as he:
        logger.error(f"HTTP error during extraction: {he.detail}")
        raise
    except Exception as e:
        logger.exception(f"Unexpected error during extraction: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Extraction failed: {str(e)}")
    finally:
        # Clean up temp file
        if os.path.exists(tmp_path):
            os.unlink(tmp_path)
            logger.debug(f"Cleaned up temporary file: {tmp_path}")
