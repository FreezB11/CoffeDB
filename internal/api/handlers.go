package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"coffedb/internal/storage"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	engine *storage.Engine
}

// NewHandlers creates a new handlers instance
func NewHandlers(engine *storage.Engine) *Handlers {
	return &Handlers{
		engine: engine,
	}
}

// CreateDocument creates a new document in a collection
func (h *Handlers) CreateDocument(c *gin.Context) {
	collection := c.Param("collection")
	
	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON body",
			"details": err.Error(),
		})
		return
	}

	// Generate ID if not provided
	id, exists := requestBody["id"]
	if !exists {
		id = generateID()
		requestBody["id"] = id
	}

	// Remove ID from data
	delete(requestBody, "id")

	if err := h.engine.Put(collection, fmt.Sprintf("%v", id), requestBody); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create document",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id": id,
		"message": "Document created successfully",
	})
}

// GetDocument retrieves a document by ID
func (h *Handlers) GetDocument(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")

	doc, err := h.engine.Get(collection, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Document not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, doc)
}

// UpdateDocument updates an existing document
func (h *Handlers) UpdateDocument(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")

	var requestBody map[string]interface{}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON body",
			"details": err.Error(),
		})
		return
	}

	// Check if document exists
	_, err := h.engine.Get(collection, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Document not found",
			"details": err.Error(),
		})
		return
	}

	if err := h.engine.Put(collection, id, requestBody); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update document",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Document updated successfully",
	})
}

// DeleteDocument deletes a document by ID
func (h *Handlers) DeleteDocument(c *gin.Context) {
	collection := c.Param("collection")
	id := c.Param("id")

	if err := h.engine.Delete(collection, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete document",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Document deleted successfully",
	})
}

// QueryDocuments queries documents in a collection
func (h *Handlers) QueryDocuments(c *gin.Context) {
	collection := c.Param("collection")

	// Parse query parameters
	filter := make(map[string]interface{})
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 && key != "limit" && key != "offset" {
			// Try to parse as number, fall back to string
			if num, err := strconv.Atoi(values[0]); err == nil {
				filter[key] = num
			} else if num, err := strconv.ParseFloat(values[0], 64); err == nil {
				filter[key] = num
			} else if values[0] == "true" || values[0] == "false" {
				filter[key] = values[0] == "true"
			} else {
				filter[key] = values[0]
			}
		}
	}

	// Parse limit and offset
	limit := 100 // default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	docs, err := h.engine.Query(collection, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query documents",
			"details": err.Error(),
		})
		return
	}

	// Apply pagination
	total := len(docs)
	start := offset
	end := offset + limit

	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	result := docs[start:end]

	c.JSON(http.StatusOK, gin.H{
		"documents": result,
		"total": total,
		"limit": limit,
		"offset": offset,
		"count": len(result),
	})
}

// CreateIndex creates a secondary index on a field
func (h *Handlers) CreateIndex(c *gin.Context) {
	collection := c.Param("collection")

	var requestBody struct {
		Field string `json:"field" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	if err := h.engine.CreateIndex(collection, requestBody.Field); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create index",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": fmt.Sprintf("Index created on field '%s'", requestBody.Field),
	})
}

// HealthCheck returns the health status of the database
func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version": "1.0.0",
	})
}

// GetStats returns database statistics
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.engine.Stats()
	
	c.JSON(http.StatusOK, gin.H{
		"database": "CoffeDB",
		"version": "1.0.0",
		"uptime": "running", // Would track actual uptime
		"statistics": stats,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// Helper functions

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
