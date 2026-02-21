import sqlite3
import json
import gzip
import os
from datetime import datetime

def export_data():
    db_path = 'zepto_scraper.db'
    export_dir = '../backend/seeds'
    export_filename = 'scraped_data_seed.json.gz'
    export_path = os.path.join(export_dir, export_filename)

    if not os.path.exists(export_dir):
        os.makedirs(export_dir)

    print(f"Connecting to {db_path}...")
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    cursor = conn.cursor()

    # Export Brands
    print("Fetching brands...")
    cursor.execute("SELECT * FROM brands")
    brands = [dict(row) for row in cursor.fetchall()]

    # Export Products
    print("Fetching products...")
    cursor.execute("SELECT * FROM products")
    products = []
    for row in cursor.fetchall():
        product = dict(row)
        # Handle JSON fields if they are strings in SQLite
        if isinstance(product.get('tags'), str):
            try:
                product['tags'] = json.loads(product['tags'])
            except:
                pass
        if isinstance(product.get('nutrition_info'), str):
            try:
                product['nutrition_info'] = json.loads(product['nutrition_info'])
            except:
                pass
        products.append(product)

    data = {
        "exported_at": datetime.utcnow().isoformat(),
        "brands": brands,
        "products": products
    }

    print(f"Exporting {len(brands)} brands and {len(products)} products to {export_path}...")
    
    with gzip.open(export_path, 'wt', encoding='utf-8') as f:
        json.dump(data, f, indent=2)

    print("Export completed successfully.")
    conn.close()

if __name__ == "__main__":
    export_data()
