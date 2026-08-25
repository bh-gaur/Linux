package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracer      trace.Tracer
	db          *sql.DB
	dbConnected = false
	kafkaWriter *kafka.Writer
)

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
			semconv.ServiceNameKey.String("order-service"),
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

	for i := 0; i < 5; i++ {
		var err error
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Println("Order service successfully connected to PostgreSQL")
				dbConnected = true
				break
			}
		}
		log.Printf("PostgreSQL not ready yet, retrying in 3s (%d/5): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if dbConnected {
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS orders (
				order_id VARCHAR(50) PRIMARY KEY,
				user_id VARCHAR(50) NOT NULL,
				item_id INT NOT NULL,
				quantity INT NOT NULL,
				total_price NUMERIC(10, 2) NOT NULL,
				status VARCHAR(20) NOT NULL,
				shipment_label VARCHAR(100),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			);
		`)
		if err != nil {
			log.Printf("Failed to bootstrap orders table: %v", err)
		}
	}
}

func initKafka() {
	kafkaBrokers := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokers == "" {
		kafkaBrokers = "kafka:9092"
	}
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaBrokers),
		Topic:    "order-events",
		Balancer: &kafka.LeastBytes{},
	}
	log.Printf("Kafka producer initialized with broker: %s", kafkaBrokers)
}

// Client HTTP helper with injected OTel propagation
func callService(ctx context.Context, method, url string, payload interface{}) (int, []byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Inject context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, err
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Printf("Failed to initialize OTel tracer: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
		tracer = otel.Tracer("order-service-tracer")
	}

	go initDB()
	initKafka()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if tp != nil {
		r.Use(otelgin.Middleware("order-service"))
	}

	catalogURL := os.Getenv("CATALOG_SERVICE_URL")
	if catalogURL == "" {
		catalogURL = "http://catalog-service:8092"
	}
	inventoryURL := os.Getenv("INVENTORY_SERVICE_URL")
	if inventoryURL == "" {
		inventoryURL = "http://inventory-service:8095"
	}
	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://payment-service:8096"
	}
	shippingURL := os.Getenv("SHIPPING_SERVICE_URL")
	if shippingURL == "" {
		shippingURL = "http://shipping-service:8097"
	}

	r.POST("/orders", func(c *gin.Context) {
		var orderReq struct {
			UserID   string `json:"user_id"`
			ItemID   int    `json:"item_id"`
			Quantity int    `json:"quantity"`
		}
		if err := c.ShouldBindJSON(&orderReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order input payload"})
			return
		}

		ctx := c.Request.Context()
		orderID := fmt.Sprintf("ord_%d", time.Now().UnixNano())
		log.Printf("Starting order placement sequence: %s for user %s", orderID, orderReq.UserID)

		// 1. Fetch Item Catalog info to verify price and existence
		catStatus, catBody, err := callService(ctx, "GET", fmt.Sprintf("%s/catalog/%d", catalogURL, orderReq.ItemID), nil)
		if err != nil || catStatus != 200 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Item catalog check failed or item not found"})
			return
		}
		var item struct {
			Price float64 `json:"price"`
			Name  string  `json:"name"`
		}
		json.Unmarshal(catBody, &item)
		totalPrice := item.Price * float64(orderReq.Quantity)

		// 2. Reserve stock in Inventory-Service
		reservePayload := map[string]interface{}{"item_id": orderReq.ItemID, "quantity": orderReq.Quantity}
		invStatus, _, err := callService(ctx, "POST", fmt.Sprintf("%s/inventory/reserve", inventoryURL), reservePayload)
		if err != nil || invStatus != 200 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Stock reservation failed (out of stock)"})
			return
		}

		// 3. Charge payment in Payment-Service
		payPayload := map[string]interface{}{"user_id": orderReq.UserID, "amount": totalPrice, "order_id": orderID}
		payStatus, payBody, err := callService(ctx, "POST", fmt.Sprintf("%s/charge", paymentURL), payPayload)
		if err != nil || payStatus != 200 {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "Credit transaction failed"})
			return
		}
		var payResp struct {
			TransactionID string `json:"transaction_id"`
		}
		json.Unmarshal(payBody, &payResp)

		// 4. Request shipment tracking label from Shipping-Service
		shipPayload := map[string]interface{}{"order_id": orderID, "user_id": orderReq.UserID}
		shipStatus, shipBody, err := callService(ctx, "POST", fmt.Sprintf("%s/ship", shippingURL), shipPayload)
		var label string
		if err == nil && shipStatus == 200 {
			var shipResp struct {
				TrackingLabel string `json:"tracking_label"`
			}
			json.Unmarshal(shipBody, &shipResp)
			label = shipResp.TrackingLabel
		} else {
			label = "PENDING_SHIPPING_SERVICE_DOWN"
		}

		// 5. Store order in PostgreSQL database
		status := "completed"
		if dbConnected {
			_, err = db.ExecContext(ctx, `
				INSERT INTO orders (order_id, user_id, item_id, quantity, total_price, status, shipment_label)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, orderID, orderReq.UserID, orderReq.ItemID, orderReq.Quantity, totalPrice, status, label)
			if err != nil {
				log.Printf("Failed to record order inside database: %v", err)
			}
		}

		// 6. Asynchronously announce order completion to Kafka topic
		event := map[string]interface{}{
			"order_id":       orderID,
			"user_id":        orderReq.UserID,
			"item_id":        orderReq.ItemID,
			"quantity":       orderReq.Quantity,
			"total":          totalPrice,
			"shipment_label": label,
			"timestamp":      time.Now().Format(time.RFC3339),
		}
		eventBytes, _ := json.Marshal(event)

		// Extract W3C propagator for Kafka Headers
		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		traceparent := carrier["traceparent"]

		kafkaMsg := kafka.Message{
			Key:   []byte(orderID),
			Value: eventBytes,
			Headers: []kafka.Header{
				{Key: "traceparent", Value: []byte(traceparent)},
			},
		}

		go func() {
			err = kafkaWriter.WriteMessages(context.Background(), kafkaMsg)
			if err != nil {
				log.Printf("Failed to dispatch completion event to Kafka topic: %v", err)
			} else {
				log.Printf("Dispatched complete event to Kafka topic for order: %s", orderID)
			}
		}()

		c.JSON(http.StatusOK, gin.H{
			"status":         status,
			"order_id":       orderID,
			"total":          totalPrice,
			"item_name":      item.Name,
			"tracking_label": label,
		})
	})

	r.GET("/orders", func(c *gin.Context) {
		if !dbConnected {
			c.JSON(http.StatusOK, []interface{}{})
			return
		}
		rows, err := db.QueryContext(c.Request.Context(), "SELECT order_id, user_id, item_id, quantity, total_price, status, shipment_label FROM orders ORDER BY created_at DESC LIMIT 50")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var list []map[string]interface{}
		for rows.Next() {
			var ordID, usrID, status, label string
			var itemID, qty int
			var price float64
			rows.Scan(&ordID, &usrID, &itemID, &qty, &price, &status, &label)
			list = append(list, map[string]interface{}{
				"order_id":       ordID,
				"user_id":        usrID,
				"item_id":        itemID,
				"quantity":       qty,
				"total":          price,
				"status":         status,
				"shipment_label": label,
			})
		}
		c.JSON(http.StatusOK, list)
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "order-service", "db_connected": dbConnected})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8094"
	}
	log.Printf("Order-service running on port %s", port)
	r.Run("0.0.0.0:" + port)
}

