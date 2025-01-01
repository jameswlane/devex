package userutil

import "os/user"

type MockUserProvider struct {
	CurrentUser *user.User
	Users       map[string]*user.User
}

func NewMockUserProvider() *MockUserProvider {
	return &MockUserProvider{
		Users: make(map[string]*user.User),
	}
}

func (m *MockUserProvider) Current() (*user.User, error) {
	if m.CurrentUser != nil {
		return m.CurrentUser, nil
	}
	return nil, user.UnknownUserError("current user not set")
}

func (m *MockUserProvider) Lookup(username string) (*user.User, error) {
	if user, exists := m.Users[username]; exists {
		return user, nil
	}
	return nil, user.UnknownUserError(username)
}
