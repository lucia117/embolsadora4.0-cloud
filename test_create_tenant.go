package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type TenantRequest struct {
	Name        string `json:"name"`
	CompanyName string `json:"companyName"`
	Subdomain   string `json:"subdomain"`
	Description string `json:"description"`
	AdminUser   struct {
		Email     string `json:"email"`
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
		Password  string `json:"password"`
	} `json:"adminUser"`
	Theme struct {
		PrimaryColor    string `json:"primaryColor"`
		SecondaryColor  string `json:"secondaryColor"`
		AccentColor     string `json:"accentColor"`
		TextColor       string `json:"textColor"`
		BackgroundColor string `json:"backgroundColor"`
	} `json:"theme"`
	Address struct {
		Street     string `json:"street"`
		City       string `json:"city"`
		State      string `json:"state"`
		PostalCode string `json:"postalCode"`
		Country    string `json:"country"`
	} `json:"address"`
}

func main() {
	// Test data matching the API specification
	req := TenantRequest{
		Name:        "New Tenant",
		CompanyName: "New Company",
		Subdomain:   "new-tenant-test",
		Description: "A new tenant for testing",
		AdminUser: struct {
			Email     string `json:"email"`
			FirstName string `json:"firstName"`
			LastName  string `json:"lastName"`
			Password  string `json:"password"`
		}{
			Email:     "admin@new-tenant-test.com",
			FirstName: "Admin",
			LastName:  "User",
			Password:  "securePassword123!",
		},
		Theme: struct {
			PrimaryColor    string `json:"primaryColor"`
			SecondaryColor  string `json:"secondaryColor"`
			AccentColor     string `json:"accentColor"`
			TextColor       string `json:"textColor"`
			BackgroundColor string `json:"backgroundColor"`
		}{
			PrimaryColor:    "#3b82f6",
			SecondaryColor:  "#6366f1",
			AccentColor:     "#8b5cf6",
			TextColor:       "#1f2937",
			BackgroundColor: "#ffffff",
		},
		Address: struct {
			Street     string `json:"street"`
			City       string `json:"city"`
			State      string `json:"state"`
			PostalCode string `json:"postalCode"`
			Country    string `json:"country"`
		}{
			Street:     "456 New St",
			City:       "Buenos Aires",
			State:      "Buenos Aires",
			PostalCode: "C1002",
			Country:    "Argentina",
		},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return
	}

	// Try to make the request (assuming server is running on localhost:8080)
	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest("POST", "http://localhost:8080/api/tenants", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	fmt.Println("Testing tenant creation endpoint...")
	fmt.Printf("Request body: %s\n", string(jsonData))
	
	resp, err := client.Do(httpReq)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		fmt.Println("Make sure the API server is running on localhost:8080")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	fmt.Printf("Response Status: %s\n", resp.Status)
	fmt.Printf("Response Body: %s\n", string(body))
}
