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
	"github.com/urfave/cli/v2"

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

	app := cli.NewApp()

	createUserFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "username",
			Value: "",
			Usage: "Give it a user name to be created",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "createuser",
			Flags: createUserFlags,
			Action: func(c *cli.Context) error {
				userId, err := uuid.NewRandom()
				if err != nil {
					return err
				}
				userName := c.String("username")
				userParams := game.UserParams{UserID: userId.String(), UserName: userName}
				err = s.Client.CreateUser(ctx, os.Stdout, userParams)
				if err != nil {
					return err
				}
				fmt.Printf("%+v is created\n", userParams)

				return nil

			},
		},
	}

	app.Name = "game-api"
	/* TODO */
	app.Usage = `
	Usage: game-api
	`
	app.Run(os.Args)

}
