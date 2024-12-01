package route

import (
	"pateproject/config"
	controller "pateproject/controller"
	"pateproject/db"
	"pateproject/handler"
	"pateproject/middleware"
	"pateproject/model"
	"pateproject/repository"
	"pateproject/service"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) error {

	config, err := config.ReadConfig("config/development.yaml")
	if err != nil {
		return err
	}
	err = db.InitDB(config)
	if err != nil {
		return err
	}
	gormDbInstance := db.GetDBInstance()
	migrationErr := gormDbInstance.AutoMigrate(
		&model.Item{},
		&model.Unit{},
		&model.User{},
		&model.UserInventory{},
		&model.Recipe{},
		&model.RecipeIngredient{},
	)

	if migrationErr != nil {
		return migrationErr
	}

	itemRepository := repository.NewItemRepository(gormDbInstance)
	unitRepository := repository.NewUnitRepository(gormDbInstance)
	userRepository := repository.NewUserRepository(gormDbInstance)
	// Create an instance of InventoryController
	itemController := controller.NewitemController(itemRepository)
	unitController := controller.NewUnitController(unitRepository)
	userController := controller.NewUserController(userRepository)

	// Initialize services
	authService := service.NewAuthService(userController, config)
	authHandler := handler.NewAuthHandler(authService)

	itemHandler := handler.NewItemHandler(itemController)
	unitHandler := handler.NewUnitHandler(unitController)
	userHandler := handler.NewUserHandler(userController)

	// Initialize the Gin router

	publicRoutes := r.Group("/")
	publicRoutes.POST("/auth/login", authHandler.Login)

	publicRoutes.GET("/items/:id", itemHandler.GetItem)
	publicRoutes.POST("/items", itemHandler.Create)
	publicRoutes.PUT("/items/:id", itemHandler.UpdateItem)
	publicRoutes.DELETE("/items/:id", itemHandler.DeleteItem)

	publicRoutes.GET("/units/:id", unitHandler.GetUnit)
	publicRoutes.PUT("/units/:id", unitHandler.UpdateUnit)
	publicRoutes.POST("/units", unitHandler.Create)
	publicRoutes.DELETE("/units/:id", unitHandler.DeleteUnit)

	// user things
	protectedRoutes := r.Group("/")
	protectedRoutes.Use(middleware.AuthenticateJWT(config))
	protectedRoutes.GET("/users/:id", userHandler.GetUser)
	protectedRoutes.POST("/users", userHandler.Create)
	protectedRoutes.PUT("/users/:id", userHandler.UpdateUser)
	protectedRoutes.DELETE("/users/:id", userHandler.DeleteUser)

	// Define route for authentication

	return nil
}
