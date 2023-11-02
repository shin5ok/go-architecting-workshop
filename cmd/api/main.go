/*
Copyright 2023 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/profiler"
	"cloud.google.com/go/pubsub"
	chiprometheus "github.com/766b/chi-prometheus"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/go-chi/render"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	game "github.com/shin5ok/go-architecting-workshop"
	internal "github.com/shin5ok/go-architecting-workshop/cmd/api/internal"
)

var (
	appName    = "myapp"
	appVersion = "1.01"

	spannerString = os.Getenv("SPANNER_STRING")
	redisHost     = os.Getenv("REDIS_HOST")
	servicePort   = os.Getenv("PORT")
	projectId     = os.Getenv("GOOGLE_CLOUD_PROJECT")
	rev           = os.Getenv("K_REVISION")
	logger        *slog.Logger
)

var (
	topicName      = os.Getenv("TOPIC_NAME")
	authHeaderName = os.Getenv("AUTH_HEADER")
	pubsubClient   *pubsub.Client
)

type Serving struct {
	Client game.GameUserOperation
}

type User struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

func init() {
	replace := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.LevelKey && a.Value.String() == slog.LevelWarn.String() {
			return slog.String("severity", "WARNING")
		}
		if a.Key == "level" {
			return slog.String("severity", a.Value.String())
		}
		if a.Key == "msg" {
			return slog.String("message", a.Value.String())
		}
		return a
	}

	options := slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true, ReplaceAttr: replace,
	}

	logger = slog.New(slog.NewJSONHandler(os.Stdout, &options))
	slog.SetDefault(logger)

}

func main() {

	ctx := context.Background()
	tp, err := internal.NewTracer(projectId)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	defer tp.Shutdown(ctx)

	profilerCfg := profiler.Config{
		Service:           appName,
		ServiceVersion:    appVersion,
		ProjectID:         projectId,
		EnableOCTelemetry: true,
	}

	if err := profiler.Start(profilerCfg); err != nil {
		logger.Error(err.Error())
		return
	}

	pubsubClient, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		logger.Error(err.Error())
		return
	}
	defer pubsubClient.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:        redisHost,
		Password:    "",
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
		DialTimeout: 1 * time.Second,
	})

	c := game.Caching{RedisClient: rdb}

	client, err := game.NewClient(ctx, spannerString, &c)
	if err != nil {
		logger.Error(err.Error())
		return
	}

	defer client.Sc.Close()
	defer rdb.Close()

	s := Serving{
		Client: client,
	}

	oplog := httplog.LogEntry(context.Background())
	/* jsonify logging */
	httpLogger := httplog.NewLogger(appName, httplog.Options{JSON: true, LevelFieldName: "severity", Concise: true})

	/* exporter for prometheus */
	m := chiprometheus.NewMiddleware(appName)

	r := chi.NewRouter()
	// r.Use(middleware.Throttle(8))
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(httplog.RequestLogger(httpLogger))
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(headerAuth)

	r.Use(m)
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/ping", s.pingPong)

	r.Route("/api", func(t chi.Router) {
		t.Get("/user_id/{user_id:[a-z0-9-.]+}", s.getUserItems)
		t.Post("/user/{user_name:[a-z0-9-.]+}", s.createUser)
		t.Put("/user_id/{user_id:[a-z0-9-.]+}/{item_id:[a-z0-9-.]+}", s.addItemToUser)
	})

	if err := http.ListenAndServe(":"+servicePort, r); err != nil {
		oplog.Err(err)
	}

}

var errorRender = func(w http.ResponseWriter, r *http.Request, httpCode int, err error) {
	render.Status(r, httpCode)
	render.JSON(w, r, map[string]interface{}{"ERROR": err.Error()})
}

func (s Serving) getUserItems(w http.ResponseWriter, r *http.Request) {

	userID := chi.URLParam(r, "user_id")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "getUserItems.root")
	span.SetAttributes(attribute.String("server", "getUserItems"))
	defer span.End()

	/* sample log related to span id */
	traceWithLog(ctx, span).Str("method", "ok").Send()

	results, err := s.Client.UserItems(ctx, w, userID)
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}

	// publish log, just for test
	if topicName != "" {
		p := map[string]interface{}{"id": userID, "rev": rev}
		internal.PublishLog(pubsubClient, topicName, p)
	}

	render.JSON(w, r, results)
}

func (s Serving) createUser(w http.ResponseWriter, r *http.Request) {
	userId, _ := uuid.NewRandom()
	userName := chi.URLParam(r, "user_name")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "createUser.root")
	span.SetAttributes(attribute.String("server", "createUser"))
	defer span.End()

	err := s.Client.CreateUser(ctx, w, game.UserParams{UserID: userId.String(), UserName: userName})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, User{
		Id:   userId.String(),
		Name: userName,
	})
}

func (s Serving) addItemToUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	itemID := chi.URLParam(r, "item_id")
	ctx := r.Context()

	ctx, span := otel.Tracer("main").Start(ctx, "addItemToUser.root")
	span.SetAttributes(attribute.String("server", "addItemToUser"))
	defer span.End()

	err := s.Client.AddItemToUser(ctx, w, game.UserParams{UserID: userID}, game.ItemParams{ItemID: itemID})
	if err != nil {
		errorRender(w, r, http.StatusInternalServerError, err)
		return
	}
	render.JSON(w, r, map[string]string{})
}

func (s Serving) pingPong(w http.ResponseWriter, r *http.Request) {
	render.Status(r, http.StatusOK)
	render.PlainText(w, r, "Pong\n")
}

func headerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authHeaderName != "" {
			auth := r.Header.Get(authHeaderName)
			if auth == "" {
				// w.WriteHeader(http.StatusForbidden)
				log.Printf("Forbidden request info: %+v", r)
				http.Error(w, "You're NOT permitted to enter here", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func traceWithLog(ctx context.Context, span trace.Span) *zerolog.Event {
	trace := fmt.Sprintf("projects/%s/traces/%s", projectId, span.SpanContext().TraceID().String())
	oplog := httplog.LogEntry(ctx)
	return oplog.Info().
		Str("logging.googleapis.com/trace", trace).
		Str("logging.googleapis.com/spanId", span.SpanContext().SpanID().String())
}
