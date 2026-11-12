package application

type InventoryReservedPayload struct {
	OrderID  string `json:"order_id"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
	Address  string `json:"address"`
}

type ShippingScheduledPayload struct {
	OrderID  string `json:"order_id"`
	Tracking string `json:"tracking"`
	Address  string `json:"address"`
}

type ShippingFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
