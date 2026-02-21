from playwright.async_api import async_playwright
from bs4 import BeautifulSoup
import asyncio
import json
import logging
import re

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class InstamartScraper:
    def __init__(self):
        self.browser = None
        self.context = None

    async def start(self):
        """Start the browser efficiently."""
        self.playwright = await async_playwright().start()
        self.browser = await self.playwright.chromium.launch(
            headless=True,
            args=['--disable-gpu', '--disable-dev-shm-usage', '--no-sandbox']
        )
        self.context = await self.browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
            viewport={'width': 1920, 'height': 1080}
        )
        # Block heavy resources
        await self.context.route("**/*", lambda route: self._handle_route(route))

    async def _handle_route(self, route):
        if route.request.resource_type in ["image", "media", "font"]:
            await route.abort()
        else:
            await route.continue_()

    async def stop(self):
        if self.context:
            await self.context.close()
        if self.browser:
            await self.browser.close()
        if self.playwright:
            await self.playwright.stop()

    async def scrape_product(self, url: str):
        """Scrapes a single product from Instamart (detail view)."""
        if not self.browser:
            await self.start()
        
        page = await self.context.new_page()
        try:
            logger.info(f"Navigating to {url}")
            await page.goto(url, wait_until='domcontentloaded', timeout=45000)
            await page.wait_for_timeout(3000) # Allow React to hydrate

            # Swiggy Instamart detail might be a modal or full page.
            # We need to find the "Nutritional Information" section.
            
            content = await page.content()
            soup = BeautifulSoup(content, 'html.parser')
            
            product_data = {'url': url}
            
            # 1. Product Name (Try multiple selectors)
            # Usually h1 or a distinct class in the modal
            # Based on exploration: "Product Name: div containing the product title"
            # In detail view, it's likely an h1 or large div.
            h1 = soup.find('h1')
            if h1:
                product_data['name'] = h1.get_text().strip()
            else:
                # Fallback: finding distinct class from exploration if valid, 
                # but valid class names are dynamic (sc-...). 
                # Let's rely on metadata or known structure if possible.
                pass

            # 2. Nutrition Info
            # "Found in the detail panel under a div containing 'Nutritional Information'"
            nutrition_header = soup.find(string=re.compile("Nutritional Information", re.IGNORECASE))
            if nutrition_header:
                # The data is listed in li elements
                nutrition_container = nutrition_header.find_parent('div')
                if nutrition_container:
                    # Traverse down to find list items
                    # It might be in a sibling or child
                    # Robust approach: Find the container, then find all li's inside the parent's container
                    # or the next sibling.
                    
                    # Assuming standard structure where header is followed by content
                    parent_section = nutrition_container.parent
                    items = parent_section.find_all('li')
                    
                    nutrition_info = {}
                    for item in items:
                        text = item.get_text().strip()
                        # "Energy: 62.10 kcal"
                        if ":" in text:
                            key, val = text.split(":", 1)
                            nutrition_info[key.strip().lower()] = val.strip()
                        else:
                            # Sometimes just "Protein 3.4g"
                            parts = text.split(" ")
                            if len(parts) >= 2:
                                nutrition_info[parts[0].strip().lower()] = " ".join(parts[1:])
                    
                    product_data['nutrition_info'] = nutrition_info
            
            # 3. Image
            img = soup.find('img', alt=product_data.get('name', 'Product Image'))
            if img:
                product_data['image'] = img.get('src')
                
            # 4. Price and other details
            # If not found, we might scrape from JSON-LD if available
            scripts = soup.find_all('script', type='application/ld+json')
            for script in scripts:
                try:
                    data = json.loads(script.string)
                    if data.get('@type') == 'Product':
                        product_data['name'] = product_data.get('name') or data.get('name')
                        product_data['image'] = product_data.get('image') or data.get('image')
                        product_data['brand'] = data.get('brand', {}).get('name')
                        product_data['description'] = data.get('description')
                except:
                    pass

            return product_data

        except Exception as e:
            logger.error(f"Error scraping Instamart product {url}: {e}")
            return None
        finally:
            await page.close()

    def _extract_ids_from_json(self, data, ids_set):
        """Recursively search for 'itemId' in JSON data."""
        if isinstance(data, dict):
            for key, value in data.items():
                # Check for direct itemId or productId (as seen in network analysis)
                if key in ['itemId', 'id', 'productId'] and isinstance(value, str):
                    # Swiggy IDs are typically 10 chars, alphanumeric, uppercase.
                    # e.g. UVE5PH7SUO, 4ZHMXAN34F
                    if len(value) == 10 and value.isalnum() and value.isupper():
                        ids_set.add(value)
                
                # Recursive search
                self._extract_ids_from_json(value, ids_set)
        
        elif isinstance(data, list):
            for item in data:
                self._extract_ids_from_json(item, ids_set)

    async def crawl_category(self, search_url: str, max_products: int = 50):
        """
        Crawls a search result page by intercepting API responses using page.route.
        """
        if not self.browser:
            await self.start()
            
        page = await self.context.new_page()
        product_ids = set()
        
        # Intercept network requests
        async def handle_route(route):
            response = await route.fetch()
            
            # Check for API usage
            # https://www.swiggy.com/api/instamart/search/v2
            # Also dapi/instamart/search
            if "search" in route.request.url and "json" in response.headers.get("content-type", ""):
                 try:
                    data = await response.json()
                    self._extract_ids_from_json(data, product_ids)
                    logger.info(f"Intercepted search response, found {len(product_ids)} total IDs so far.")
                 except:
                    pass
            
            # Fulfill the request so the page continues working
            await route.fulfill(response=response)

        # Route all requests to our handler
        # We need to be careful not to block resources that self._handle_route does
        # So we should combine them or just route API calls specifically.
        # Let's route everything API-like.
        await page.route("**/api/instamart/search/**", handle_route)
        await page.route("**/dapi/instamart/search/**", handle_route)
        
        try:
            logger.info(f"Crawling search: {search_url}")
            await page.goto(search_url, wait_until='domcontentloaded', timeout=45000)
            await page.wait_for_timeout(5000) # Wait for initial API calls
            
            last_height = await page.evaluate("document.body.scrollHeight")
            
            # Scroll loop
            no_change_count = 0
            while len(product_ids) < max_products:
                
                # Scroll
                await page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                await page.wait_for_timeout(4000)
                
                new_height = await page.evaluate("document.body.scrollHeight")
                if new_height == last_height:
                    no_change_count += 1
                    if no_change_count >= 2: # Stop after 2 retries of no scroll
                        break
                else:
                    no_change_count = 0
                    
                last_height = new_height
                
            logger.info(f"Final count: {len(product_ids)} products.")
            
            # Construct URLs
            product_urls = [f"https://www.swiggy.com/instamart/item/{pid}" for pid in product_ids]
            return product_urls[:max_products]

        except Exception as e:
            logger.error(f"Error crawling Instamart search {search_url}: {e}")
            return []
        finally:
            await page.close()

if __name__ == "__main__":
    async def main():
        scraper = InstamartScraper()
        # Test search
        # https://www.swiggy.com/instamart/search?query=curd
        urls = await scraper.crawl_category("https://www.swiggy.com/instamart/search?query=curd", max_products=5)
        print(f"URLs found: {urls}")
        
        if urls:
            data = await scraper.scrape_product(urls[0])
            print(json.dumps(data, indent=2))
            
        await scraper.stop()
    
    asyncio.run(main())
