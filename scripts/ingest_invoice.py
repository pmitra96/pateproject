import sys
import requests
import json
import time

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 ingest_invoice.py <path_to_pdf> [user_id]")
        sys.exit(1)

    pdf_path = sys.argv[1]
    user_id = sys.argv[2] if len(sys.argv) > 2 else "1"
    api_base = "http://localhost:8080"
    
    # 1. Extract
    print(f"🔍 Extracting items from {pdf_path}...")
    with open(pdf_path, 'rb') as f:
        files = {'invoice': f}
        headers = {'Authorization': f'Bearer {user_id}'}
        resp = requests.post(f"{api_base}/items/extract", files=files, headers=headers)
    
    if resp.status_code != 200:
        print(f"❌ Extraction failed: {resp.text}")
        return

    data = resp.json()
    
    # Handle both old (array) and new (object) formats for robustness
    if isinstance(data, dict):
        provider = data.get('provider', 'zepto')
        items = data.get('items', [])
    else:
        provider = 'zepto'  # Fallback for old format
        items = data

    print(f"✅ Extracted {len(items)} items. Detected provider: {provider}")

    # 2. Transform & Ingest
    print(f"📥 Ingesting into pantry for user {user_id}...")
    order_data = {
        "user_id": user_id,
        "external_order_id": f"ORD_{int(time.time())}",
        "provider": provider,
        "order_date": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "items": [
            {
                "raw_name": item['name'],
                "quantity": item['count'] * item['unit_value'],
                "unit": item['unit']
            } for item in items
        ]
    }

    headers = {
        'Content-Type': 'application/json',
        'X-API-Key': 'secret-key'
    }
    
    resp = requests.post(f"{api_base}/ingest/order", json=order_data, headers=headers)
    
    if resp.status_code in [200, 201]:
        print("🎉 Ingestion successful!")
        print(json.dumps(resp.json(), indent=2))
    else:
        print(f"❌ Ingestion failed: {resp.text}")

if __name__ == "__main__":
    main()
