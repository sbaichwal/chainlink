package config

import (
	"math/big"

	"gorm.io/gorm"
)

type chainScopedConfigORM struct {
	id *big.Int
	db *gorm.DB
}

func (o *chainScopedConfigORM) load(name string, val interface{}) error {
	panic("TODO")
}

func (o *chainScopedConfigORM) store(name string, val interface{}) error {
	panic("TODO")
}
