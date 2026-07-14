package bootstrap

import (
	"__GO_CRAFT_MODULE__/internal/adapter/sqlite/identity"
	"gorm.io/gorm"
)

func identityRepository(db *gorm.DB) *identity.UserRepository {
	return identity.NewUserRepository(db)
}
