package query

import (
	"fmt"
	"reflect"
	"strings"
)

// Processor handles query processing and filtering
type Processor struct{}

// NewProcessor creates a new query processor
func NewProcessor() *Processor {
	return &Processor{}
}

// Filter applies a filter to a document
func (p *Processor) Filter(doc map[string]interface{}, filter map[string]interface{}) bool {
	for key, expectedValue := range filter {
		if !p.matchField(doc, key, expectedValue) {
			return false
		}
	}
	return true
}

// matchField checks if a field matches the expected value
func (p *Processor) matchField(doc map[string]interface{}, fieldPath string, expectedValue interface{}) bool {
	value := p.getNestedValue(doc, fieldPath)
	return p.compareValues(value, expectedValue)
}

// getNestedValue retrieves a value from a nested object using dot notation
func (p *Processor) getNestedValue(doc map[string]interface{}, fieldPath string) interface{} {
	parts := strings.Split(fieldPath, ".")
	current := doc
	
	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}
		
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	
	return nil
}

// compareValues compares two values for equality
func (p *Processor) compareValues(actual, expected interface{}) bool {
	if actual == nil && expected == nil {
		return true
	}
	
	if actual == nil || expected == nil {
		return false
	}
	
	// Handle different types
	actualValue := reflect.ValueOf(actual)
	expectedValue := reflect.ValueOf(expected)
	
	// Try direct comparison first
	if actualValue.Type() == expectedValue.Type() {
		return reflect.DeepEqual(actual, expected)
	}
	
	// Handle numeric comparisons
	if isNumeric(actualValue) && isNumeric(expectedValue) {
		return compareNumeric(actualValue, expectedValue)
	}
	
	// Handle string comparisons
	if actualValue.Kind() == reflect.String && expectedValue.Kind() == reflect.String {
		return actual.(string) == expected.(string)
	}
	
	return false
}

// isNumeric checks if a value is numeric
func isNumeric(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		 reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		 reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// compareNumeric compares two numeric values
func compareNumeric(a, b reflect.Value) bool {
	aFloat := convertToFloat64(a)
	bFloat := convertToFloat64(b)
	return aFloat == bFloat
}

// convertToFloat64 converts a numeric value to float64
func convertToFloat64(v reflect.Value) float64 {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint())
	case reflect.Float32, reflect.Float64:
		return v.Float()
	}
	return 0
}

// ParseQuery parses a query string into a filter map
func (p *Processor) ParseQuery(queryStr string) (map[string]interface{}, error) {
	// Simplified query parsing - in production would support more complex queries
	filter := make(map[string]interface{})
	
	if queryStr == "" {
		return filter, nil
	}
	
	// For now, just return empty filter
	// In production, this would parse SQL-like queries or JSON queries
	return filter, nil
}

// Aggregate performs aggregation operations on a set of documents
func (p *Processor) Aggregate(docs []map[string]interface{}, operations []AggregateOp) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	for _, op := range operations {
		switch op.Type {
		case "count":
			result[op.Field] = len(docs)
		case "sum":
			sum, err := p.sum(docs, op.Field)
			if err != nil {
				return nil, err
			}
			result[op.Field] = sum
		case "avg":
			avg, err := p.average(docs, op.Field)
			if err != nil {
				return nil, err
			}
			result[op.Field] = avg
		case "min":
			min, err := p.minimum(docs, op.Field)
			if err != nil {
				return nil, err
			}
			result[op.Field] = min
		case "max":
			max, err := p.maximum(docs, op.Field)
			if err != nil {
				return nil, err
			}
			result[op.Field] = max
		}
	}
	
	return result, nil
}

// AggregateOp represents an aggregation operation
type AggregateOp struct {
	Type  string `json:"type"`
	Field string `json:"field"`
}

// Helper functions for aggregation

func (p *Processor) sum(docs []map[string]interface{}, field string) (float64, error) {
	sum := 0.0
	for _, doc := range docs {
		value := p.getNestedValue(doc, field)
		if num, ok := value.(float64); ok {
			sum += num
		} else if num, ok := value.(int); ok {
			sum += float64(num)
		}
	}
	return sum, nil
}

func (p *Processor) average(docs []map[string]interface{}, field string) (float64, error) {
	sum, err := p.sum(docs, field)
	if err != nil {
		return 0, err
	}
	
	if len(docs) == 0 {
		return 0, nil
	}
	
	return sum / float64(len(docs)), nil
}

func (p *Processor) minimum(docs []map[string]interface{}, field string) (interface{}, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents to aggregate")
	}

	var min interface{}
	
	for _, doc := range docs {
		value := p.getNestedValue(doc, field)
		if min == nil {
			min = value
		} else if p.isLess(value, min) {
			min = value
		}
	}
	
	return min, nil
}

func (p *Processor) maximum(docs []map[string]interface{}, field string) (interface{}, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents to aggregate")
	}
	
	var max interface{}
	
	for _, doc := range docs {
		value := p.getNestedValue(doc, field)
		if max == nil {
			max = value
		} else if p.isGreater(value, max) {
			max = value
		}
	}
	
	return max, nil
}

func (p *Processor) isLess(a, b interface{}) bool {
	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)
	
	if isNumeric(aValue) && isNumeric(bValue) {
		return convertToFloat64(aValue) < convertToFloat64(bValue)
	}
	
	return false
}

func (p *Processor) isGreater(a, b interface{}) bool {
	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)
	
	if isNumeric(aValue) && isNumeric(bValue) {
		return convertToFloat64(aValue) > convertToFloat64(bValue)
	}
	
	return false
}
