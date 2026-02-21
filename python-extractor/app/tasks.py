from app.worker import celery_app
from app.scraper import ZeptoScraper
from app.scrapers.instamart import InstamartScraper
import asyncio
from app.database import SessionLocal
from app.models import Product, Brand
import logging

logger = logging.getLogger(__name__)

async def _scrape_product_async(url: str, category_id: str = None):
    if "swiggy.com" in url:
        scraper = InstamartScraper()
    else:
        scraper = ZeptoScraper()
        
    try:
        data = await scraper.scrape_product(url)
        if data:
            # Save to DB
            db = SessionLocal()
            try:
                # Check if brand exists, else create
                brand_name = data.get('brand')
                brand = None
                if brand_name:
                    brand = db.query(Brand).filter(Brand.name == brand_name).first()
                    if not brand:
                        brand = Brand(name=brand_name, slug=brand_name.lower().replace(" ", "-")) # Simple slug
                        db.add(brand)
                        db.commit()
                        db.refresh(brand)
                
                # Check if product exists
                # Using name as unique identifier if zepto_id is not available
                product_name = data.get('name')
                if product_name:
                    product = db.query(Product).filter(Product.name == product_name).first() # Should use Zepto ID ideally
                    
                    status = "created"
                    if not product:
                        product = Product(
                            name=product_name,
                            url=url,
                            image_url=data.get('image'),
                            brand_id=brand.id if brand else None,
                            category_id=category_id,
                            nutrition_info=data.get('nutrition_info'),
                            
                            # Mapped Fields
                            product_type=data.get('product_type'),
                            flavour=data.get('flavour'),
                            variant=data.get('variant'),
                            key_features=data.get('key_features'),
                            spice_level=data.get('spice_level'),
                            material_type_free=data.get('material_type_free'),
                            ingredients=data.get('ingredients'),
                            allergen_information=data.get('allergen_information'),
                            fssai_license=data.get('fssai_license'),
                            dietary_preference=data.get('dietary_preference'),
                            cuisine_type=data.get('cuisine_type'),
                            weight=data.get('weight'),
                            unit=data.get('unit'),
                            packaging_type=data.get('packaging_type'),
                            storage_instruction=data.get('storage_instruction'),
                            is_perishable=data.get('is_perishable'),
                            serving_size=data.get('serving_size'),
                            
                            # Normalization fields
                            nutrition_basis=data.get('nutrition_basis'),
                            serving_size_value=data.get('serving_size_value'),
                            serving_size_unit=data.get('serving_size_unit'),
                        )
                        db.add(product)
                    else:
                        status = "updated"
                        # Update
                        product.nutrition_info = data.get('nutrition_info')
                        product.image_url = data.get('image')
                        product.ingredients = data.get('ingredients')
                        product.allergen_information = data.get('allergen_information')
                        
                        # Update normalization fields
                        product.nutrition_basis = data.get('nutrition_basis')
                        product.serving_size_value = data.get('serving_size_value')
                        product.serving_size_unit = data.get('serving_size_unit')
                        
                        if category_id:
                            product.category_id = category_id
                    
                    db.commit()
                    logger.info(f"Successfully {status}: {product_name}")
                    return f"Product {status}: {product_name}"
            except Exception as e:
                db.rollback()
                logger.error(f"DB Error: {e}")
                raise e
            finally:
                db.close()
        else:
            logger.warning(f"No data found for {url}")
            return "No data found"
    finally:
        await scraper.stop()

@celery_app.task(
    bind=True, 
    max_retries=5, 
    default_retry_delay=60,
    rate_limit="10/m" # Significant reduction to avoid 429s
)
def scrape_product_task(self, url: str, category_id: str = None):
    import random
    import time
    
    # Add jitter to avoid synchronized bursts
    time.sleep(random.uniform(2, 5))
    
    try:
        loop = asyncio.get_event_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        
    try:
        result = loop.run_until_complete(_scrape_product_async(url, category_id))
        if result == "No data found":
             # This often happens on 429 or anti-bot
             # We should retry with backoff
             logger.warning(f"Likely blocked or empty data for {url}, retrying...")
             raise self.retry(countdown=random.randint(120, 300))
        return result
    except Exception as exc:
        # Retry on most exceptions (e.g. timeout, network)
        logger.error(f"Retrying task for {url} due to: {exc}")
        raise self.retry(exc=exc, countdown=60)

async def _crawl_category_async(url: str, max_products: int, category_id: str = None):
    if "swiggy.com" in url:
        scraper = InstamartScraper()
    else:
        scraper = ZeptoScraper()
        
    try:
        product_urls = await scraper.crawl_category(url, max_products)
        logger.info(f"Found {len(product_urls)} products in {url}")
        for p_url in product_urls:
            scrape_product_task.delay(p_url, category_id)
        return len(product_urls)
    finally:
        await scraper.stop()

@celery_app.task(bind=True, rate_limit="5/m") # Slower rate for category pages
def crawl_category_task(self, url: str, max_products: int = 1000, category_id: str = None):
    try:
        loop = asyncio.get_event_loop()
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    
    try:
        return loop.run_until_complete(_crawl_category_async(url, max_products, category_id))
    except Exception as exc:
        logger.error(f"Error crawling category {url}: {exc}")
        raise self.retry(exc=exc, countdown=120)
