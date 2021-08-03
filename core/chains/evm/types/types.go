package types

import (
	"math/big"

	"github.com/smartcontractkit/chainlink/core/utils"
	null "gopkg.in/guregu/null.v4"
)

// TODO: Rename this to just 'Chain' and figure out what to do with the other model
type Chain struct {
	ID utils.Big `gorm:"primary_key"`
	// TODO: Add a name here?
	Nodes []Node `gorm:"->"`
	// TODO: Add a config here which can read from database overrides but defaults to the default chain config
}

func (Chain) TableName() string {
	return "evm_chains"
}
func (c Chain) IsL2() bool {
	return IsL2(c.ID.ToInt())
}
func (c Chain) IsArbitrum() bool {
	return IsArbitrum(c.ID.ToInt())
}
func (c Chain) IsOptimism() bool {
	return IsOptimism(c.ID.ToInt())
}

// IsArbitrum returns true if the chain is arbitrum mainnet or testnet
func IsArbitrum(id *big.Int) bool {
	return id.Cmp(big.NewInt(42161)) == 0 || id.Cmp(big.NewInt(421611)) == 0
}

// IsOptimism returns true if the chain is optimism mainnet or testnet
func IsOptimism(id *big.Int) bool {
	return id.Cmp(big.NewInt(10)) == 0 || id.Cmp(big.NewInt(69)) == 0
}

// IsL2 returns true if this chain is an L2 chain, notably that the block
// numbers used for log searching are different from calling block.number
func IsL2(id *big.Int) bool {
	return IsOptimism(id) || IsArbitrum(id)
}

type Node struct {
	ID         int32 `gorm:"primary_key"`
	Name       string
	EVMChain   Chain       `gorm:"foreignkey:EVMChainID"`
	EVMChainID utils.Big   `gorm:"column:evm_chain_id"`
	WSURL      string      `gorm:"column:ws_url"`
	HTTPURL    null.String `gorm:"column:http_url"`
	SendOnly   bool
}
