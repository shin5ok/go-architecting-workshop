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
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/google/uuid"

	game "github.com/shin5ok/go-architecting-workshop"
)

var (
	appName       = "myapp"
	spannerString = os.Getenv("SPANNER_STRING")
	redisHost     = os.Getenv("REDIS_HOST")
	projectId     = os.Getenv("GOOGLE_CLOUD_PROJECT")
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
	userId, _ := uuid.NewRandom()
	userName := os.Args[1]
	userParams := game.UserParams{UserID: userId.String(), UserName: userName}
	err = s.Client.CreateUser(ctx, os.Stdout, userParams)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(userParams, "is created")

}
