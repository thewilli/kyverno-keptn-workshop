package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	flagd "github.com/open-feature/go-sdk-contrib/providers/flagd/pkg"
	"github.com/open-feature/go-sdk/openfeature"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	serviceName  = os.Getenv("SERVICE_NAME")
	collectorURL = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
)

func initTracer() func(context.Context) error {

	secureOption := otlptracegrpc.WithInsecure()

	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			secureOption,
			otlptracegrpc.WithEndpoint(collectorURL),
		),
	)

	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}
	resources, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		log.Fatalf("Could not set resources: %v", err)
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(resources),
		),
	)
	return exporter.Shutdown
}

func main() {
	cleanup := initTracer()
	defer cleanup(context.Background())

	r := gin.Default()
	r.Use(otelgin.Middleware(serviceName))

	r.GET("/fast", fastHandler)
	r.GET("/slow", slowHandler)
	r.GET("/healthz", healthzHandler)
	r.GET("/", rootHandler)

	if err := r.Run(":8080"); err != nil {
		panic(err)
	}
}

func healthzHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func fastHandler(c *gin.Context) {
	//ctx, span := otel.GetTracerProvider().Tracer("go-web-service").Start(c.Request.Context(), "fastHandler")
	//defer span.End()

	c.JSON(http.StatusOK, gin.H{"message": "This is a fast response"})
}

func slowHandler(c *gin.Context) {
	//ctx, span := otel.GetTracerProvider().Tracer("go-web-service").Start(c.Request.Context(), "slowHandler")
	//defer span.End()
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // Simulate slow operation

	c.JSON(http.StatusOK, gin.H{"message": "This is a slow response"})
}

func rootHandler(c *gin.Context) {
	err := openfeature.SetProviderAndWait(flagd.NewProvider())
	if err != nil {
		// If a provider initialization error occurs, log it and exit
		log.Fatalf("Failed to set the OpenFeature provider: %v", err)
	}
	// Create a new client
	client := openfeature.NewClient("app")

	v2Enabled, _ := client.BooleanValue(
		context.Background(), "v2_enabled", true, openfeature.EvaluationContext{},
	)

	if v2Enabled {
		c.HTML(http.StatusOK, "index.html", nil)
	} else {
		c.JSON(http.StatusGatewayTimeout, gin.H{"message": "Something went wrong. Please try again later."})
	}
}