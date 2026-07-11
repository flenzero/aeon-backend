package redisx_test

import (
	"context"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/redisx"
)

func TestMemoryClientJSONAndSet(t *testing.T) {
	c := redisx.NewMemoryClient()
	ctx := context.Background()
	type row struct {
		ID int `json:"id"`
	}
	if err := c.SetJSON(ctx, "k", row{ID: 7}, time.Minute); err != nil {
		t.Fatal(err)
	}
	var got row
	if err := c.GetJSON(ctx, "k", &got); err != nil || got.ID != 7 {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if err := c.SAdd(ctx, "s", "a", "b"); err != nil {
		t.Fatal(err)
	}
	members, err := c.SMembers(ctx, "s")
	if err != nil || len(members) != 2 {
		t.Fatalf("members=%v err=%v", members, err)
	}
	_ = c.SRem(ctx, "s", "a")
	members, _ = c.SMembers(ctx, "s")
	if len(members) != 1 || members[0] != "b" {
		t.Fatalf("members=%v", members)
	}
}
