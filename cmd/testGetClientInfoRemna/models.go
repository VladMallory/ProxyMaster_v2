package main

// структура для запроса в апи remanwave
type BulkExtendRequest struct {
	UUIDs []string `json:"uuids"`
	Days  int      `json:"days"`
}
