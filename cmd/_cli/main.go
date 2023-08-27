package main

import (
	"context"
	"encoding/json"
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

	userItemFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "userid",
			Value: "",
			Usage: "Give it a user id to be targeted",
		},
	}

	addItemFlags := []cli.Flag{
		&cli.StringFlag{
			Name:  "userid",
			Value: "",
			Usage: "Give it a user id to be targeted",
		},
		&cli.StringFlag{
			Name:  "itemid",
			Value: "",
			Usage: "item id to be added to the user id",
		},
	}

	app.Commands = []*cli.Command{
		{
			Name:  "createuser",
			Flags: createUserFlags,

			Action: func(c *cli.Context) error {

				userId, err := uuid.NewRandom()
				if err != nil {
					fmt.Println(err)
					return err
				}

				userName := c.String("username")
				userParams := game.UserParams{UserID: userId.String(), UserName: userName}
				err = s.Client.CreateUser(ctx, os.Stdout, userParams)
				if err != nil {
					fmt.Println(err)
					return err
				}

				fmt.Printf("%+v is created\n", userParams)
				return nil

			},
		},
		{
			Name:  "additem",
			Flags: addItemFlags,

			Action: func(c *cli.Context) error {
				userID := c.String("userid")
				itemID := c.String("itemid")

				ctx := context.Background()

				err := s.Client.AddItemToUser(ctx, os.Stdout, game.UserParams{UserID: userID}, game.ItemParams{ItemID: itemID})
				if err != nil {
					fmt.Println(err)
					return err
				}

				fmt.Printf("%s is added to %s\n", itemID, userID)
				return nil
			},
		},
		{
			Name:  "useritems",
			Flags: userItemFlags,

			Action: func(c *cli.Context) error {
				userID := c.String("userid")

				ctx := context.Background()

				results, err := s.Client.UserItems(ctx, os.Stdout, userID)
				if err != nil {
					fmt.Println(err)
					return err
				}

				/* TODO
				handle results as a stream interface, such as io.Reader.
				To do it, we should change definition of the interface.
				*/
				data, err := json.Marshal(results)
				if err != nil {
					fmt.Println(err)
					return err
				}

				fmt.Println(string(data))
				return nil
			},
		},
	}

	app.Name = "game-api"
	app.Usage = `Game operation CLI via REST API`
	app.Run(os.Args)

}
