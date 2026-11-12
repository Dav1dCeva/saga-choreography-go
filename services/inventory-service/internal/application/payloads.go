package application

type PaymentCompletedPayload struct {
	OrderID  string  `json:"order_id"`
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
	Address  string  `json:"address"`
}

type InventoryReservedPayload struct {
	OrderID  string `json:"order_id"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
	Address  string `json:"address"`
}

type InventoryFailedPayload struct {
	OrderID string `json:"order_id"`
	Product string `json:"product"`
	Reason  string `json:"reason"`
}

type InventoryReleasedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type ShippingFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
