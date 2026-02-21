from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker, declarative_base
import os
from dotenv import load_dotenv

load_dotenv()

# Use PostgreSQL for scalability with 20+ workers
DATABASE_URL = os.getenv("DATABASE_URL")

if not DATABASE_URL or not DATABASE_URL.startswith("postgresql"):
    raise ValueError("DATABASE_URL must be a valid PostgreSQL connection string for this service to run.")

engine = create_engine(
    DATABASE_URL,
    # High concurrency settings
    pool_size=20,
    max_overflow=10,
    pool_timeout=30
)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

Base = declarative_base()

def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
