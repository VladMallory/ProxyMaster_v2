// Package viewtype содержит типы экранов Telegram UI.
// Вынесен в отдельный пакет, чтобы избежать циклических импортов
// между transport/telegram и feature-пакетами.
package viewtype

type ViewType string

const (
	ViewTypeDownloadApp        ViewType = "download_app"
	ViewTypeIosRegion          ViewType = "ios_region"
	ViewTypeTariffs            ViewType = "tariffs"
	ViewTypeTopUp              ViewType = "top_up"
	ViewTypeCheckPayment       ViewType = "check_payment"
	ViewTypeProfile            ViewType = "profile"
	ViewTypeDeviceLimits       ViewType = "device_limits"
	ViewTypeTrafficLimits      ViewType = "traffic_limits"
	ViewTypeConnect            ViewType = "connect"
	ViewTypeSubscriptionResult ViewType = "subscription_result"
	ViewTypeMain               ViewType = "main"
	ViewTypeServiceInfo        ViewType = "service_info"
	ViewTypePrivacyPolicy      ViewType = "privacy_policy"
	ViewTypeUserAgreement      ViewType = "user_agreement"
)
