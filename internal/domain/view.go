package domain

// ViewType тип отображаемого экрана в Telegram.
type ViewType string

const (
	ViewTypeDownloadApp ViewType = "download_app"
	ViewTypeIosRegion   ViewType = "ios_region"
	ViewTypeTariffs     ViewType = "tariffs"
	ViewTypeTopUp       ViewType = "top_up"
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
