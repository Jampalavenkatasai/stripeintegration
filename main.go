package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/checkout/session"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	stripeSecretKey = "sk_test_df98VGtVdmCoZdUbDPZjmoMq"
	dbUser          = "postgres"
	dbPassword      = "Sai@996361"
	dbName          = "postgres"
)

// User model for database

type User struct {
	gorm.Model
	StripeSessionID string
	Status          string
}

var Req struct {
	UserID string `json:"userId" binding:"required"`
	Amount int64  `json:"amount" binding:"required"`
}

func main() {
	stripe.Key = stripeSecretKey

	// Initialize Gin
	router := gin.Default()

	// Setup GORM with PostgreSQL
	dsn := fmt.Sprintf("user=%s password=%s dbname=%s sslmode=disable", dbUser, dbPassword, dbName)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to the database")
	}
	db.AutoMigrate(&User{})

	// Define routes
	router.POST("/create-stripe-session", createStripeSessionHandler(db))
	router.POST("/update-payment-status", updatePaymentStatusHandler(db))
	router.GET("/success", func(c *gin.Context) {
		// Load the success HTML page
		tmpl, err := loadHTMLPage("success.html")
		if err != nil {
			log.Printf("Error loading HTML page: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading HTML page"})
			return
		}

		// Execute the template and return as HTML response
		err = tmpl.Execute(c.Writer, nil)
		if err != nil {
			log.Printf("Error executing HTML template: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error executing HTML template"})
			return
		}
	})
	router.GET("/cancel", func(c *gin.Context) {
		// Load the success HTML page
		tmpl, err := loadHTMLPage("cancel.html")
		if err != nil {
			log.Printf("Error loading HTML page: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error loading HTML page"})
			return
		}

		// Execute the template and return as HTML response
		err = tmpl.Execute(c.Writer, nil)
		if err != nil {
			log.Printf("Error executing HTML template: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error executing HTML template"})
			return
		}
	})

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func createStripeSessionHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request parameters
		if err := c.ShouldBindJSON(&Req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		//get user name form user id
		username := "test"
		amount := Req.Amount * 100
		// Create a Checkout Session
		params := &stripe.CheckoutSessionParams{
			PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
			LineItems: []*stripe.CheckoutSessionLineItemParams{
				{
					Name:     stripe.String(username),
					Amount:   &amount,
					Quantity: stripe.Int64(1),
					Currency: stripe.String(string(stripe.CurrencyINR)),
				},
			},
			Mode:       stripe.String(string(stripe.CheckoutSessionModePayment)),
			SuccessURL: stripe.String("http://localhost:8080/success"),
			CancelURL:  stripe.String("http://localhost:8080/cancel"),
		}

		session, err := session.New(params)
		if err != nil {
			log.Printf("Error creating checkout session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating checkout session"})
			return
		}
		// Save the Stripe Session ID and status in the database
		user := User{StripeSessionID: session.ID, Status: "pending"}
		if err := db.Create(&user).Error; err != nil {
			log.Printf("Error saving user record: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving user record"})
			return
		}

		// Return the session ID and URL as JSON
		response := map[string]string{"id": session.ID, "payment_url": session.URL}
		c.JSON(http.StatusOK, response)
	}
}

func updatePaymentStatusHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		var req struct {
			UserID          string `json:"userId" binding:"required"`
			StripeSessionID string `json:"stripeSessionId" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Perform actions to update payment status in the database

		// Retrieve the session from Stripe
		s, err := session.Get(req.StripeSessionID, nil)
		if err != nil {
			log.Printf("Error retrieving session from Stripe: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving session from Stripe"})
			return
		}

		// Check the payment status
		if s.PaymentStatus == "paid" {
			// Payment successful, update the status in the database
			var user User
			if err := db.Where("stripe_session_id = ?", req.StripeSessionID).First(&user).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			user.Status = "success"
			// Update the payment status in the database
			if err := db.Save(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating payment status"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Payment verified and status updated successfully"})
		} else {
			// Payment not successful, handle accordingly
			c.JSON(http.StatusBadRequest, gin.H{"error": "Payment not successful"})
		}
	}

}

func loadHTMLPage(pageName string) (*template.Template, error) {
	// Get the current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting working directory: %v", err)
	}

	// Construct the path to the "templates" folder
	templatesDir := filepath.Join(workingDir, "templates")

	// Load the HTML page from the "templates" folder
	pagePath := filepath.Join(templatesDir, pageName)

	// Check if the file exists
	_, err = os.Stat(pagePath)
	if err != nil {
		return nil, fmt.Errorf("error loading HTML page: %v", err)
	}

	// Parse and return the HTML template
	tmpl, err := template.ParseFiles(pagePath)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML template: %v", err)
	}

	return tmpl, nil
}
