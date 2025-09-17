package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Simple test to verify the database works
func main() {
	baseURL := "http://localhost:8080/api/v1"
	
	fmt.Println("🧪 Testing CoffeDB...")
	
	// Test 1: Health check
	fmt.Print("1. Health check... ")
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("❌ FAILED")
		return
	}
	fmt.Println("✅ PASSED")
	
	// Test 2: Create document
	fmt.Print("2. Create document... ")
	user := map[string]interface{}{
		"name":  "Alice Johnson",
		"email": "alice@example.com", 
		"age":   28,
	}
	
	data, _ := json.Marshal(user)
	resp, err = http.Post(baseURL+"/collections/users/documents", "application/json", bytes.NewBuffer(data))
	if err != nil || resp.StatusCode != 201 {
		fmt.Println("❌ FAILED")
		return
	}
	
	// Extract document ID from response
	var createResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResp)
	docID := createResp["id"].(string)
	fmt.Printf("✅ PASSED (ID: %s)\n", docID[:8]+"...")
	
	// Test 3: Get document
	fmt.Print("3. Get document... ")
	resp, err = http.Get(fmt.Sprintf("%s/collections/users/documents/%s", baseURL, docID))
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("❌ FAILED")
		return
	}
	fmt.Println("✅ PASSED")
	
	// Test 4: Query documents
	fmt.Print("4. Query documents... ")
	resp, err = http.Get(baseURL + "/collections/users/query?age=28")
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("❌ FAILED")
		return
	}
	
	var queryResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&queryResp)
	if queryResp["count"].(float64) < 1 {
		fmt.Println("❌ FAILED - No documents found")
		return
	}
	fmt.Println("✅ PASSED")
	
	// Test 5: Create index
	fmt.Print("5. Create index... ")
	indexData := map[string]string{"field": "email"}
	data, _ = json.Marshal(indexData)
	resp, err = http.Post(baseURL+"/collections/users/indexes", "application/json", bytes.NewBuffer(data))
	if err != nil || resp.StatusCode != 201 {
		fmt.Println("❌ FAILED")
		return
	}
	fmt.Println("✅ PASSED")
	
	// Test 6: Get stats
	fmt.Print("6. Get stats... ")
	resp, err = http.Get(baseURL + "/stats")
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("❌ FAILED")
		return
	}
	fmt.Println("✅ PASSED")
	
	fmt.Println()
	fmt.Println("🎉 All tests passed! CoffeDB is working correctly.")
	fmt.Println("📊 Database contains your test data and is ready for use.")
}