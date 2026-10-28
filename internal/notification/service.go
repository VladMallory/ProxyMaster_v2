package notification

type NotificationGit struct {
	admin []Admin
}

func NewNotificationGit(admin []Admin) *NotificationGit {
	return &NotificationGit{
		admin: admin,
	}
}
