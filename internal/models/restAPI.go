package models

type GetSubscriptionURLResponse struct {
	URL string `json:"url"`
}

type ErrResponseRestAPI struct {
	StatusCode int    `json:"status_code"`
	ErrMessage string `json:"err_message"`
}
