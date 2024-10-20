package userservice

import "slices"

type UserServiceStatic struct {
	allowedUserIDs []int64
	allowedChatIDs []int64
	adminIDs       []int64
}

func NewUserServiceStatic(allowedUserIDs []int64, allowedChatIDs []int64, adminIDs []int64) UserService {
	return UserServiceStatic{allowedUserIDs, allowedChatIDs, adminIDs}
}

func (us UserServiceStatic) IsAdmin(userID int64) bool {
	return slices.Contains(us.adminIDs, userID)
}

func (us UserServiceStatic) IsUsageAllowed(userID, chatID int64) bool {
	return slices.Contains(us.allowedUserIDs, userID) ||
		slices.Contains(us.allowedChatIDs, chatID) ||
		us.IsAdmin(userID)
}
