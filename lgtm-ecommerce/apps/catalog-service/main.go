package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

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
			semconv.ServiceNameKey.String("catalog-service"),
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

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Stock int     `json:"stock"`
}

func main() {
	tp, err := initTracer()
	if err != nil {
		log.Printf("Failed to initialize OTel tracer: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
		tracer = otel.Tracer("catalog-service-tracer")
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	if tp != nil {
		r.Use(otelgin.Middleware("catalog-service"))
	}

	// Products Catalog
	products := []Product{
		{ID: 101, Name: "Telemetry T-Shirt", Price: 25.99, Stock: 50},
		{ID: 102, Name: "Grafana Mug", Price: 12.50, Stock: 120},
		{ID: 103, Name: "Prometheus Stickers Pack", Price: 4.99, Stock: 300},
		{ID: 104, Name: "Jaeger Hooded Sweatshirt", Price: 49.99, Stock: 40},
	}

	r.GET("/catalog", func(c *gin.Context) {
		log.Println("Catalog service serving products request")
		c.JSON(http.StatusOK, products)
	})

	r.GET("/catalog/:id", func(c *gin.Context) {
		idStr := c.Param("id")
		var id int
		_, err := fmt.Sscanf(idStr, "%d", &id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
			return
		}

		for _, p := range products {
			if p.ID == id {
				c.JSON(http.StatusOK, p)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "catalog-service"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8092"
	}
	log.Printf("Catalog-service running on port %s", port)
	r.Run("0.0.0.0:" + port)
}

