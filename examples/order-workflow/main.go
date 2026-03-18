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
	workflow := sdk.NewWorkflow("order-processing")

	workflow.Step("reserve_inventory", ReserveInventory)
	workflow.Step("process_payment", ProcessPayment)
	workflow.Step("send_email", SendEmail)

	worker := sdk.NewWorker()

	worker.Register("reserve_inventory", ReserveInventory)
	worker.Register("process_payment", ProcessPayment)
	worker.Register("send_email", SendEmail)

	client := sdk.NewClient([]string{"localhost:19092"})

	client.StartWorkflow(context.Background(), "order-processing", "exec_123")
}
