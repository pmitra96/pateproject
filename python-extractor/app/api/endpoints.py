from fastapi import APIRouter, Depends, HTTPException, BackgroundTasks
from sqlalchemy.orm import Session
from app.database import get_db
from app.models import Product, Brand
from app.tasks import scrape_product_task
from app.scraper import ZeptoScraper
import asyncio

router = APIRouter()

@router.post("/scrape/category")
async def trigger_category_scrape(url: str, max_products: int = 50):
    """
    Crawls a category and queues scraping tasks for each product found.
    """
    scraper = ZeptoScraper()
    try:
        product_urls = await scraper.crawl_category(url, max_products=max_products)
        await scraper.stop()
        
        for p_url in product_urls:
            scrape_product_task.delay(p_url)
            
        return {"message": f"Queued {len(product_urls)} products for scraping", "urls": product_urls}
    except Exception as e:
        return {"error": str(e)}

@router.post("/scrape/search/trigger")
async def trigger_search_scrape(query: str, max_products: int = 20):
    """
    Triggers a search on Zepto and queues results for scraping.
    """
    search_url = f"https://www.zepto.com/search?query={query}"
    scraper = ZeptoScraper()
    try:
        product_urls = await scraper.crawl_category(search_url, max_products=max_products)
        await scraper.stop()
        
        for p_url in product_urls:
            scrape_product_task.delay(p_url)
            
        return {"message": f"Queued {len(product_urls)} products for query '{query}'", "urls": product_urls}
    except Exception as e:
        return {"error": str(e)}

@router.get("/nutrition")
def get_nutrition(brand_name: str = None, item_name: str = None, db: Session = Depends(get_db)):
    query = db.query(Product).join(Brand)
    
    if brand_name:
        query = query.filter(Brand.name.ilike(f"%{brand_name}%"))
    
    if item_name:
        query = query.filter(Product.name.ilike(f"%{item_name}%"))
        
    products = query.all()
    return products

@router.get("/products/search")
def search_products(query: str, min_protein: float = None, max_fat: float = None, db: Session = Depends(get_db)):
    db_query = db.query(Product).filter(Product.name.ilike(f"%{query}%"))
    
    # Note: Complex JSON filtering is DB-specific. 
    # For SQLite/Simple JSON, we might need to filter in Python or use specific JSON operators if supported.
    # This is a simplified implementation.
    products = db_query.all()
    
    filtered_products = []
    for product in products:
        if not product.nutrition_info:
            continue
            
        protein = float(product.nutrition_info.get("protein", 0))
        fat = float(product.nutrition_info.get("fat", 0))
        
        if min_protein is not None and protein < min_protein:
            continue
        if max_fat is not None and fat > max_fat:
            continue
            
        filtered_products.append(product)
        
    return filtered_products
