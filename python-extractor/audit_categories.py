from app.scraper import ZeptoScraper
import asyncio
from app.database import SessionLocal
from app.models import Category
from fuzzywuzzy import fuzz

async def audit_categories():
    # 1. Fetch our DB categories
    db = SessionLocal()
    db_cats = {c.name.lower() for c in db.query(Category).all()}
    db.close()
    
    print(f"Loaded {len(db_cats)} categories from DB.")

    # 2. Scrape live categories
    scraper = ZeptoScraper()
    await scraper.start()
    page = await scraper.context.new_page()
    
    print("Navigating to Zepto Home to find categories...")
    # Zepto usually has a 'Categories' sidebar or menu.
    # Let's try to access the sidebar directly if possible, or just scrape 'a' tags that look like categories
    # URL structure for category is usually /cn/[category-slug]/[cid]
    
    await page.goto("https://www.zepto.com/", wait_until='domcontentloaded')
    await page.wait_for_timeout(5000)
    
    # Extract all links that contain "/cn/" (Category Name) or "/cid/" (Category ID)
    # This is a good proxy for "Is this a category?"
    live_categories = await page.evaluate("""() => {
        const anchors = Array.from(document.querySelectorAll('a'));
        return anchors
            .filter(a => a.href.includes('/cn/') || a.href.includes('/cid/'))
            .map(a => a.textContent.trim())
            .filter(t => t.length > 0)
    }""")
    
    live_categories = set(live_categories)
    print(f"Found {len(live_categories)} potential categories on Homepage.")
    
    # 3. Compare
    missing = []
    for live_cat in live_categories:
        # Check for exact match
        if live_cat.lower() in db_cats:
            continue
            
        # Check for fuzzy match (e.g., "Fresh Fruits" vs "Fruits")
        # If > 80 match, assume covered
        is_covered = False
        for db_cat in db_cats:
            if fuzz.ratio(live_cat.lower(), db_cat) > 80:
                is_covered = True
                break
        
        if not is_covered:
            missing.append(live_cat)
            
    print("\nPotentially Missing Categories:")
    for m in sorted(missing):
        print(f"- {m}")
        
    await scraper.stop()

if __name__ == "__main__":
    asyncio.run(audit_categories())
