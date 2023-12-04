package game

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"

	//game "github.com/shin5ok/go-architecting-workshop"
	"github.com/shin5ok/go-architecting-workshop/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	fakeDbString = os.Getenv("SPANNER_STRING") + testutil.GenStr()

	noCleanup = os.Getenv("NO_CLEANUP") != ""

	itemTestID = "d169f397-ba3f-413b-bc3c-a465576ef06e"
	userTestID string

	testDbClient dbClient
)

type Serving struct {
	Client dbClient
}

type dummyCaching struct{}

func (c *dummyCaching) Get(key string) (string, error) {
	return "", nil
}

func (c *dummyCaching) Set(key string, data string) error {
	return nil
}

func init() {

	fmt.Println(noCleanup)
	log.Println("Creating " + fakeDbString)

	if match, _ := regexp.MatchString("^projects/your-project-id/", fakeDbString); match {
		os.Setenv("SPANNER_EMULATOR_HOST", "localhost:9010")
	}

	ctx := context.Background()

	var c Cacher

	// no use redis
	c = &dummyCaching{}

	rdb := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:6379",
		Password:    "",
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
		DialTimeout: 1 * time.Second,
	})

	c = &Caching{RedisClient: rdb}

	fmt.Printf("cache %#+v\n", c)

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

func TestNewClient(t *testing.T) {
	ctx := context.Background()

	client, err := NewClient(ctx, fakeDbString, nil)
	assert.IsType(t, client, dbClient{})

	testDbClient = client

	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
}

func TestCreateUser(t *testing.T) {
	userId, _ := uuid.NewUUID()

	err := testDbClient.CreateUser(
		context.Background(),
		io.Discard,
		UserParams{
			UserID:   userId.String(),
			UserName: "test",
		},
	)

	if err != nil {
		t.Error(err)
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
