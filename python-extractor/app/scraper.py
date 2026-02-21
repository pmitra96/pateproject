from playwright.async_api import async_playwright
from bs4 import BeautifulSoup
import asyncio
import json
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class ZeptoScraper:
    def __init__(self):
        self.browser = None
        self.context = None

    async def start(self):
        """Start the browser efficiently."""
        self.playwright = await async_playwright().start()
        # Launch browser with arguments to disable some features for speed
        self.browser = await self.playwright.chromium.launch(
            headless=True,
            args=[
                '--disable-gpu',
                '--disable-dev-shm-usage',
                '--no-sandbox'
            ]
        )
        # Create a persistent context to reuse session/connection details if needed
        # But for new pages, a fresh context is cleaner to avoid cookie buildup? 
        # Actually reusing context is faster.
        self.context = await self.browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
            viewport={'width': 1280, 'height': 800},
            extra_http_headers={
                "Accept-Language": "en-US,en;q=0.9",
                "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
                "sec-ch-ua": '"Not A(Brand";v="99", "Google Chrome";v="121", "Chromium";v="121"',
                "sec-ch-ua-mobile": "?0",
                "sec-ch-ua-platform": '"macOS"',
            }
        )
        
        # Block heavy resources for speed
        await self.context.route("**/*", lambda route: self._handle_route(route))

    async def _handle_route(self, route):
        """Intercept and block unnecessary resources."""
        # Allow stylesheets as SPAs often need them for layout/hydration
        if route.request.resource_type in ["image", "media", "font"]:
            await route.abort()
        else:
            await route.continue_()

    async def stop(self):
        """Stop the browser."""
        if self.context:
            await self.context.close()
        if self.browser:
            await self.browser.close()
        if self.playwright:
            await self.playwright.stop()

    async def scrape_product(self, url: str):
        """Scrapes a single product page via Playwright."""
        if not self.browser:
            await self.start()
        
        page = await self.context.new_page()
        try:
            logger.info(f"Navigating to {url}")
            # Wait for DOMContentLoaded instead of NetworkIdle (faster)
            # Since we block images, networkidle might be fine, but domcontentloaded is safer + selector wait.
            await page.goto(url, wait_until='domcontentloaded', timeout=30000)
            
            # Wait for a key element to ensure hydration (e.g. h1 product title or price)
            try:
                await page.wait_for_selector('h1', timeout=10000)
            except:
                logger.warning("Timeout waiting for h1, dumping HTML for debug")
            
            # Additional small wait for dynamic content (React hydration)
            await page.wait_for_timeout(2000) 
            
            content = await page.content()
            with open("debug_zepto.html", "w") as f:
                f.write(content)

            soup = BeautifulSoup(content, 'html.parser')
            
            product_data = {}
            
            # Try to find JSON-LD or similar structured data first
            scripts = soup.find_all('script', type='application/ld+json')
            for script in scripts:
                try:
                    data = json.loads(script.string)
                    if data.get('@type') == 'Product':
                        product_data['name'] = data.get('name')
                        product_data['description'] = data.get('description')
                        product_data['image'] = data.get('image')
                        product_data['brand'] = data.get('brand', {}).get('name')
                except:
                    pass
            
            if not product_data.get('name'):
                 h1 = await page.query_selector('h1')
                 if h1:
                     product_data['name'] = await h1.inner_text()

            # Dynamic Scraping of Key-Value pairs
            # Structure identified: 
            # <div class="...">
            #   <div class="w-1/2 ..."><h3>Key</h3></div>
            #   <div class="w-1/2 ..."><p>Value</p></div>
            # </div>
            
            # We'll find all h3 elements and check their "cousin" p element.
            try:
                # Get all h3 elements
                h3_elements = await page.query_selector_all('h3')
                
                for h3 in h3_elements:
                    key = await h3.inner_text()
                    if not key:
                        continue
                        
                    # Normalize key
                    key = key.strip().lower()
                    slug_key = key.replace(" ", "_").replace("/", "_").replace("-", "_")
                    
                    # Traverse to value: h3 -> parent div -> next sibling div -> p
                    # We can use xpath relative to the handle
                    # Note: Playwright ElementHandle xpath querying is a bit different, 
                    # often easier to just use page.evaluate or careful selector construction.
                    
                    # Let's try to get the parent's next sibling.
                    # This is tricky with pure ElementHandle API without evaluation.
                    # Using Xpath on the page might be safer if we can construct it unique to this h3, 
                    # but we don't have unique IDs.
                    
                    # Evaluating JS to get the value path
                    value = await h3.evaluate("""(element) => {
                        const parent = element.parentElement;
                        if (!parent) return null;
                        const nextDiv = parent.nextElementSibling;
                        if (!nextDiv) return null;
                        const p = nextDiv.querySelector('p');
                        return p ? p.textContent : null;
                    }""")
                    
                    if value:
                        product_data[slug_key] = value.strip()

            except Exception as e:
                logger.error(f"Error scraping key-values: {e}")

            # Parse Nutrition Info
            raw_nutrition = product_data.get('nutrition_information')
            if not raw_nutrition:
                for k, v in product_data.items():
                    if 'nutrition' in k or 'facts' in k:
                        raw_nutrition = v
                        break
            
            nutrition_json = {}
            if raw_nutrition:
                nutrition_json['raw'] = raw_nutrition
                import re
                
                def extract_val(pattern, text):
                    match = re.search(pattern, text, re.IGNORECASE)
                    if match:
                        try:
                            val = re.sub(r'[^\d.]', '', match.group(1))
                            return float(val)
                        except:
                            return None
                    return None

                # Extract raw macros
                protein = extract_val(r"Protein.*?(\d+(?:\.\d+)?)", raw_nutrition)
                fat = extract_val(r"Total Fat.*?(\d+(?:\.\d+)?)", raw_nutrition) or extract_val(r"Fat.*?(\d+(?:\.\d+)?)", raw_nutrition)
                carbs = extract_val(r"Carbohydrate.*?(\d+(?:\.\d+)?)", raw_nutrition) or extract_val(r"Carb.*?(\d+(?:\.\d+)?)", raw_nutrition)
                energy = extract_val(r"Energy.*?(\d+(?:\.\d+)?)", raw_nutrition) or extract_val(r"Calories.*?(\d+(?:\.\d+)?)", raw_nutrition)

                # Determine Basis and Normalize
                # 1. Check for explicit basis key in product_data
                unit_hint = (product_data.get('unit') or "").lower()
                is_liquid = "ml" in unit_hint or "litre" in unit_hint or "liter" in unit_hint
                default_basis_unit = "ml" if is_liquid else "g"
                basis = f"per_100{default_basis_unit}"
                
                raw_basis = product_data.get('nutrition_information_basis') or product_data.get('nutritional_value_basis')
                
                if raw_basis:
                    if "serving" in raw_basis.lower() or "sachet" in raw_basis.lower() or "unit" in raw_basis.lower():
                        basis = "per_serving"
                    elif "100" in raw_basis.lower():
                        basis_unit = "ml" if "ml" in raw_basis.lower() else "g"
                        basis = f"per_100{basis_unit}"
                else:
                    # 2. Check within raw_nutrition text
                    if "per serving" in raw_nutrition.lower() or "per sachet" in raw_nutrition.lower() or "per stick" in raw_nutrition.lower():
                        basis = "per_serving"
                    elif "per 100 ml" in raw_nutrition.lower():
                        basis = "per_100ml"
                    elif "per 100 g" in raw_nutrition.lower() or "per 100" in raw_nutrition.lower():
                         basis = "per_100g"
                    else:
                        # 3. Heuristic: If serving_size is small and calories are low (<50 kcal), 
                        # it's very likely per serving.
                        for k in product_data.keys():
                            if k == 'serving_size' and product_data[k]:
                                # If energy is found and is small
                                if energy is not None and energy < 50:
                                     basis = "per_serving"
                                     break
                
                product_data['nutrition_basis'] = basis
                
                # Parse Serving Size if exists
                serving_val = None
                serving_unit = None
                raw_serving = product_data.get('serving_size')
                if raw_serving:
                    match = re.search(r"(\d+(?:\.\d+)?)\s*([a-zA-Z]+)", raw_serving)
                    if match:
                        serving_val = float(match.group(1))
                        serving_unit = match.group(2).lower()
                        product_data['serving_size_value'] = serving_val
                        product_data['serving_size_unit'] = serving_unit

                # Normalize to 100 units if basis is per_serving
                if basis == "per_serving" and serving_val:
                    factor = 100.0 / serving_val
                    target_unit = serving_unit if serving_unit in ["g", "ml"] else default_basis_unit
                    nutrition_json['protein'] = round(protein * factor, 2) if protein is not None else None
                    nutrition_json['fat'] = round(fat * factor, 2) if fat is not None else None
                    nutrition_json['carbohydrates'] = round(carbs * factor, 2) if carbs is not None else None
                    nutrition_json['energy'] = round(energy * factor, 2) if energy is not None else None
                    logger.info(f"Normalized nutrition for {product_data.get('name')} from {serving_val}{serving_unit} to 100{target_unit} (Factor: {factor})")
                    product_data['nutrition_basis'] = f"per_100{target_unit}"
                else:
                    nutrition_json['protein'] = protein
                    nutrition_json['fat'] = fat
                    nutrition_json['carbohydrates'] = carbs
                    nutrition_json['energy'] = energy

                product_data['nutrition_info'] = nutrition_json
            
            # Extract Breadcrumbs for validation
            breadcrumbs = await page.evaluate("""() => {
                const items = document.querySelectorAll('[class*="breadcrumb"] li, [class*="Breadcrumb"] span');
                return Array.from(items).map(i => i.textContent.trim()).filter(t => t && t !== '>');
            }""")
            product_data['breadcrumbs'] = breadcrumbs
            
            # Simple validation check
            if not self.is_consumable(product_data):
                logger.warning(f"Skipping non-consumable product: {product_data.get('name')} (Categories: {breadcrumbs})")
                return None

            return product_data

        except Exception as e:
            logger.error(f"Error scraping {url}: {e}")
            return None
        finally:
            await page.close()

    async def crawl_category(self, category_url: str, max_products: int = 100):
        """
        Crawls a category page to find product URLs.
        Scrolls down until enough products are found or end of page.
        """
        if not self.browser:
            await self.start()
            
        page = await self.context.new_page()
        product_urls = set()
        
        try:
            logger.info(f"Crawling category: {category_url}")
            await page.goto(category_url, wait_until='domcontentloaded', timeout=60000)
            await page.wait_for_timeout(3000)

            last_height = await page.evaluate("document.body.scrollHeight")
            
            while len(product_urls) < max_products:
                # Extract links
                # Search pages might use different structures.
                # All product links still contain "/pn/"
                links = await page.evaluate("""() => {
                    const anchors = Array.from(document.querySelectorAll('a'));
                    return anchors
                        .map(a => a.href)
                        .filter(href => href && href.includes('/pn/'));
                }""")
                
                # Filter for valid product links (Zepto specific structure)
                # Usually: https://zepto.com/pn/slug/pvid/uuid
                # Ensure we don't get duplicates or non-product pages
                new_links = [l for l in links if '/pn/' in l and '/pvid/' in l]
                
                initial_count = len(product_urls)
                
                initial_count = len(product_urls)
                product_urls.update(new_links)
                logger.info(f"Found {len(product_urls)} distinct products so far...")
                
                if len(product_urls) >= max_products:
                    break
                
                # Scroll down
                await page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                await page.wait_for_timeout(2000)
                
                new_height = await page.evaluate("document.body.scrollHeight")
                if new_height == last_height:
                    logger.info("Reached end of category page.")
                    break
                last_height = new_height
                
        except Exception as e:
            logger.error(f"Error crawling category {category_url}: {e}")
        finally:
            await page.close()
            
        return list(product_urls)

    def is_consumable(self, product_data: dict) -> bool:
        """
        Check if the product is a consumable based on breadcrumbs or category.
        """
        breadcrumbs = product_data.get('breadcrumbs', [])
        # Whitelist of consumable categories (English)
        consumable_keywords = [
            'munchies', 'dairy', 'bakery', 'fruits', 'vegetables', 
            'meat', 'seafood', 'beverages', 'packaged food', 
            'breakfast', 'sauces', 'snacks', 'biscuits', 'ice cream',
            'cooking essentials', 'atta', 'rice', 'dal', 'oil', 'spices'
        ]
        
        # Blacklist of non-consumables
        non_consumable_keywords = [
            'cleaning', 'detergent', 'household', 'beauty', 
            'personal care', 'electronics', 'baby care'
        ]
        
        # Join breadcrumbs for searching
        bc_text = " ".join(breadcrumbs).lower()
        
        # If any blacklist keyword is found, it's not a consumable
        if any(k in bc_text for k in non_consumable_keywords):
            return False
            
        # If no breadcrumbs found, assume it's okay (fallback)
        if not breadcrumbs:
            return True
            
        # Check if it matches any consumable keyword
        return any(k in bc_text for k in consumable_keywords)

# Standalone runner for testing
if __name__ == "__main__":
    async def main():
        scraper = ZeptoScraper()
        # Test with the URL provided by user
        url = "https://www.zepto.com/pn/lays-american-cream-onion-potato-chips/pvid/1e28f2c0-fbc5-4366-9efb-a65bd12428cb"
        data = await scraper.scrape_product(url)
        print(json.dumps(data, indent=2))
        await scraper.stop()

    asyncio.run(main())
