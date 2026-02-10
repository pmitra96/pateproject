import cv2
import pytesseract
import numpy as np
import re
import argparse
import os
import sys


# ---------- NON-FOOD ITEM FILTER ----------
NON_FOOD_KEYWORDS = [
    "wooden", "plate", "spoon", "fork", "knife", "bowl", "container", "box",
    "bag", "napkin", "tissue", "straw", "cup", "glass", "mug", "utensil",
    "cutlery", "tray", "basket", "wrapper", "packaging", "lid", "cover",
    "stand", "holder", "rack", "mat", "coaster", "bottle opener", "corkscrew",
    "toothpick", "chopstick", "ladle", "spatula", "tong", "peeler", "grater",
    "sieve", "strainer", "funnel", "measuring", "timer", "thermometer"
]

def is_non_food_item(name: str) -> bool:
    """Check if an item name matches non-food keywords."""
    name_lower = name.lower()
    for keyword in NON_FOOD_KEYWORDS:
        if keyword in name_lower:
            return True
    return False


# ---------- NAME CLEANING ----------
def clean_name(name: str) -> str:
    return re.sub(r"\s*(?:\([^)]*\)\s*)+$", "", name).strip()


# ---------- QUANTITY PARSER ----------
def parse_quantity(qty: str):
    if not qty:
        return 1.0, "pcs", 1.0

    # OCR fix: treat '|', 'I', 'l' as '1' in quantity contexts
    qty = qty.lower().replace('|', '1').replace('I', '1').replace('l', '1').replace('o', '0')
    qty = qty.replace(' (', '(').replace('pieces', 'pcs').replace('piece', 'pc')
    
    # Handle 's' misread (could be 5 or 3)
    # Common Blinkit misread: 'S00g' for '500g'
    if 's00' in qty:
        qty = qty.replace('s00', '500')
    qty = qty.replace('s', '3') # fallback for other 's' misreads

    # count after x: "Piece x 1", "200gx1"
    count_match = re.search(r"x\s*(\d+)", qty)
    count = float(count_match.group(1)) if count_match else 1.0

    # weight RANGE (300–400 g)
    qty = qty.replace('q', 'g')
    range_match = re.search(r"(\d+)\s*[-–\s]+\s*(\d+)\s*g", qty)
    if range_match:
        return (float(range_match.group(1)) + float(range_match.group(2))) / 2, "g", count

    # single weight g: handles "200 9g" -> 200
    g_match = re.search(r"(\d+)[\s\d]*g", qty)
    if g_match:
        val = g_match.group(1)
        # If the number is suspiciously small (like '9' in '200 9g'), try to find the bigger one
        nums = re.findall(r"\d+", qty[:qty.find('g')])
        if nums:
            val = max([int(n) for n in nums])
        return float(val), "g", count

    # kg → g
    kg_match = re.search(r"(\d+(?:\.\d+)?)\s*kg", qty)
    if kg_match:
        return float(kg_match.group(1)) * 1000, "g", count

    # pcs
    pcs_match = re.search(r"(\d+)\s*(pcs|pc|item|pieces|piece)", qty)
    if pcs_match:
        return float(pcs_match.group(1)), "pcs", count

    return 1.0, "pcs", count


