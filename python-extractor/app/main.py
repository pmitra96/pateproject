from fastapi import FastAPI
from app.api.endpoints import router as api_router
from app.api.extraction import router as extraction_router
from sqladmin import Admin, ModelView
from app.database import engine, Base
from app.models import Product, Brand, Category

# Initialize database tables
Base.metadata.create_all(bind=engine)

app = FastAPI(title="Zepto Scraper API")

# SQLAdmin Setup
admin = Admin(app, engine)

class ProductAdmin(ModelView, model=Product):
    column_list = [Product.name, Product.brand_id, Product.category_id, Product.product_type, Product.nutrition_info]
    column_details_list = [
        Product.name, Product.brand_id, Product.category_id, Product.product_type, Product.nutrition_info, 
        Product.ingredients, Product.image_url, Product.url, Product.zepto_id, Product.tags,
        Product.flavour, Product.variant, Product.weight, Product.unit, Product.serving_size
    ]
    column_searchable_list = [Product.name, Product.product_type]
    # column_filters = [Product.brand, Product.product_type]
    can_export = True
    can_view_details = True

class BrandAdmin(ModelView, model=Brand):
    column_list = [Brand.id, Brand.name, Brand.slug]
    column_searchable_list = [Brand.name]
    can_view_details = True

class CategoryAdmin(ModelView, model=Category):
    column_list = [Category.name, Category.parent_id, Category.search_query, Category.is_leaf]
    column_searchable_list = [Category.name]
    can_view_details = True

admin.add_view(ProductAdmin)
admin.add_view(BrandAdmin)
admin.add_view(CategoryAdmin)

app.include_router(api_router, prefix="/api/v1")
app.include_router(extraction_router)

@app.get("/")
def read_root():
    return {"message": "Zepto Nutrition Scraper API is up", "status": "healthy"}

@app.get("/health")
def health_check():
    return {"status": "up", "service": "python-extractor"}
