/*
This is just for local test with Spanner Emulator
Note: Before running this test, run spanner emulator and create an instance as "test-instance"
*/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis"
	game "github.com/shin5ok/go-architecting-workshop"
	"github.com/shin5ok/go-architecting-workshop/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	fakeDbString = os.Getenv("SPANNER_STRING") + testutil.GenStr()
	fakeServing  Serving

	noCleanup = os.Getenv("NO_CLEANUP") != ""

	itemTestID = "d169f397-ba3f-413b-bc3c-a465576ef06e"
	userTestID string
)

type dummyCaching struct{}

func (c *dummyCaching) Get(key string) (string, error) {
	return "", errors.New("")
}

func (c *dummyCaching) Set(key string, data string) error {
	return nil
}

var _ game.Cacher = (*dummyCaching)(nil)

func init() {

	log.Println("Creating " + fakeDbString)

	if match, _ := regexp.MatchString("^projects/your-project-id/", fakeDbString); match {
		os.Setenv("SPANNER_EMULATOR_HOST", "localhost:9010")
	}

	ctx := context.Background()

	var c game.Cacher

	// no use redis
	c = &dummyCaching{}

	if redisHost != "" {
		rdb := redis.NewClient(&redis.Options{
			Addr:        redisHost,
			Password:    "",
			DB:          0,
			PoolSize:    10,
			PoolTimeout: 30 * time.Second,
			DialTimeout: 1 * time.Second,
		})

		c = &game.Caching{RedisClient: rdb}
	}

	log.Printf("cache %#+v\n", c)

	client, err := game.NewClient(ctx, spannerString, c)
	if err != nil {
		log.Fatal(err)
	}

	fakeServing = Serving{
		Client: client,
	}

	schemaFiles, err := filepath.Glob("schemas/*_ddl.sql")
	if err != nil {
		log.Fatal(err)
	}

	if err := testutil.InitData(ctx, fakeDbString, schemaFiles); err != nil {
		log.Fatal(err)
	}

	dmlFiles, err := filepath.Glob("schemas/*_dml.sql")
	if err != nil {
		log.Fatal(err)
	}

	if err := testutil.MakeData(ctx, fakeDbString, dmlFiles); err != nil {
		log.Fatal(err)
	}
}

func TestRun(t *testing.T) {

	req, err := http.NewRequest("GET", "/", nil)
	assert.Nil(t, err)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.pingPong)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

}

func TestCreateUser(t *testing.T) {

	path := "test-user"
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_name", path)

	r := &http.Request{}
	req, err := http.NewRequestWithContext(r.Context(), "POST", "/api/user/"+path, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.createUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}
	var u User
	json.Unmarshal(rr.Body.Bytes(), &u)
	userTestID = u.Id

}

// This test depends on Test_createUser
func TestAddItemUser(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)
	ctx.URLParams.Add("item_id", itemTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s/%s", userTestID, itemTestID)
	req, err := http.NewRequestWithContext(r.Context(), "PUT", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.addItemToUser)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s", http.StatusOK, rr.Code, rr.Body)
	}

}

func TestGetUserItems(t *testing.T) {

	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("user_id", userTestID)

	r := &http.Request{}
	uriPath := fmt.Sprintf("/api/user_id/%s", userTestID)
	req, err := http.NewRequestWithContext(r.Context(), "GET", uriPath, nil)
	assert.Nil(t, err)
	newReq := req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(fakeServing.getUserItems)
	handler.ServeHTTP(rr, newReq)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected: %d. Got: %d, Message: %s, Request: %+v", http.StatusOK, rr.Code, rr.Body, req)
	}
}

func TestCleaning(t *testing.T) {
	t.Cleanup(
		func() {
			if noCleanup {
				t.Log("###########", "skip cleanup")
				return
			}
			ctx := context.Background()
			if err := testutil.DropData(ctx, fakeDbString); err != nil {
				t.Error(err)
			}
			t.Log("cleanup test data")
		},
	)
}
