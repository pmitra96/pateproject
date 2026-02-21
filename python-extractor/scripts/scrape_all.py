import sys
import os
import logging

# Add parent directory to path
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from app.database import SessionLocal
from app.models import Category
from app.tasks import crawl_category_task

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def scrape_all_categories():
    db = SessionLocal()
    try:
        # Fetch all leaf categories with a start_url
        categories = db.query(Category).filter(
            Category.is_leaf == True,
            Category.start_url != None
        ).all()
        
        logger.info(f"Found {len(categories)} leaf categories to scrape.")
        
        for cat in categories:
            logger.info(f"Queueing scrape for: {cat.name} ({cat.start_url})") 
            # Set high limit to scrape all products in the category
            crawl_category_task.delay(cat.start_url, max_products=1000, category_id=str(cat.id))
            
        logger.info(f"Successfully queued {len(categories)} category crawl tasks.")
        logger.info("Monitor progress using: celery -A app.worker.celery_app inspect active")
        
    except Exception as e:
        logger.error(f"Error: {e}")
    finally:
        db.close()

if __name__ == "__main__":
    scrape_all_categories()
