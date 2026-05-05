package main

import (
	"github.com/pmitra96/pateproject/logger"
)

func main() {
	logger.Init()
	logger.Info("This should be JSON format if ENV=production")
}
