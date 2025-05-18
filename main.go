package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"github.com/glebarez/sqlite"
	"github.com/labstack/echo/v4/middleware"
)

const PRODUCTS_ID = "/products/:id"


type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	CategoryID  uint      `json:"category_id"`
	Category    Category  `gorm:"foreignKey:CategoryID" json:"category"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Products  []Product `gorm:"foreignKey:CategoryID" json:"products"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Cart struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Products  []Product `gorm:"many2many:cart_products;" json:"products"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PaymentRequest struct {
    CartID     uint   `json:"cart_id"`
    CardNumber string `json:"card_number"`
    Amount     float64 `json:"amount"`
}

func main() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&Product{}, &Cart{}, &Category{})

	e := echo.New()

	 // middleware CORS
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins: []string{"*"},
        AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
    }))
    
	e.Use(DBMiddleware(db))

	e.POST("/test", func(c echo.Context) error {
		body, _ := io.ReadAll(c.Request().Body)
		fmt.Printf("\n--- RAW REQUEST BODY ---\n%s\n-----------------------\n", body)
		return c.String(http.StatusOK, string(body))
	})

	// Produkty
	e.POST("/products", createProduct)
	e.GET(PRODUCTS_ID, getProduct)
	e.GET("/products", getAllProducts)
	e.PUT(PRODUCTS_ID, updateProduct)
	e.DELETE(PRODUCTS_ID, deleteProduct)

	// Koszyki
	e.POST("/carts", createCart)
	e.POST("/carts/:id/products", addProductToCart)
	e.GET("/carts/:id", getCart)
	e.DELETE("/carts/:id/products/:productId", removeProductFromCart)

	// Kategorie
	e.POST("/categories", createCategory)
	e.GET("/categories/:id", getCategory)

	// Płatnosci
	e.POST("/payments", processPayment)
	e.Logger.Fatal(e.Start(":1323"))
}

func DBMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("db", db)
			return next(c)
		}
	}
}

// CRUD dla produktów
func createProduct(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	p := new(Product)
	if err := c.Bind(p); err != nil {
		return err
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	db.Create(p)
	return c.JSON(http.StatusCreated, p)
}

func getProduct(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	id := c.Param("id")
	var p Product
	db.Preload("Category").First(&p, id)
	return c.JSON(http.StatusOK, p)
}

func getAllProducts(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	var products []Product
	db.Preload("Category").Find(&products)
	return c.JSON(http.StatusOK, products)
}

func updateProduct(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	id := c.Param("id")
	var p Product
	if err := db.First(&p, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Product not found")
	}
	if err := c.Bind(&p); err != nil {
		return err
	}
	p.UpdatedAt = time.Now()
	db.Save(&p)
	return c.JSON(http.StatusOK, p)
}

func deleteProduct(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	id := c.Param("id")
	db.Delete(&Product{}, id)
	return c.NoContent(http.StatusNoContent)
}

// Obsługa koszyka
func createCart(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	cart := Cart{CreatedAt: time.Now(), UpdatedAt: time.Now()}
	db.Create(&cart)
	return c.JSON(http.StatusCreated, cart)
}

func addProductToCart(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	cartID := c.Param("id")
	
	var body struct{ ProductID uint `json:"product_id"` }
	if err := c.Bind(&body); err != nil {
		return err
	}

	var cart Cart
	var product Product
	db.First(&cart, cartID)
	db.First(&product, body.ProductID)

	db.Model(&cart).Association("Products").Append(&product)
	return c.JSON(http.StatusOK, cart)
}

func getCart(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	id := c.Param("id")
	var cart Cart
	db.Preload("Products").First(&cart, id)
	return c.JSON(http.StatusOK, cart)
}

// Obsługa kategorii
func createCategory(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	cat := new(Category)
	if err := c.Bind(cat); err != nil {
		return err
	}
	cat.CreatedAt = time.Now()
	cat.UpdatedAt = time.Now()
	db.Create(cat)
	return c.JSON(http.StatusCreated, cat)
}

func getCategory(c echo.Context) error {
	db := c.Get("db").(*gorm.DB)
	id := c.Param("id")
	var category Category
	if err := db.Preload("Products").First(&category, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Category not found")
	}
	return c.JSON(http.StatusOK, category)
}

// Funkcja obsługującą płatności
func processPayment(c echo.Context) error {
    payment := new(PaymentRequest)
    
    if err := c.Bind(payment); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid payment data")
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "status": "success",
        "transaction_id": time.Now().UnixNano(),
        "cart_id": payment.CartID,
    })
}

// Nowa funkcja obsługująca usuwanie produktu
func removeProductFromCart(c echo.Context) error {
    db := c.Get("db").(*gorm.DB)
    cartID := c.Param("id")
    productID := c.Param("productId")
    
    var cart Cart
    if err := db.Preload("Products").First(&cart, cartID).Error; err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Koszyk nie istnieje")
    }
    
    var product Product
    if err := db.First(&product, productID).Error; err != nil {
        return echo.NewHTTPError(http.StatusNotFound, "Produkt nie istnieje")
    }
    
    db.Model(&cart).Association("Products").Delete(&product)
    return c.JSON(http.StatusOK, cart)
}