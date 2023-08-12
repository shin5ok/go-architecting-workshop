/*
Work in progress
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"

	game "github.com/shin5ok/go-architecting-workshop"
)

var (
	spannerString = os.Getenv("SPANNER_STRING")
	redisHost     = os.Getenv("REDIS_HOST")
)

type Serving struct {
	Client game.GameUserOperation
}

type User struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

func main() {

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:        redisHost,
		Password:    "",
		DB:          0,
		PoolSize:    10,
		PoolTimeout: 30 * time.Second,
		DialTimeout: 1 * time.Second,
	})

	client, err := game.NewClient(ctx, spannerString, rdb)
	if err != nil {
		log.Fatal(err)
	}

	defer client.Sc.Close()
	defer rdb.Close()

	s := Serving{
		Client: client,
	}

	/* Just to test */
	userId, err := uuid.NewRandom()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) != 2 {
		log.Fatal("error: invalid number of args")
	}
	userName := os.Args[1]
	userParams := game.UserParams{UserID: userId.String(), UserName: userName}
	err = s.Client.CreateUser(ctx, os.Stdout, userParams)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(userParams, "is created")

}
