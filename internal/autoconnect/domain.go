package autoconnect

// Приложения для которых строится страница автоподключения.
const (
	ProviderHapp = "happ"
	ProviderIncy = "incy"
)

// Link ссылка подписки, которую нужно открыть в приложении.
type Link struct {
	Subscription string
}
