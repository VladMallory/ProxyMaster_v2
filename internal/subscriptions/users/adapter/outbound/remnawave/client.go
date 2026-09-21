package remnawave

import (
	platformremnawave "github.com/VladMallory/ProxyMaster_v2/internal/platform/remnawave"
)

type RemnawaveAdapter struct {
	apiKey string
	client *platformremnawave.Client
}

func NewRemnawaveClient(client *platformremnawave.Client, apiKey string) *RemnawaveAdapter {
	return &RemnawaveAdapter{
		client: client,
		apiKey: apiKey,
	}
}

// func mapErr(err error) error {
// 	if errors.Is(err, platformremnawave.ErrNotFound) {
// 		return platformremnawave.ErrNotFound
// 	}
//
// 	return err
// }
