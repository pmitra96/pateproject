import os
import sys
import re

# Add parent directory to path to import app modules
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from app.database import SessionLocal
from app.models import Category

def parse_markdown(file_path):
    with open(file_path, 'r') as f:
        lines = f.readlines()

    categories = []
    current_h1 = None # Level 1
    current_h2 = None # Level 2

    for line in lines:
        line = line.strip()
        if not line:
            continue
            
        # Level 1 Category: # 1. Fresh Produce
        h1_match = re.match(r'^#\s+\d+\.\s+(.+)', line)
        if h1_match:
            current_h1 = h1_match.group(1).strip()
            current_h2 = None
            categories.append({"name": current_h1, "parent": None, "is_leaf": False})
            print(f"Found L1: {current_h1}")
            continue

        # Level 2 Category: ## 1.1 Fresh Vegetables
        h2_match = re.match(r'^##\s+[\d\.]+\s+(.+)', line)
        if h2_match:
            current_h2 = h2_match.group(1).strip()
            # Parent is current_h1
            categories.append({"name": current_h2, "parent": current_h1, "is_leaf": False})
            print(f"  Found L2: {current_h2} (Parent: {current_h1})")
            continue
            
        # Leaf Category: - Root vegetables
        leaf_match = re.match(r'^-\s+(.+)', line)
        if leaf_match:
            leaf_name = leaf_match.group(1).strip()
            # Parent is current_h2 if exists, else current_h1
            parent = current_h2 if current_h2 else current_h1
            if parent:
                categories.append({"name": leaf_name, "parent": parent, "is_leaf": True})
                # print(f"    Found Leaf: {leaf_name} (Parent: {parent})")

    return categories

def seed_database(categories):
    db = SessionLocal()
    try:
        # Cache existing categories to avoid duplicates and lookups
        existing = {c.name: c for c in db.query(Category).all()}
        
        count = 0
        for cat_data in categories:
            name = cat_data["name"]
            parent_name = cat_data["parent"]
            is_leaf = cat_data["is_leaf"]
            
            if name in existing:
                continue

            parent_id = None
            if parent_name and parent_name in existing:
                parent_id = existing[parent_name].id
            
            # Construct search query/url for leaf nodes
            search_query = None
            start_url = None
            if is_leaf:
                search_query = name
                # Standard Zepto search URL pattern
                start_url = f"https://www.zepto.com/search?query={name.replace(' ', '%20')}"

            new_cat = Category(
                name=name,
                parent_id=parent_id,
                is_leaf=is_leaf,
                search_query=search_query,
                start_url=start_url
            )
            db.add(new_cat)
            db.commit()
            db.refresh(new_cat)
            existing[name] = new_cat # Add to local cache for children
            count += 1
            
        print(f"Seeded {count} new categories.")
    except Exception as e:
        print(f"Error seeding: {e}")
        db.rollback()
    finally:
        db.close()

if __name__ == "__main__":
    md_path = os.path.join(os.path.dirname(__file__), '../../all_categories.md')
    if not os.path.exists(md_path):
        print(f"Error: File not found at {md_path}")
        sys.exit(1)
        
    print("Parsing categories...")
    parsed_cats = parse_markdown(md_path)
    print(f"Found {len(parsed_cats)} categories in markdown.")
    
    print("Seeding database...")
    seed_database(parsed_cats)
