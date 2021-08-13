package evmtest

import (
	"math/big"
	"testing"

	"github.com/smartcontractkit/chainlink/core/chains/evm"
	evmtypes "github.com/smartcontractkit/chainlink/core/chains/evm/types"
	"github.com/smartcontractkit/chainlink/core/services/eth"
	"github.com/smartcontractkit/chainlink/core/store/config"
	"github.com/smartcontractkit/chainlink/core/utils"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func verifyMatchingChainIDs(t testing.TB, n *big.Int, m *big.Int) {
	require.Equal(t, n.Cmp(m), 0, "expected chain IDs to match")
}

type TestChainOpts struct {
	Client        eth.Client
	GeneralConfig config.GeneralConfig
	ChainCfg      evmtypes.ChainCfg
}

// NewChainCollection returns a simple chain collection with one chain and
// allows to mock client/config on that chain
func NewChainCollection(t testing.TB, testopts TestChainOpts) evm.ChainCollection {
	opts := evm.ChainCollectionOpts{}

	opts.GenEthClient = func(c evmtypes.Chain) eth.Client {
		return testopts.Client
	}

	chains := []evmtypes.Chain{
		{
			ID:  *utils.NewBigI(0),
			Cfg: testopts.ChainCfg,
		},
	}

	cc, err := evm.NewChainCollection(opts, chains)
	require.NoError(t, err)
	return cc
}

func MustGetDefaultChain(t testing.TB, cc evm.ChainCollection) evm.Chain {
	chain, err := cc.Default()
	require.NoError(t, err)
	return chain
}

func MustInsertChainWithNode(t testing.TB, db *gorm.DB, chain evmtypes.Chain) evmtypes.Chain {
	err := db.Create(&chain).Error
	require.NoError(t, err)
	return chain
}

// evm.NewChainCollection(opts , dbchains []types.Chain) (ChainCollection, error) {
//     name := uuid.NewV4().String()
//     chainID := rand.Int63()
//     wsURL := "ws://example.invalid"

//     c := MockChain

//     cll := evm.NewChainCollection()
// }

// INSERT INTO evm_chains (id, created_at, updated_at) VALUES (0, NOW(), NOW());

// INSERT INTO nodes (name, evm_chain_id, ws_url, http_url, send_only, created_at, updated_at) VALUES (
//     'primary-0',
//     0,
//
//     null,
//     false,
//     NOW(),
//     NOW()
// );

// var _ evm.Chain = &MockChain{}

// type MockChain struct{}

// // Start the service.
// func (c *MockChain) Start() error {
//     panic("not implemented") // TODO: Implement
// }

// // Stop the Service.
// // Invariants: Usually after this call the Service cannot be started
// // again, you need to build a new Service to do so.
// func (c *MockChain) Close() error {
//     panic("not implemented") // TODO: Implement
// }

// // Checkables should return nil if ready, or an error message otherwise.
// func (c *MockChain) Ready() error {
//     panic("not implemented") // TODO: Implement
// }

// // Checkables should return nil if healthy, or an error message otherwise.
// func (c *MockChain) Healthy() error {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) IsL2() bool {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) IsArbitrum() bool {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) IsOptimism() bool {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) ID() *big.Int {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) Client() eth.Client {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) Config() evmconfig.ChainScopedConfig {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) LogBroadcaster() log.Broadcaster {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) HeadBroadcaster() httypes.HeadBroadcaster {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) TxManager() bulletprooftxmanager.TxManager {
//     panic("not implemented") // TODO: Implement
// }

// func (c *MockChain) HeadTracker() httypes.Tracker {
//     panic("not implemented") // TODO: Implement
// }
