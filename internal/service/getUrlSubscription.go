// Package service содержит сервис по получению URL подписки,
// взаимодействующий с remnawave.
package service

import (
	"ProxyMaster_v2/internal/domain"
)

type subscriptionService struct {
	remnawaveClient domain.RemnawaveClient
}

//func NewSubscriptionService(remnawaveClient domain.RemnawaveClient) domain.SubscriptionService {
//	return &subscriptionService{
//		remnawaveClient: remnawaveClient,
//	}
//}
