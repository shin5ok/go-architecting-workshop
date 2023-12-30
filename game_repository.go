package game

import (
	"context"
	"io"
)

type GameUserOperation interface {
	CreateUser(context.Context, io.Writer, UserParams) error
	AddItemToUser(context.Context, io.Writer, UserParams, ItemParams) error
	UserItems(context.Context, io.Writer, string) ([]map[string]interface{}, error)
}

type Cacher interface {
	Get(string) (string, error)
	Set(string, string) error
}
