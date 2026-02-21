from app.database import engine
from sqlalchemy import inspect

inspector = inspect(engine)
print("Existing tables:", inspector.get_table_names())
