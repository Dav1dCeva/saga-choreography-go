package application

// Los payloads son contratos JSON entre servicios. Se mantienen explícitos
// para que cada servicio pueda evolucionar su propio modelo.

type OrderCreatedPayload struct {
	OrderID   string  `json:"order_id"`
	Customer  string  `json:"customer"`
	Product   string  `json:"product"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Amount    float64 `json:"amount"`
	Address   string  `json:"address"`
}

type PaymentCompletedPayload struct {
	OrderID  string  `json:"order_id"`
	Product  string  `json:"product"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
	Address  string  `json:"address"`
}

type PaymentFailedPayload struct {
	OrderID string  `json:"order_id"`
	Reason  string  `json:"reason"`
	Amount  float64 `json:"amount"`
}

type PaymentRefundedPayload struct {
	OrderID string  `json:"order_id"`
	Reason  string  `json:"reason"`
	Amount  float64 `json:"amount"`
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

type ShippingScheduledPayload struct {
	OrderID  string `json:"order_id"`
	Tracking string `json:"tracking"`
	Address  string `json:"address"`
}

type ShippingFailedPayload struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}
