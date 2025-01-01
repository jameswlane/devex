package userutil

import "os/user"

type UserProvider interface {
	Current() (*user.User, error)
	Lookup(username string) (*user.User, error)
}

type OSUserProvider struct{}

func (OSUserProvider) Current() (*user.User, error) {
	return user.Current()
}

func (OSUserProvider) Lookup(username string) (*user.User, error) {
	return user.Lookup(username)
}

var UserProviderInstance UserProvider = OSUserProvider{}

func Current() (*user.User, error) {
	return UserProviderInstance.Current()
}

func Lookup(username string) (*user.User, error) {
	return UserProviderInstance.Lookup(username)
}
