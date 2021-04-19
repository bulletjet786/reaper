package go_source

import (
	"context"
	"time"
)

func Test() {
	ctx := context.Background()
	context.WithTimeout(ctx, 5*time.Second)
}

func main() {

}
