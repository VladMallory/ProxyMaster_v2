package config

// хранит глобальные настройки для панели
type Config struct {
	RemnaPanelURL string // страница панели
	RemnawaveKey  string // ключ для разработчика, чтобы подключиться
}

func New() (*Config, error) {
	return &Config{
		RemnaPanelURL: "https://panel.moment-was-da.ru/auth/login?eyMjBapF=OXaAOjtG",
		RemnawaveKey:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1dWlkIjoiYWE2NWRiZTctNDAyMy00NGIzLThiYzQtOWM1NjJjZGI2ZTFjIiwidXNlcm5hbWUiOm51bGwsInJvbGUiOiJBUEkiLCJpYXQiOjE3NjYyMTU1MzAsImV4cCI6MTA0MDYxMjkxMzB9.5XToqcCL4v8aMBmBa6UcoM_DaYRPQbqJAwLkK4ZZIPQ",
	}, nil
}
