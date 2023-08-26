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
package gameexample

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"encoding/json"

	"cloud.google.com/go/spanner"
	"github.com/go-redis/redis"
	"go.opentelemetry.io/otel"
	"google.golang.org/api/iterator"
)

type GameUserOperation interface {
	CreateUser(context.Context, io.Writer, UserParams) error
	AddItemToUser(context.Context, io.Writer, UserParams, ItemParams) error
	UserItems(context.Context, io.Writer, string) ([]map[string]interface{}, error)
}

type UserParams struct {
	UserID   string
	UserName string
}

type ItemParams struct {
	ItemID string
}

type dbClient struct {
	Sc    *spanner.Client
	Cache *redis.Client
}

var baseItemSliceCap = 100

func NewClient(ctx context.Context, dbString string, redisClient *redis.Client) (dbClient, error) {

	client, err := spanner.NewClient(ctx, dbString)
	if err != nil {
		return dbClient{}, err
	}

	return dbClient{
		Sc:    client,
		Cache: redisClient,
	}, nil
}

// create a user
func (d dbClient) CreateUser(ctx context.Context, w io.Writer, u UserParams) error {

	ctx, span := otel.Tracer("main").Start(ctx, "CreateUser")
	defer span.End()

	_, err := d.Sc.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {
		sqlToUsers := `INSERT users (user_id, name, created_at, updated_at)
		  VALUES (@userID, @userName, @timestamp, @timestamp)`
		t := time.Now().Format("2006-01-02 15:04:05")
		params := map[string]interface{}{
			"userID":    u.UserID,
			"userName":  u.UserName,
			"timestamp": t,
		}
		stmtToUsers := spanner.Statement{
			SQL:    sqlToUsers,
			Params: params,
		}
		rowCountToUsers, err := txn.UpdateWithOptions(ctx, stmtToUsers, spanner.QueryOptions{RequestTag: "func=CreateUser,env=dev,action=insert"})
		_ = rowCountToUsers
		if err != nil {
			return err
		}

		return nil
	}, spanner.TransactionOptions{TransactionTag: "func=CreateUser,env=dev"})

	return err
}

/*
add item specified item_id to specific user
additionally show example how to use span of trace
*/
func (d dbClient) AddItemToUser(ctx context.Context, w io.Writer, u UserParams, i ItemParams) error {

	ctx, span := otel.Tracer("main").Start(ctx, "AddItemUser")
	defer span.End()

	_, err := d.Sc.ReadWriteTransactionWithOptions(ctx, func(ctx context.Context, txn *spanner.ReadWriteTransaction) error {

		sqlToUsers := `INSERT user_items (user_id, item_id, created_at, updated_at)
		  VALUES (@userID, @itemID, @timestamp, @timestamp)`
		t := time.Now().Format("2006-01-02 15:04:05")
		params := map[string]interface{}{
			"userID":    u.UserID,
			"itemId":    i.ItemID,
			"timestamp": t,
		}
		stmtToUsers := spanner.Statement{
			SQL:    sqlToUsers,
			Params: params,
		}
		rowCountToUsers, err := txn.Update(ctx, stmtToUsers)
		_ = rowCountToUsers
		if err != nil {
			return err
		}
		return nil
	}, spanner.TransactionOptions{TransactionTag: "func=AddItemToUser,env=dev"})

	return err
}

// get items the user has
func (d dbClient) UserItems(ctx context.Context, w io.Writer, userID string) ([]map[string]interface{}, error) {

	ctx, span := otel.Tracer("main").Start(ctx, "GetCache")
	key := fmt.Sprintf("UserItems_%s", userID)
	data, err := d.Cache.Get(key).Result()
	span.End()

	if err != nil {
		log.Println(key, "Error", err)
	} else {
		_, span := otel.Tracer("main").Start(ctx, "JsonUnmarshal")
		results := []map[string]interface{}{}
		err := json.Unmarshal([]byte(data), &results)
		if err != nil {
			log.Println(err)
		}
		span.End()
		log.Println(key, "from cache")
		return results, nil
	}

	txn := d.Sc.ReadOnlyTransaction()
	defer txn.Close()
	sql := `select users.name,items.item_name,user_items.item_id
		from user_items join items on items.item_id = user_items.item_id join users on users.user_id = user_items.user_id
		where user_items.user_id = @user_id`
	stmt := spanner.Statement{
		SQL: sql,
		Params: map[string]interface{}{
			"user_id": userID,
		},
	}

	ctx, span = otel.Tracer("main").Start(ctx, "txnQuery")
	iter := txn.QueryWithOptions(ctx, stmt, spanner.QueryOptions{RequestTag: "func=UserItems,env=dev,action=query"})
	defer iter.Stop()
	span.End()

	_, span = otel.Tracer("main").Start(ctx, "readResults")
	results := make([]map[string]interface{}, 0, baseItemSliceCap)
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return results, err
		}
		var userName string
		var itemNames string
		var itemIds string
		if err := row.Columns(&userName, &itemNames, &itemIds); err != nil {
			return results, err
		}

		results = append(results,
			map[string]interface{}{
				"user_name": userName,
				"item_name": itemNames,
				"item_id":   itemIds,
			})

	}
	span.End()

	_, span = otel.Tracer("main").Start(ctx, "setResults")
	jsonedResults, _ := json.Marshal(results)
	err = d.Cache.Set(key, string(jsonedResults), 10*time.Second).Err()
	if err != nil {
		log.Println(err)
	}
	span.End()

	return results, nil
}
