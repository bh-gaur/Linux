package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer
var db *sql.DB
var dbConnected = false

func initTracer() (*sdktrace.TracerProvider, error) {
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "alloy:4318"
	}

	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("inventory-service"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func initDB() {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "postgres"
	}
	connStr := fmt.Sprintf("host=%s port=5432 user=lgtm_user password=lgtm_password dbname=lgtmdb sslmode=disable", dbHost)

	// Attempt connection with retry
	for i := 0; i < 5; i++ {
		var err error
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Inventory service successfully connected to PostgreSQL")
				dbConnected = true
				break
			}
		}
		log.Printf("PostgreSQL not ready yet, retrying in 3s (%d/5): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if dbConnected {
		// Ensure inventory table exists
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS inventory (
				item_id INT PRIMARY KEY,
				stock INT NOT NULL
			);
			INSERT INTO inventory (item_id, stock) VALUES 
			(101, 100), (102, 100), (103, 100), (104, 100)
			ON CONFLICT (item_id) DO NOTHING;
		`)
		if err != nil {
			log.Printf("Failed to bootstrap inventory table: %v", err)
		}
	}
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Printf("Failed to initialize OTel tracer: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
		tracer = otel.Tracer("inventory-service-tracer")
	}

	// Connect to PG in background
	go initDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if tp != nil {
		r.Use(otelgin.Middleware("inventory-service"))
	}

	// Simple in-memory fallback map if DB is down
	memInventory := map[int]int{
		101: 50, 102: 120, 103: 300, 104: 40,
	}

	r.POST("/inventory/reserve", func(c *gin.Context) {
		var req struct {
			ItemID   int `json:"item_id"`
			Quantity int `json:"quantity"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload format"})
			return
		}

		ctx := c.Request.Context()
		_, span := tracer.Start(ctx, "ReserveStock", trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		span.SetAttributes(
			attribute.Int("item.id", req.ItemID),
			attribute.Int("quantity", req.Quantity),
		)

		if !dbConnected {
			// Memory Fallback
			log.Println("DB down, using in-memory inventory fallback")
			span.SetAttributes(attribute.String("datasource", "in-memory-fallback"))
			stock, exists := memInventory[req.ItemID]
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
				return
			}
			if stock < req.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock"})
				return
			}
			memInventory[req.ItemID] -= req.Quantity
			c.JSON(http.StatusOK, gin.H{"status": "reserved", "item_id": req.ItemID, "remaining": memInventory[req.ItemID]})
			return
		}

		span.SetAttributes(attribute.String("datasource", "postgresql"))
		// PostgreSQL Transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			return
		}
		defer tx.Rollback()

		var stock int
		err = tx.QueryRowContext(ctx, "SELECT stock FROM inventory WHERE item_id = $1 FOR UPDATE", req.ItemID).Scan(&stock)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database read query error"})
			return
		}

		if stock < req.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient stock"})
			return
		}

		newStock := stock - req.Quantity
		_, err = tx.ExecContext(ctx, "UPDATE inventory SET stock = $1 WHERE item_id = $2", newStock, req.ItemID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database update query error"})
			return
		}

		err = tx.Commit()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database commit transaction failed"})
			return
		}

		log.Printf("Reserved %d units of Item %d. Remaining stock: %d", req.Quantity, req.ItemID, newStock)
		c.JSON(http.StatusOK, gin.H{"status": "reserved", "item_id": req.ItemID, "remaining": newStock})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "inventory-service", "db_connected": dbConnected})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}
	log.Printf("Inventory-service running on port %s", port)
	r.Run("0.0.0.0:" + port)
}

