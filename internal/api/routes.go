package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"coffedb/internal/config"
	"coffedb/internal/storage"
)

// Server represents the HTTP server
type Server struct {
	engine   *storage.Engine
	handlers *Handlers
	router   *gin.Engine
	server   *http.Server
	config   *config.Config
}

// NewServer creates a new HTTP server
func NewServer(engine *storage.Engine, cfg *config.Config) *Server {
	handlers := NewHandlers(engine)
	
	// Set gin mode
	if cfg.Server.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	
	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(jsonMiddleware())

	server := &Server{
		engine:   engine,
		handlers: handlers,
		router:   router,
		config:   cfg,
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// API version 1
	v1 := s.router.Group("/api/v1")
	
	// Health and stats endpoints
	v1.GET("/:id", s.handlers.Custom)
	v1.GET("/health", s.handlers.HealthCheck)
	v1.GET("/stats", s.handlers.GetStats)
	// v1.GET("/:id", s.handlers.Custom)
	
	// Collection routes
	collections := v1.Group("/collections/:collection")
	{
		// Document CRUD operations
		documents := collections.Group("/documents")
		{
			documents.POST("", s.handlers.CreateDocument)
			documents.GET("/:id", s.handlers.GetDocument)
			documents.PUT("/:id", s.handlers.UpdateDocument)
			documents.DELETE("/:id", s.handlers.DeleteDocument)
		}
		
		// Query endpoint
		collections.GET("/query", s.handlers.QueryDocuments)
		
		// Index management
		collections.POST("/indexes", s.handlers.CreateIndex)
	}

	// Root endpoint
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        "CoffeDB",
			"version":     "1.0.0",
			"description": "Production-ready NoSQL document database",
			"endpoints": gin.H{
				"health":      "/api/v1/health",
				"stats":       "/api/v1/stats",
				"collections": "/api/v1/collections",
			},
		})
	})
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(s.config.Server.IdleTimeout) * time.Second,
	}

	log.Printf("Server starting on %s", addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	if s.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.server.Shutdown(ctx)
}

// Middleware functions

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func jsonMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.Next()
	}
}
