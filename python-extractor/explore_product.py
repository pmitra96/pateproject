from app.scraper import ZeptoScraper
import asyncio

async def explore_product(url):
    scraper = ZeptoScraper()
    await scraper.start()
    page = await scraper.context.new_page()
    
    print(f"Navigating to {url}...")
    await page.goto(url, wait_until='domcontentloaded')
    await page.wait_for_timeout(5000)
    
    # Extract all text nodes
    texts = await page.evaluate("""() => {
        const results = [];
        const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT, null, false);
        let node;
        while(node = walk.nextNode()) {
            const t = node.textContent.trim();
            if (t.length > 5) results.push(t);
        }
        return results;
    }""")
    
    print(f"Found {len(texts)} text nodes.")
    for t in texts:
        if '100' in t or 'serving' in t.lower() or 'basis' in t.lower() or 'approx' in t.lower() or 'value' in t.lower():
            print(f"> {t}")
            
    await scraper.stop()

if __name__ == "__main__":
    from app.database import SessionLocal
    from app.models import Product
    db = SessionLocal()
    p = db.query(Product).filter(Product.name.ilike('%Orika Herby%')).first()
    url = p.url if p else None
    db.close()
    
    if url:
        asyncio.run(explore_product(url))
    else:
        print("Product URL not found in DB")
