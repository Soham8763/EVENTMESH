package main

import (
	"context"
	"eventmesh/pkg/sdk"
)

func ReserveInventory(ctx context.Context, data []byte) error {
	return nil
}

func ProcessPayment(ctx context.Context, data []byte) error {
	return nil
}

func SendEmail(ctx context.Context, data []byte) error {
	return nil
}

func main() {
	client := sdk.NewClient([]string{"localhost:19092"})
	sdk.SetDefaultClient(client)

	workflow := sdk.NewWorkflow("order-processing").
		Step("reserve_inventory", ReserveInventory).
		Step("process_payment", ProcessPayment).
		Step("send_email", SendEmail)

	ctx := context.Background()

	// Register workflow definition dynamically
	if err := workflow.Register(ctx); err != nil {
		panic(err)
	}

	worker := sdk.NewWorker()
	worker.Register("reserve_inventory", ReserveInventory)
	worker.Register("process_payment", ProcessPayment)
	worker.Register("send_email", SendEmail)

	client.StartWorkflow(ctx, "order-processing", "exec_123")
}
