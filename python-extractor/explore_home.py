from app.scraper import ZeptoScraper
import asyncio

async def explore():
    scraper = ZeptoScraper()
    await scraper.start()
    page = await scraper.context.new_page()
    
    print("Navigating to Zepto Home...")
    await page.goto("https://www.zepto.com/", wait_until='domcontentloaded')
    await page.wait_for_timeout(5000) # Wait for hydration
    
    # Dump HTML
    content = await page.content()
    with open("zepto_home.html", "w") as f:
        f.write(content)
        
    # Try to find common menu patterns
    links = await page.evaluate("""() => {
        return Array.from(document.querySelectorAll('a')).map(a => ({href: a.href, text: a.textContent.trim()}))
    }""")
    
    print(f"Found {len(links)} links.")
    for l in links[:20]:
        print(l)
        
    await scraper.stop()

if __name__ == "__main__":
    asyncio.run(explore())
