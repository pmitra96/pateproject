from sqlalchemy import Column, String, Float, Integer, ForeignKey, JSON, DateTime, ARRAY, Boolean
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import relationship
from datetime import datetime
import uuid
from app.database import Base

def generate_uuid():
    return str(uuid.uuid4())

class Brand(Base):
    __tablename__ = "brands"

    id = Column(String, primary_key=True, default=generate_uuid)
    name = Column(String, unique=True, index=True)
    slug = Column(String, index=True)
    created_at = Column(DateTime, default=datetime.utcnow)

    products = relationship("Product", back_populates="brand")

class Category(Base):
    __tablename__ = "categories"

    id = Column(String, primary_key=True, default=generate_uuid)
    name = Column(String, nullable=False)
    parent_id = Column(String, ForeignKey("categories.id"), nullable=True)
    search_query = Column(String)
    start_url = Column(String) # For direct scraping if available
    is_leaf = Column(Boolean, default=False)
    created_at = Column(DateTime, default=datetime.utcnow)

    # Relationships
    parent = relationship("Category", remote_side=[id], backref="children")
    products = relationship("Product", back_populates="category")


class Product(Base):
    __tablename__ = "products"

    id = Column(String, primary_key=True, default=generate_uuid)
    brand_id = Column(String, ForeignKey("brands.id"))
    name = Column(String, index=True)
    zepto_id = Column(String, unique=True, index=True)
    url = Column(String)
    image_url = Column(String)
    
    # New Category link
    category_id = Column(String, ForeignKey("categories.id"), nullable=True)
    
    tags = Column(JSON)
    nutrition_info = Column(JSON)
    
    # Detailed Info
    product_type = Column(String)
    flavour = Column(String)
    variant = Column(String)
    key_features = Column(String) # Could be Text for longer content
    spice_level = Column(String)
    material_type_free = Column(String)
    ingredients = Column(String) # Text
    allergen_information = Column(String)
    fssai_license = Column(String)
    dietary_preference = Column(String)
    cuisine_type = Column(String)
    weight = Column(String)
    unit = Column(String)
    packaging_type = Column(String)
    storage_instruction = Column(String)
    is_perishable = Column(String) # Keeping as String "Yes"/"No" or boolean if we parse it
    
    # Nutrition Normalization Fields
    nutrition_basis = Column(String) # "per_100g", "per_serving", "per_unit"
    serving_size_value = Column(Float)
    serving_size_unit = Column(String) # "g", "ml", "sachet", etc.
    
    serving_size = Column(String) # Raw string from Zepto

    last_scraped_at = Column(DateTime, default=datetime.utcnow)

    brand = relationship("Brand", back_populates="products")
    category = relationship("Category", back_populates="products")
