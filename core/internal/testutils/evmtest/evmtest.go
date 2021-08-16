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

func MustSetChainCfg(t testing.TB, db *gorm.DB, chainID *big.Int, chainCfg evmtypes.ChainCfg) {
	res := db.Exec(`UPDATE evm_chains SET cfg = ? WHERE id = ?`, chainCfg, chainID.String())
	if res.RowsAffected == 0 {
		t.Fatal("no chains updated")
	}
	require.NoError(t, res.Error)
}