# ---------- IMAGE PREPROCESSING ----------
def preprocess_for_ocr(img):
    """Clean up the image: remove lines and small noise (icons)."""
    h, w = img.shape[:2]
    
    # 1. Convert to gray and threshold
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    
    # Increase contrast to make text pop
    gray = cv2.convertScaleAbs(gray, alpha=1.5, beta=0)
    
    thresh = cv2.threshold(gray, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)[1]

    # 2. Remove horizontal lines (separator lines in UI)
    horizontal_kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (int(w * 0.1), 1))
    remove_horizontal = cv2.morphologyEx(thresh, cv2.MORPH_OPEN, horizontal_kernel, iterations=2)
    cnts = cv2.findContours(remove_horizontal, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    cnts = cnts[0] if len(cnts) == 2 else cnts[1]
    for c in cnts:
        cv2.drawContours(thresh, [c], -1, (0,0,0), -1)

    # 3. Remove small noise / icons
    cnts = cv2.findContours(thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    cnts = cnts[0] if len(cnts) == 2 else cnts[1]
    for c in cnts:
        area = cv2.contourArea(c)
        x,y,cw,ch = cv2.boundingRect(c)
        # Small blobs or fragments that aren't tall enough to be text
        if area < 15 or ch < 8:
            cv2.drawContours(thresh, [c], -1, (0,0,0), -1)

    # 4. Invert back to black text on white background
    cleaned = cv2.bitwise_not(thresh)
    return cleaned


# ---------- OCR ONE ROW ----------
# ---------- OCR ONE ROW ----------
def ocr_row(img, psm=6):
    # If it's a color image, preprocess it
    if len(img.shape) == 3:
        img = preprocess_for_ocr(img)
    
    # Resize for better OCR accuracy
    img_resized = cv2.resize(img, None, fx=2.5, fy=2.5, interpolation=cv2.INTER_CUBIC)
    
    config = f"--oem 3 --psm {psm} -l eng"
    text = pytesseract.image_to_string(img_resized, config=config).strip()
    
    return text





# ---------- MAIN ----------
def load_image(image_path: str):


    """Robustly load an image to handle path issues and permissions."""
    try:
        # Method 1: standard imread
        img = cv2.imread(image_path)
        if img is not None:
            return img
            
        # Method 2: read via numpy (more robust for some paths)
        with open(image_path, 'rb') as f:
            chunk = np.frombuffer(f.read(), dtype=np.uint8)
            img = cv2.imdecode(chunk, cv2.IMREAD_COLOR)
            return img
    except PermissionError:
        print(f"\n[!] PERMISSION ERROR: macOS is preventing access to {image_path}", file=sys.stderr)
        print("    Try moving the file to the project folder or granting Full Disk Access to Terminal.", file=sys.stderr)
        raise
    except Exception as e:
        print(f"\n[!] ERROR loading image: {e}", file=sys.stderr)
        return None

def extract_items(image_path, debug=False):
    img = load_image(image_path)
    if img is None:
        return []
        
    h, w = img.shape[:2]
    if debug:
        print(f"Debug: Image dimensions {w}x{h}")

    # Crop item list region (tuned for mobile screenshots)
    # y1 is usually around 15-20% down, y2 around 85-90%
    y1, y2 = int(h * 0.15), int(h * 0.90)
    items_region = img[y1:y2, :]
    
    if debug:
        cv2.imwrite("debug_crop.jpg", items_region)
        print("Debug: Saved debug_crop.jpg")

    gray = cv2.cvtColor(items_region, cv2.COLOR_BGR2GRAY)
    thresh = cv2.adaptiveThreshold(
        gray, 255,
        cv2.ADAPTIVE_THRESH_GAUSSIAN_C,
        cv2.THRESH_BINARY_INV,
        31, 2
    )
    
    if debug:
        cv2.imwrite("debug_thresh.jpg", thresh)
        print("Debug: Saved debug_thresh.jpg")

    contours, _ = cv2.findContours(
        thresh, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE
    )

    rows = []
    for c in contours:
        x, y, cw, ch = cv2.boundingRect(c)
        # Relaxed height/width requirements even further
        if ch > 40 and cw > w * 0.3:
            rows.append((y, y + ch))

    if debug:
        print(f"Debug: Found {len(rows)} potential rows")

    # NEW APPROACH: Always try full region OCR as well, as it can be more robust
    # for certain layouts where row detection fails.
    if debug:
        print("Debug: Performing full region OCR (PSM 11)...")
        
    # Pre-process for full region
    full_text = ocr_row(items_region, psm=11) # Sparse text
    if not full_text or len(full_text) < 20:
        if debug: print("Debug: PSM 11 failed, trying PSM 3...")
        full_text = ocr_row(items_region, psm=3)

    if debug:
        with open("debug_ocr.txt", "w") as f:
            f.write(full_text)
        print("Debug: Saved debug_ocr.txt")

    items = parse_raw_text(full_text)
    
    # If we found nothing with full text, and we had rows, try rows as fallback
    if not items and rows:
        if debug: print("Debug: Full OCR failed, trying individual rows...")
        rows = sorted(rows)
        # Merge overlapping or very close rows
        merged_rows = []
        if rows:
            curr_y1, curr_y2 = rows[0]
            for next_y1, next_y2 in rows[1:]:
                if next_y1 < curr_y2 + 20: # 20px buffer
                    curr_y2 = max(curr_y2, next_y2)
                else:
                    merged_rows.append((curr_y1, curr_y2))
                    curr_y1, curr_y2 = next_y1, next_y2
            merged_rows.append((curr_y1, curr_y2))

        for y1, y2 in merged_rows:
            row_img = items_region[y1:y2, :]
            text = ocr_row(row_img)
            row_items = parse_raw_text(text)
            items.extend(row_items)

    return items


# ---------- NOISE CLEANING ----------
def clean_ocr_noise(text: str) -> str:
    """Aggressively remove OCR artifacts from item names."""
    # 1. Recursive leading junk and prefix stripping
    prev_text = ""
    noise_prefixes = [
        "foe", "of", "is", "is-", "r=", "gx1", "gx", "the", "at", 
        "sinn", "petted", "oad", "way", "sey", "wd", "wz", "unit", 
        "tet", "laa", "pe", "pont", "wi", "gt", "tia"
    ]
    
    while text != prev_text:
        prev_text = text
        # Remove leading symbols/numbers
        text = re.sub(r'^[—_ \.&\+–\-\=\*“"\'(>»\^~`\|\\©®™\d\?~]+', '', text).strip()
        # Remove known noise words at start
        for prefix in noise_prefixes:
            text = re.sub(rf'^{prefix}(\s+|$)', '', text, flags=re.IGNORECASE).strip()
    
    # 2. Explicitly remove stubborn fragments anywhere
    noise_fragments = [
        "Hil", "SS", "0288", "Sil", "SINN", "oad", "way", "sey", 
        "wd", "wz", "lk pt", "carat", "rd", "unit", "TET", "laa", 
        "PE", "PONT", "Wi", "gt", "Tia"
    ]
    for noise in noise_fragments:
        text = re.sub(rf'\b{noise}\b', '', text, flags=re.IGNORECASE).strip()

    # 3. Final junk strip
    text = re.sub(r'[—_ \.&\+–\-\=\*“"\'(>»\^~`\|\\©®™\d]+$', '', text).strip()
    
    return text


def is_mostly_garbage(text: str):
    """Check if a string is likely OCR noise or a pure price line."""
    text_clean = re.sub(r'[^A-Za-z0-9]', '', text)
    if not text_clean:
        return True
    
    # Pure numeric lines are likely prices or IDs, not item names
    if text_clean.isdigit():
        return True
        
    # If a word is just numbers and maybe one or two letters (e.g. SINN 295)
    letters = len(re.findall(r'[A-Za-z]', text))
    if letters < 2: # Very few letters
        return True
        
    # Density check - loosened slightly for multi-line branding
    if letters < len(text) * 0.18:
        return True
        
    return False


def parse_raw_text(text: str):
    """Helper to parse OCR text into item objects with high precision."""
    items = []
    lines = [l.strip() for l in text.splitlines() if l.strip()]
    
    # regex for quantity line "200gx1", "1 Piece x 1", "Piece x |" etc.
    # Added allowance for misread 's', 'S', 'l', '|' in numeric contexts
    qty_anchor_pattern = re.compile(r'([\d\|lsS\?]+(?:\.\d+)?\s*(?:g|kg|pcs|pc|item|items)|x\s*[\d\|l]|[\d\|l]\s*x|Piece\s*x\s*[\d\|l])', re.IGNORECASE)


    i = 0
    while i < len(lines):
        line = lines[i]
        
        # 1. Skip obvious garbage/headers/numeric prices
        if (len(line) < 2 or 
            any(skip in line.lower() for skip in [
                "item details", "delivered", "ordered on", "billing", 
                "total", "image", "delivered to", "payment", "items count",
                "savings", "discount", "summary", "delivered at", "delivered on",
                "items in this order", "how were your ordered", "bill details", "mrp"
            ]) or
            is_mostly_garbage(line)):
            i += 1
            continue

        # 2. Potential item name starts here
        name_parts = []
        name_candidate = clean_ocr_noise(line)
        if len(name_candidate) >= 3:
            name_parts.append(name_candidate)
        else:
            i += 1
            continue
        
        found_qty = False
        j = i + 1
        
        # 3. Look ahead for quantity
        while j < min(i + 15, len(lines)):
            next_line = lines[j]
            
            # If we find a quantity, we satisfy the item
            if qty_anchor_pattern.search(next_line):
                unit_value, unit, count = parse_quantity(next_line)
                full_name = " ".join(name_parts)
                full_name = clean_ocr_noise(full_name)

                if len(full_name) > 3:
                    items.append({
                        "name": full_name,
                        "count": count,
                        "unit_value": unit_value,
                        "unit": unit
                    })
                    found_qty = True
                    i = j + 1
                break

            is_noise = is_mostly_garbage(next_line)
            
            # Content line? Add to name parts
            cleaned_next = clean_ocr_noise(next_line)
            if cleaned_next and len(cleaned_next) >= 3 and not is_noise:
                # Be more persistent if we're still building the name
                if len(name_parts) >= 4 and cleaned_next[0].isupper() and any(is_mostly_garbage(l) for l in lines[i+1:j]):
                    break
                name_parts.append(cleaned_next)
            
            j += 1


        
        if not found_qty:
            # Fallback for name-only lines
            full_name = " ".join(name_parts)
            full_name = clean_ocr_noise(full_name)
            if len(full_name) > 20 and re.search(r'[A-Za-z]{10,}', full_name):
                items.append({
                    "name": full_name,
                    "count": 1.0,
                    "unit_value": 1.0,
                    "unit": "pcs"
                })
            i = j
        
    # Filter out non-food items
    items = [item for item in items if not is_non_food_item(item["name"])]
    
    return items


# ---------- RUN ----------
if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Extract items from receipt image")
    parser.add_argument("image_path", help="Path to the image file (supports ~ for home directory)")
    parser.add_argument("--debug", action="store_true", help="Save debug images")
    args = parser.parse_args()
    
    # Expand ~ to full home directory path
    image_path = os.path.expanduser(args.image_path)
    
    # Check if file exists cleanly
    if not os.path.exists(image_path):
        print(f"Error: File not found: {image_path}", file=sys.stderr)
        sys.exit(1)
    
    # Detailed diagnosis if needed
    if not os.access(image_path, os.R_OK):
        print(f"\n[!] ACCESS DENIED: File exists but is not readable: {image_path}", file=sys.stderr)
        print("    This is likely due to macOS Privacy/Security settings.", file=sys.stderr)
        print("    FIX: Please copy the file to the current directory:", file=sys.stderr)
        print(f"    cp {image_path} ./", file=sys.stderr)
        sys.exit(1)

    print(f"Processing image: {image_path}")
    
    try:
        items = extract_items(image_path, debug=args.debug)
        
        if not items:
            print("\n[?] No items detected. This could be due to:")
            print("    1. The image resolution is too low")
            print("    2. The UI layout for 'Firstclub' has changed")
            print("    3. Tesseract OCR is not installed or configured correctly")
            print("\n    TIP: Try running with --debug to see intermediate images.")
        else:
            print(f"\nFound {len(items)} items:")
            for i, item in enumerate(items, 1):
                print(f"{i}. {item}")
    except PermissionError:
        # Handled inside load_image, but just in case
        sys.exit(1)
    except (pytesseract.TesseractNotFoundError, Exception) as e:
        if "tesseract is not installed" in str(e).lower() or isinstance(e, pytesseract.TesseractNotFoundError):
            print(f"\n[!] MISSING DEPENDENCY: Tesseract OCR engine not found.", file=sys.stderr)
            print("    The 'pytesseract' library requires the Tesseract binary to be installed on your system.", file=sys.stderr)
            print("\n    FIX (macOS): Run this command in your terminal:", file=sys.stderr)
            print("    brew install tesseract", file=sys.stderr)
        else:
            import traceback
            print(f"An unexpected error occurred: {e}", file=sys.stderr)
            traceback.print_exc()
        sys.exit(1)




