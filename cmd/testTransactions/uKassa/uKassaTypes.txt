// юкасса добавлена для примера
// ukassa/types.go
package ukassa

// Request для UKassa
type CreatePaymentRequest struct {
	Amount struct {
		Value    string `json:"value"`
		Currency string `json:"currency"`
	} `json:"amount"`
	Capture         bool                   `json:"capture"`
	Description     string                 `json:"description"`
	Metadata        map[string]string      `json:"metadata,omitempty"`
	Confirmation    map[string]interface{} `json:"confirmation"`
	Receipt         map[string]interface{} `json:"receipt,omitempty"`
	PaymentMethodID string                 `json:"payment_method_id,omitempty"`
}

// Response от UKassa
type CreatePaymentResponse struct {
	ID            string                 `json:"id"`
	Status        string                 `json:"status"`
	Paid          bool                   `json:"paid"`
	Amount        map[string]interface{} `json:"amount"`
	Description   string                 `json:"description"`
	Metadata      map[string]string      `json:"metadata"`
	CreatedAt     string                 `json:"created_at"`
	ExpiresAt     string                 `json:"expires_at"`
	Confirmation  map[string]interface{} `json:"confirmation"`
	Test          bool                   `json:"test"`
	Refundable    bool                   `json:"refundable"`
	PaymentMethod map[string]interface{} `json:"payment_method"`
}
