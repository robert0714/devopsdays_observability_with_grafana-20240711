package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.corp.ikala.tv/ai-architect/ikit/interface/logger"
	kitLoggerProvider "gitlab.corp.ikala.tv/ai-architect/ikit/provider/zap_logger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.17.0"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// bad practice
var l logger.Logger

func init() {
	l = NewLogger()
}

func main() {
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint())
	if err != nil {
		l.FatalK("failed to initialize opentelemetry exporter: %v", err)
	}
	// Create the Tracer provider
	l.DebugF("Initialized OpenTelemetry Tracer")
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(os.Getenv("SERVICE_NAME")),
		)),
	)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C Trace Context
		propagation.Baggage{},      // W3C Baggage
	))

	defer func() { _ = tp.Shutdown(context.Background()) }()

	otel.SetTracerProvider(tp)

	l.DebugF("Initialized Prometheus metrics")
	foodCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ikala_lunch_decision_count",
			Help: "The number of lunch decisions made",
		},
		[]string{"food", "suggester", "decision"}, // Labels
	)
	prometheus.MustRegister(foodCounter)

	router := gin.Default()
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"POST", "GET", "PUT", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "Accept", "User-Agent", "Cache-Control", "Pragma"}
	config.ExposeHeaders = []string{"Content-Length"}

	router.Use(otelgin.Middleware("demo"))
	router.Use(gin.Recovery())
	router.Use(cors.New(config))
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.POST("/lunch-suggestion/:food", makeLunchDecision(foodCounter))
	router.GET("/lunch-decision/:food", personOpinion)

	l.DebugF("Service started")
	router.Run()
}

func makeLunchDecision(foodCounter *prometheus.CounterVec) gin.HandlerFunc {
	return func(context *gin.Context) {
		requestCtx := context.Request.Context()
		considerUnexpectedIssue := context.DefaultQuery("unexpected_issue", "false")

		UnexpectedIssueRate := 0.0
		if considerUnexpectedIssue == "true" {
			l.DebugF("Consider unexpected issue")
			UnexpectedIssueRate = 0.1
		}

		randomEvent := randomTrueFalse(UnexpectedIssueRate)
		if randomEvent == "true" {
			l.WithContext(requestCtx).ErrorF("The elevator in the building is out of service. We can't go out for lunch.")
			context.Error(fmt.Errorf("the elevator in the building is out of service. We can't go out for lunch"))
			context.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Unexpected issue occurred. Please try again later."},
			)
			return
		}

		httpClient := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
		l.DebugF("How about having some %s?", context.Param("food"))

		agreeCount, disagreeCount := 1, 0 // suggester always agrees to have the food that he/she suggests
		colleagues := strings.Split(os.Getenv("PERSON_LIST"), ",")

		for _, person := range colleagues {
			voteURL := fmt.Sprintf("http://%s:8080/lunch-decision/%s", person, context.Param("food"))
			voteResult, err := fetchOpinion(requestCtx, &httpClient, voteURL)
			if err != nil {
				context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}

			if voteResult == "true" {
				l.WithContext(requestCtx).InfoF("Person %s agrees to have %s", person, context.Param("food"))
				agreeCount++
			} else {
				l.WithContext(requestCtx).WarnF("Person %s disagrees to have %s", person, context.Param("food"))
				disagreeCount++
			}

		}
		l.InfoF("Agree count: %d, Disagree count: %d", agreeCount, disagreeCount)

		if agreeCount > disagreeCount {
			foodCounter.WithLabelValues(context.Param("food"), os.Getenv("SERVICE_NAME"), "agree").Inc()
			l.WithContext(requestCtx).InfoF("Majority vote: Yes. Let's go eat %s!", context.Param("food"))
			context.String(http.StatusOK, "Majority vote: Yes. Let's go have some %s!", context.Param("food"))
		} else if agreeCount < disagreeCount {
			foodCounter.WithLabelValues(context.Param("food"), os.Getenv("SERVICE_NAME"), "reject").Inc()
			l.WithContext(requestCtx).ErrorF("Majority vote: No. We decide not to have %s.", context.Param("food"))
			context.String(http.StatusOK, "Majority vote: No.")
		} else {
			foodCounter.WithLabelValues(context.Param("food"), os.Getenv("SERVICE_NAME"), "tie").Inc()
			l.WithContext(requestCtx).WarnF("Tie vote. We can't decide if we should have %s.", context.Param("food"))
			context.String(http.StatusOK, "Tie vote. We can't decide if we should have %s.", context.Param("food"))
		}
	}
}

func fetchOpinion(ctx context.Context, client *http.Client, url string) (string, error) {
	// call person service
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response struct {
		Opinion string `json:"opinion"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "false", err
	}
	return response.Opinion, nil
}

func personOpinion(context *gin.Context) {
	// fake opinion service
	context.JSON(http.StatusOK, gin.H{
		"person":  os.Getenv("SERVICE_NAME"),
		"opinion": randomOpinion(context),
	})
}

func randomOpinion(context *gin.Context) string {
	// fake random opinion between true and false
	food := context.Param("food")
	requestCtx := context.Request.Context()
	rand.Seed(time.Now().UnixNano())
	mockConsideration()
	considerResult := randomTrueFalse(0.5)

	// legacy code just for demo
	if food == "mos_burger" {
		considerResult = "true"
	}
	if food == "kfc" {
		considerResult = "false"
	}

	if considerResult == "true" {
		l.WithContext(requestCtx).InfoF("Agree to have %s", food)
	} else {
		l.WithContext(requestCtx).InfoF("Disagree to have %s", food)
	}
	return considerResult
}

func mockConsideration() {
	//fake consideration time
	considerTime := os.Getenv("CONSIDERATION_TIME_MILLISECONDS")
	milliseconds, err := strconv.Atoi(considerTime)
	if err != nil {
		l.FatalF("Error converting CONSIDERATION_TIME_MILLISECONDS to integer: %v", err)
	}
	duration := time.Duration(rand.Intn(milliseconds)) * time.Millisecond
	l.InfoF("Consideration time: %v", duration)
	time.Sleep(duration)
}

func randomTrueFalse(trueRate float64) string {
	rand.Seed(time.Now().UnixNano())
	if rand.Float64() < trueRate {
		return "true"
	}
	return "false"
}

func NewLogger() logger.Logger {
	return kitLoggerProvider.NewZapLogger(kitLoggerProvider.Config{
		LogLevel: "debug",
	})
}
