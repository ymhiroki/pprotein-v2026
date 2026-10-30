package main

import (
	"os"

	"github.com/ymhiroki/pprotein-v2026/integration/standalone"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "19000"
	}
	standalone.Integrate(":" + port)
}
