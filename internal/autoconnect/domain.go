package autoconnect

// Приложения для которых строится страница автоподключения.
const (
	ProviderHapp = "happ"
	ProviderIncy = "incy"
)

// Link ссылка подписки, которую нужно открыть в приложении.
// Формат deep link зависит от приложения, поэтому его собирают адаптеры.
type Link struct {
	Subscription string
}
