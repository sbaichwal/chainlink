package evmconfigtest

import (
	"math/big"
	"testing"
	"time"

	"github.com/smartcontractkit/chainlink/core/assets"
	evmconfig "github.com/smartcontractkit/chainlink/core/chains/evm/config"
	"github.com/smartcontractkit/chainlink/core/internal/testutils/configtest"
	null "gopkg.in/guregu/null.v4"
)

var _ evmconfig.ChainScopedConfig = &TestChainScopedConfig{}

var (
	MinimumContractPayment = assets.NewLink(100)
)

type ChainScopedConfigOverrides struct {
	EvmLogBackfillBatchSize null.Int

	BlockHistoryEstimatorBlockDelay       null.Int
	BlockHistoryEstimatorBlockHistorySize null.Int
	EvmFinalityDepth                      null.Int
	EvmMaxGasPriceWei                     *big.Int
	EvmGasBumpPercent                     null.Int
	EvmGasBumpTxDepth                     null.Int

	EvmGasLimitDefault null.Int

	EvmHeadTrackerHistoryDepth       null.Int
	EvmGasBumpWei                    *big.Int
	EvmGasLimitMultiplier            null.Float
	EvmGasPriceDefault               *big.Int
	EvmHeadTrackerSamplingInterval   *time.Duration
	EvmHeadTrackerMaxBufferSize      null.Int
	EthTxResendAfterThreshold        *time.Duration
	EvmNonceAutoSync                 null.Bool
	EvmRPCDefaultBatchSize           null.Int
	FlagsContractAddress             null.String
	GasEstimatorMode                 null.String
	MinRequiredOutgoingConfirmations null.Int
}

// TestChainScopedConfig defaults to whatever config.NewChainScopedConfig()
// gives but allows overriding certain methods
type TestChainScopedConfig struct {
	evmconfig.ChainScopedConfig
	Overrides     ChainScopedConfigOverrides
	GeneralConfig *configtest.TestGeneralConfig
	t             testing.TB
}

func NewTestChainScopedConfig(t testing.TB, generalcfg *configtest.TestGeneralConfig) *TestChainScopedConfig {
	evmcfg := evmconfig.NewChainScopedConfig(nil, generalcfg, generalcfg.DefaultChainID())
	return &TestChainScopedConfig{
		evmcfg,
		ChainScopedConfigOverrides{},
		generalcfg,
		t,
	}
}

func (c *TestChainScopedConfig) GasEstimatorMode() string {
	if c.Overrides.GasEstimatorMode.Valid {
		return c.Overrides.GasEstimatorMode.String
	}
	return "FixedPrice"
}

func (c *TestChainScopedConfig) EthTxResendAfterThreshold() time.Duration {
	if c.Overrides.EthTxResendAfterThreshold != nil {
		return *c.Overrides.EthTxResendAfterThreshold
	}
	return 0
}

func (c *TestChainScopedConfig) EvmFinalityDepth() uint {
	if c.Overrides.EvmFinalityDepth.Valid {
		return uint(c.Overrides.EvmFinalityDepth.Int64)
	}
	return 15
}

func (c *TestChainScopedConfig) EthTxReaperThreshold() time.Duration {
	return 0
}

func (c *TestChainScopedConfig) EthHeadTrackerSamplingInterval() time.Duration {
	return 100 * time.Millisecond
}

func (c *TestChainScopedConfig) EvmGasBumpThreshold() uint64 {
	return 3
}

func (c *TestChainScopedConfig) MinIncomingConfirmations() uint32 {
	return 1
}

func (c *TestChainScopedConfig) MinRequiredOutgoingConfirmations() uint64 {
	if c.Overrides.MinRequiredOutgoingConfirmations.Valid {
		return uint64(c.Overrides.MinRequiredOutgoingConfirmations.Int64)
	}
	return 1
}

func (c *TestChainScopedConfig) MinimumContractPayment() *assets.Link {
	return MinimumContractPayment
}

func (c *TestChainScopedConfig) BalanceMonitorEnabled() bool {
	return false
}

func (c *TestChainScopedConfig) EvmHeadTrackerMaxBufferSize() uint {
	if c.Overrides.EvmHeadTrackerMaxBufferSize.Valid {
		return uint(c.Overrides.EvmHeadTrackerMaxBufferSize.Int64)
	}
	return c.ChainScopedConfig.EvmHeadTrackerMaxBufferSize()
}

func (c *TestChainScopedConfig) EvmGasPriceDefault() *big.Int {
	if c.Overrides.EvmGasPriceDefault != nil {
		return c.Overrides.EvmGasPriceDefault
	}
	return c.ChainScopedConfig.EvmGasPriceDefault()
}

