package userservice

type UserService interface {
	IsAdmin(userID int64) bool
	IsUsageAllowed(userID, chatID int64) bool
}
