from celery import Celery
import os
from dotenv import load_dotenv

load_dotenv()

# Use Redis as broker and backend
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")

# Support for rediss:// (SSL) required by some providers like Upstash
broker_use_ssl = None
redis_backend_use_ssl = None

if REDIS_URL.startswith("rediss://"):
    import ssl
    ssl_conf = {
        'ssl_cert_reqs': ssl.CERT_NONE  # Common for serverless/hosted Redis without custom CA
    }
    broker_use_ssl = ssl_conf
    redis_backend_use_ssl = ssl_conf

celery_app = Celery(
    "zepto_scraper",
    broker=REDIS_URL,
    backend=REDIS_URL,
    include=["app.tasks"]
)

celery_app.conf.update(
    broker_use_ssl=broker_use_ssl,
    redis_backend_use_ssl=redis_backend_use_ssl,
    task_serializer="json",
    accept_content=["json"],
    result_serializer="json",
    timezone="UTC",
    enable_utc=True,
    # High volume scraping tweaks
    worker_prefetch_multiplier=1, # One task at a time per worker process for Playwright
    task_acks_late=True,
    task_reject_on_worker_lost=True,
    task_time_limit=120, # Max 2 mins per page
)

if __name__ == "__main__":
    celery_app.start()
