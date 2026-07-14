package bootstrap

import (
	"__GO_CRAFT_MODULE__/internal/adapter/postgres/identity"
	"gorm.io/gorm"
)

func identityRepository(db *gorm.DB) *identity.UserRepository {
	return identity.NewUserRepository(db)
}