func (c *TestChainScopedConfig) SetEvmGasPriceDefault(p *big.Int) error {
	c.Overrides.EvmGasPriceDefault = p
	return nil
}

func (c *TestChainScopedConfig) BlockHistoryEstimatorBlockDelay() uint16 {
	if c.Overrides.BlockHistoryEstimatorBlockDelay.Valid {
		return uint16(c.Overrides.BlockHistoryEstimatorBlockDelay.Int64)
	}
	return c.ChainScopedConfig.BlockHistoryEstimatorBlockDelay()
}

func (c *TestChainScopedConfig) BlockHistoryEstimatorBlockHistorySize() uint16 {
	if c.Overrides.BlockHistoryEstimatorBlockHistorySize.Valid {
		return uint16(c.Overrides.BlockHistoryEstimatorBlockHistorySize.Int64)
	}
	return c.ChainScopedConfig.BlockHistoryEstimatorBlockHistorySize()
}

func (c *TestChainScopedConfig) EvmGasLimitMultiplier() float32 {
	if c.Overrides.EvmGasLimitMultiplier.Valid {
		return float32(c.Overrides.EvmGasLimitMultiplier.Float64)
	}
	return c.ChainScopedConfig.EvmGasLimitMultiplier()
}

func (c *TestChainScopedConfig) EvmNonceAutoSync() bool {
	if c.Overrides.EvmNonceAutoSync.Valid {
		return c.Overrides.EvmNonceAutoSync.Bool
	}
	return c.ChainScopedConfig.EvmNonceAutoSync()
}

func (c *TestChainScopedConfig) EvmGasBumpWei() *big.Int {
	if c.Overrides.EvmGasBumpWei != nil {
		return c.Overrides.EvmGasBumpWei
	}
	return c.ChainScopedConfig.EvmGasBumpWei()
}

func (c *TestChainScopedConfig) EvmGasBumpPercent() uint16 {
	if c.Overrides.EvmGasBumpPercent.Valid {
		return uint16(c.Overrides.EvmGasBumpPercent.Int64)
	}
	return c.ChainScopedConfig.EvmGasBumpPercent()
}

func (c *TestChainScopedConfig) EvmRPCDefaultBatchSize() uint32 {
	if c.Overrides.EvmRPCDefaultBatchSize.Valid {
		return uint32(c.Overrides.EvmRPCDefaultBatchSize.Int64)
	}
	return c.ChainScopedConfig.EvmRPCDefaultBatchSize()
}

func (c *TestChainScopedConfig) EvmMaxGasPriceWei() *big.Int {
	if c.Overrides.EvmMaxGasPriceWei != nil {
		return c.Overrides.EvmMaxGasPriceWei
	}
	return c.ChainScopedConfig.EvmMaxGasPriceWei()
}

func (c *TestChainScopedConfig) EvmGasBumpTxDepth() uint16 {
	if c.Overrides.EvmGasBumpTxDepth.Valid {
		return uint16(c.Overrides.EvmGasBumpTxDepth.Int64)
	}
	return c.ChainScopedConfig.EvmGasBumpTxDepth()
}

func (c *TestChainScopedConfig) FlagsContractAddress() string {
	if c.Overrides.FlagsContractAddress.Valid {
		return c.Overrides.FlagsContractAddress.String
	}
	return c.ChainScopedConfig.FlagsContractAddress()
}

func (c *TestChainScopedConfig) EvmHeadTrackerHistoryDepth() uint {
	if c.Overrides.EvmHeadTrackerHistoryDepth.Valid {
		return uint(c.Overrides.EvmHeadTrackerHistoryDepth.Int64)
	}
	return c.ChainScopedConfig.EvmHeadTrackerHistoryDepth()
}

func (c *TestChainScopedConfig) EvmHeadTrackerSamplingInterval() time.Duration {
	if c.Overrides.EvmHeadTrackerSamplingInterval != nil {
		return *c.Overrides.EvmHeadTrackerSamplingInterval
	}
	return c.ChainScopedConfig.EvmHeadTrackerSamplingInterval()
}

func (c *TestChainScopedConfig) EvmLogBackfillBatchSize() uint32 {
	if c.Overrides.EvmLogBackfillBatchSize.Valid {
		return uint32(c.Overrides.EvmLogBackfillBatchSize.Int64)
	}
	return c.ChainScopedConfig.EvmLogBackfillBatchSize()
}

func (c *TestChainScopedConfig) EvmGasLimitDefault() uint64 {
	if c.Overrides.EvmGasLimitDefault.Valid {
		return uint64(c.Overrides.EvmGasLimitDefault.Int64)
	}
	return c.ChainScopedConfig.EvmGasLimitDefault()
}
