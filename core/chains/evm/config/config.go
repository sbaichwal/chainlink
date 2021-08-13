package config

import (
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/pkg/errors"
	"github.com/smartcontractkit/chainlink/core/assets"
	evmtypes "github.com/smartcontractkit/chainlink/core/chains/evm/types"
	"github.com/smartcontractkit/chainlink/core/logger"
	"github.com/smartcontractkit/chainlink/core/store/config"
	ocr "github.com/smartcontractkit/libocr/offchainreporting"
	ocrtypes "github.com/smartcontractkit/libocr/offchainreporting/types"
	"go.uber.org/multierr"
	"gorm.io/gorm"
)

type EVMOnlyConfig interface {
	BalanceMonitorEnabled() bool
	BlockEmissionIdleWarningThreshold() time.Duration
	BlockHistoryEstimatorBatchSize() (size uint32)
	BlockHistoryEstimatorBlockDelay() uint16
	BlockHistoryEstimatorBlockHistorySize() uint16
	BlockHistoryEstimatorTransactionPercentile() uint16
	EthTxReaperInterval() time.Duration
	EthTxReaperThreshold() time.Duration
	EthTxResendAfterThreshold() time.Duration
	EvmDefaultBatchSize() uint32
	EvmFinalityDepth() uint
	EvmGasBumpPercent() uint16
	EvmGasBumpThreshold() uint64
	EvmGasBumpTxDepth() uint16
	EvmGasBumpWei() *big.Int
	EvmGasLimitDefault() uint64
	EvmGasLimitMultiplier() float32
	EvmGasLimitTransfer() uint64
	EvmGasPriceDefault() *big.Int
	EvmHeadTrackerHistoryDepth() uint
	EvmHeadTrackerMaxBufferSize() uint
	EvmHeadTrackerSamplingInterval() time.Duration
	EvmLogBackfillBatchSize() uint32
	EvmMaxGasPriceWei() *big.Int
	EvmMaxInFlightTransactions() uint32
	EvmMaxQueuedTransactions() uint64
	EvmMinGasPriceWei() *big.Int
	EvmNonceAutoSync() bool
	EvmRPCDefaultBatchSize() uint32
	FlagsContractAddress() string
	GasEstimatorMode() string
	LinkContractAddress() string
	MinIncomingConfirmations() uint32
	MinRequiredOutgoingConfirmations() uint64
	MinimumContractPayment() *assets.Link
	OCRContractConfirmations(override uint16) uint16
	SetEvmGasPriceDefault(value *big.Int) error
}

type ChainScopedConfig interface {
	EVMOnlyConfig
	config.GeneralConfig
	Validate() error
}

var _ ChainScopedConfig = &chainScopedConfig{}

type chainScopedConfig struct {
	config.GeneralConfig
	orm          *chainScopedConfigORM
	persistedCfg evmtypes.ChainCfg
	defaultSet   chainSpecificConfigDefaultSet
	id           *big.Int
	persistMu    sync.RWMutex
}

func NewChainScopedConfig(db *gorm.DB, gcfg config.GeneralConfig, chain evmtypes.Chain) ChainScopedConfig {
	orm := &chainScopedConfigORM{chain.ID.ToInt(), db}
	defaultSet, exists := chainSpecificConfigDefaultSets[chain.ID.ToInt().Int64()]
	if !exists {
		logger.Warnf("No chain-specific configuration found for chain %d, falling back to defaults", chain.ID.ToInt())
		defaultSet = fallbackDefaultSet
	}
	css := chainScopedConfig{gcfg, orm, chain.Cfg, defaultSet, chain.ID.ToInt(), sync.RWMutex{}}
	return &css
}

func (c *chainScopedConfig) Validate() (err error) {
	return multierr.Combine(
		c.GeneralConfig.Validate(),
		c.validate(),
	)
}

func (c *chainScopedConfig) validate() (err error) {
	ethGasBumpPercent := c.EvmGasBumpPercent()
	if uint64(ethGasBumpPercent) < core.DefaultTxPoolConfig.PriceBump {
		err = multierr.Combine(err, errors.Errorf(
			"ETH_GAS_BUMP_PERCENT of %v may not be less than Geth's default of %v",
			c.EvmGasBumpPercent(),
			core.DefaultTxPoolConfig.PriceBump,
		))
	}

	if uint32(c.EvmGasBumpTxDepth()) > c.EvmMaxInFlightTransactions() {
		err = multierr.Combine(err, errors.New("ETH_GAS_BUMP_TX_DEPTH must be less than or equal to ETH_MAX_IN_FLIGHT_TRANSACTIONS"))
	}
	if c.EvmMinGasPriceWei().Cmp(c.EvmGasPriceDefault()) > 0 {
		err = multierr.Combine(err, errors.New("ETH_MIN_GAS_PRICE_WEI must be less than or equal to ETH_GAS_PRICE_DEFAULT"))
	}
	if c.EvmMaxGasPriceWei().Cmp(c.EvmGasPriceDefault()) < 0 {
		err = multierr.Combine(err, errors.New("ETH_MAX_GAS_PRICE_WEI must be greater than or equal to ETH_GAS_PRICE_DEFAULT"))
	}
	if c.EvmHeadTrackerHistoryDepth() < c.EvmFinalityDepth() {
		err = multierr.Combine(err, errors.New("ETH_HEAD_TRACKER_HISTORY_DEPTH must be equal to or greater than ETH_FINALITY_DEPTH"))
	}
	if c.GasEstimatorMode() == "BlockHistory" && c.BlockHistoryEstimatorBlockHistorySize() <= 0 {
		err = multierr.Combine(err, errors.New("GAS_UPDATER_BLOCK_HISTORY_SIZE must be greater than or equal to 1 if block history estimator is enabled"))
	}
	if c.EvmFinalityDepth() < 1 {
		err = multierr.Combine(err, errors.New("ETH_FINALITY_DEPTH must be greater than or equal to 1"))
	}
	if c.MinIncomingConfirmations() < 1 {
		err = multierr.Combine(err, errors.New("MIN_INCOMING_CONFIRMATIONS must be greater than or equal to 1"))
	}
	var override time.Duration
	lc := ocrtypes.LocalConfig{
		BlockchainTimeout:                      c.OCRBlockchainTimeout(override),
		ContractConfigConfirmations:            c.OCRContractConfirmations(0),
		ContractConfigTrackerPollInterval:      c.OCRContractPollInterval(override),
		ContractConfigTrackerSubscribeInterval: c.OCRContractSubscribeInterval(override),
		ContractTransmitterTransmitTimeout:     c.OCRContractTransmitterTransmitTimeout(),
		DatabaseTimeout:                        c.OCRDatabaseTimeout(),
		DataSourceTimeout:                      c.OCRObservationTimeout(override),
		DataSourceGracePeriod:                  c.OCRObservationGracePeriod(),
	}
	if ocrerr := ocr.SanityCheckLocalConfig(lc); ocrerr != nil {
		err = multierr.Combine(err, ocrerr)
	}

	return err
}

func (c *chainScopedConfig) getPersistedCfg() evmtypes.ChainCfg {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.persistedCfg
}

// NOTE: The ENV vars used below will be removed after multichain is merged,
// since they no longer make sense when you can have zero or more chains. We
// will move to a chain-specific database config instead
// See: https://app.clubhouse.io/chainlinklabs/story/12739/generalise-necessary-models-tables-on-the-send-side-to-support-the-concept-of-multiple-chains

// EvmBalanceMonitorBlockDelay is the number of blocks that the balance monitor
// trails behind head. This is required e.g. for Infura because they will often
// announce a new head, then route a request to a different node which does not
// have this head yet.
func (c *chainScopedConfig) EvmBalanceMonitorBlockDelay() uint16 {
	val, ok := lookupEnv("ETH_BALANCE_MONITOR_BLOCK_DELAY", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	return c.defaultSet.balanceMonitorBlockDelay
}

// EvmGasBumpThreshold is the number of blocks to wait before bumping gas again on unconfirmed transactions
// Set to 0 to disable gas bumping
func (c *chainScopedConfig) EvmGasBumpThreshold() uint64 {
	val, ok := lookupEnv("ETH_GAS_BUMP_THRESHOLD", config.ParseUint64)
	if ok {
		return val.(uint64)
	}
	return c.defaultSet.gasBumpThreshold
}

// EvmGasBumpWei is the minimum fixed amount of wei by which gas is bumped on each transaction attempt
func (c *chainScopedConfig) EvmGasBumpWei() *big.Int {
	val, ok := lookupEnv("ETH_GAS_BUMP_WEI", config.ParseBigInt)
	if ok {
		return val.(*big.Int)
	}
	if c.persistedCfg.EvmGasBumpWei != nil {
		return c.persistedCfg.EvmGasBumpWei
	}
	n := c.defaultSet.gasBumpWei
	return &n
}

// EvmMaxInFlightTransactions controls how many transactions are allowed to be
// "in-flight" i.e. broadcast but unconfirmed at any one time
// 0 value disables the limit
func (c *chainScopedConfig) EvmMaxInFlightTransactions() uint32 {
	val, ok := lookupEnv("ETH_MAX_IN_FLIGHT_TRANSACTIONS", config.ParseUint32)
	if ok {
		return val.(uint32)
	}
	return c.defaultSet.maxInFlightTransactions
}

// EvmMaxGasPriceWei is the maximum amount in Wei that a transaction will be
// bumped to before abandoning it and marking it as errored.
func (c *chainScopedConfig) EvmMaxGasPriceWei() *big.Int {
	val, ok := lookupEnv("ETH_MAX_GAS_PRICE_WEI", config.ParseBigInt)
	if ok {
		return val.(*big.Int)
	}
	if c.persistedCfg.EvmMaxGasPriceWei != nil {
		return c.persistedCfg.EvmMaxGasPriceWei
	}
	n := c.defaultSet.maxGasPriceWei
	return &n
}

// EvmMaxQueuedTransactions is the maximum number of unbroadcast
// transactions per key that are allowed to be enqueued before jobs will start
// failing and rejecting send of any further transactions.
// 0 value disables
func (c *chainScopedConfig) EvmMaxQueuedTransactions() uint64 {
	val, ok := lookupEnv("ETH_MAX_QUEUED_TRANSACTIONS", config.ParseUint64)
	nf ok {
		return val.(uint64)
	}
	return c.defaultSet.maxQueuedTransactions
}

// EvmMinGasPriceWei is the minimum amount in Wei that a transaction may be priced.
// Chainlink will never send a transaction priced below this amount.
func (c *chainScopedConfig) EvmMinGasPriceWei() *big.Int {
	val, ok := lookupEnv("ETH_MIN_GAS_PRICE_WEI", config.ParseBigInt)
	if ok {
		return val.(*big.Int)
	}
	n := c.defaultSet.minGasPriceWei
	return &n
}

// EvmGasLimitDefault sets the default gas limit for outgoing transactions.
func (c *chainScopedConfig) EvmGasLimitDefault() uint64 {
	val, ok := lookupEnv("ETH_GAS_LIMIT_DEFAULT", config.ParseUint64)
	if ok {
		return val.(uint64)
	}
	if c.persistedCfg.EvmGasLimitDefault.Valid {
		return uint64(c.persistedCfg.EvmGasLimitDefault.Int64)
	}
	return c.defaultSet.gasLimitDefault
}

// EvmGasLimitTransfer is the gas limit for an ordinary eth->eth transfer
func (c *chainScopedConfig) EvmGasLimitTransfer() uint64 {
	val, ok := lookupEnv("ETH_GAS_LIMIT_TRANSFER", config.ParseUint64)
	if ok {
		return val.(uint64)
	}
	return c.defaultSet.gasLimitTransfer
}

// EvmGasPriceDefault is the starting gas price for every transaction
func (c *chainScopedConfig) EvmGasPriceDefault() *big.Int {
	val, ok := lookupEnv("ETH_GAS_PRICE_DEFAULT", config.ParseBigInt)
	if ok {
		return val.(*big.Int)
	}
	c.persistMu.RLock()
	defer c.persistMu.RLock()
	if c.persistedCfg.EvmGasPriceDefault != nil {
		return c.persistedCfg.EvmGasPriceDefault.ToInt()
	}
	n := c.defaultSet.gasPriceDefault
	return &n
}

// SetEvmGasPriceDefault saves a runtime value for the default gas price for transactions
func (c *chainScopedConfig) SetEvmGasPriceDefault(value *big.Int) error {
	min := c.EvmMinGasPriceWei()
	max := c.EvmMaxGasPriceWei()
	if value.Cmp(min) < 0 {
		return errors.Errorf("cannot set default gas price to %s, it is below the minimum allowed value of %s", value.String(), min.String())
	}
	if value.Cmp(max) > 0 {
		return errors.Errorf("cannot set default gas price to %s, it is above the maximum allowed value of %s", value.String(), max.String())
	}
	c.persistMu.Lock()
	defer c.persistMu.Unlock()
	c.persistedCfg.EvmGasPriceDefault = value
	// HACK: For now we do this manual cast which is less than ideal, but will
	// be replaced with chain-specific configs in a followup PR
	return c.orm.store("EvmGasPriceDefault", value)
}

// EvmFinalityDepth is the number of blocks after which an ethereum transaction is considered "final"
// BlocksConsideredFinal determines how deeply we look back to ensure that transactions are confirmed onto the longest chain
// There is not a large performance penalty to setting this relatively high (on the order of hundreds)
// It is practically limited by the number of heads we store in the database and should be less than this with a comfortable margin.
// If a transaction is mined in a block more than this many blocks ago, and is reorged out, we will NOT retransmit this transaction and undefined behaviour can occur including gaps in the nonce sequence that require manual intervention to fix.
// Therefore this number represents a number of blocks we consider large enough that no re-org this deep will ever feasibly happen.
//
// Special cases:
// ETH_FINALITY_DEPTH=0 would imply that transactions can be final even before they were mined into a block. This is not supported.
// ETH_FINALITY_DEPTH=1 implies that transactions are final after we see them in one block.
//
// Examples:
//
// Transaction sending:
// A transaction is sent at block height 42
//
// ETH_FINALITY_DEPTH is set to 5
// A re-org occurs at height 44 starting at block 41, transaction is marked for rebroadcast
// A re-org occurs at height 46 starting at block 41, transaction is marked for rebroadcast
// A re-org occurs at height 47 starting at block 41, transaction is NOT marked for rebroadcast
func (c *chainScopedConfig) EvmFinalityDepth() uint {
	val, ok := lookupEnv("ETH_FINALITY_DEPTH", config.ParseUint64)
	if ok {
		return val.(uint)
	}
	if c.persistedCfg.EvmFinalityDepth.Valid {
		return uint(c.persistedCfg.EvmFinalityDepth.Int64)
	}
	return c.defaultSet.finalityDepth
}

// EvmHeadTrackerHistoryDepth tracks the top N block numbers to keep in the `heads` database table.
// Note that this can easily result in MORE than N records since in the case of re-orgs we keep multiple heads for a particular block height.
// This number should be at least as large as `EvmFinalityDepth`.
// There may be a small performance penalty to setting this to something very large (10,000+)
func (c *chainScopedConfig) EvmHeadTrackerHistoryDepth() uint {
	val, ok := lookupEnv("ETH_HEAD_TRACKER_HISTORY_DEPTH", config.ParseUint64)
	if ok {
		return val.(uint)
	}
	if c.persistedCfg.EvmHeadTrackerHistoryDepth.Valid {
		return uint(c.persistedCfg.EvmHeadTrackerHistoryDepth.Int64)
	}
	return c.defaultSet.headTrackerHistoryDepth
}

// EvmHeadTrackerSamplingInterval is the interval between sampled head callbacks
// to services that are only interested in the latest head every some time
func (c *chainScopedConfig) EvmHeadTrackerSamplingInterval() time.Duration {
	val, ok := lookupEnv("ETH_HEAD_TRACKER_SAMPLING_INTERVAL", config.ParseDuration)
	if ok {
		return val.(time.Duration)
	}
	if c.persistedCfg.EvmHeadTrackerSamplingInterval != nil {
		return *c.persistedCfg.EvmHeadTrackerSamplingInterval
	}
	return c.defaultSet.headTrackerSamplingInterval
}

// BlockEmissionIdleWarningThreshold is the duration of time since last received head
// to print a warning log message indicating not receiving heads
func (c *chainScopedConfig) BlockEmissionIdleWarningThreshold() time.Duration {
	return c.defaultSet.blockEmissionIdleWarningThreshold
}

// EthTxResendAfterThreshold controls how long the ethResender will wait before
// re-sending the latest eth_tx_attempt. This is designed a as a fallback to
// protect against the eth nodes dropping txes (it has been anecdotally
// observed to happen), networking issues or txes being ejected from the
// mempool.
// See eth_resender.go for more details
func (c *chainScopedConfig) EthTxResendAfterThreshold() time.Duration {
	val, ok := lookupEnv("ETH_TX_RESEND_AFTER_THRESHOLD", config.ParseDuration)
	if ok {
		return val.(time.Duration)
	}
	if c.persistedCfg.EthTxResendAfterThreshold != nil {
		return c.persistedCfg.EthTxResendAfterThreshold.Duration()
	}
	return c.defaultSet.ethTxResendAfterThreshold
}

// BlockHistoryEstimatorBatchSize sets the maximum number of blocks to fetch in one batch in the block history estimator
// If the env var GAS_UPDATER_BATCH_SIZE is set to 0, it defaults to ETH_RPC_DEFAULT_BATCH_SIZE
func (c *chainScopedConfig) BlockHistoryEstimatorBatchSize() (size uint32) {
	val, ok := lookupEnv("BLOCK_HISTORY_ESTIMATOR_BATCH_SIZE", config.ParseUint32)
	if ok {
		size = val.(uint32)
	} else {
		size = c.defaultSet.blockHistoryEstimatorBatchSize
	}
	if size > 0 {
		return size
	}
	return c.EvmDefaultBatchSize()
}

// BlockHistoryEstimatorBlockDelay is the number of blocks that the block history estimator trails behind head.
// E.g. if this is set to 3, and we receive block 10, block history estimator will
// fetch block 7.
// CAUTION: You might be tempted to set this to 0 to use the latest possible
// block, but it is possible to receive a head BEFORE that block is actually
// available from the connected node via RPC. In this case you will get false
// "zero" blocks that are missing transactions.
func (c *chainScopedConfig) BlockHistoryEstimatorBlockDelay() uint16 {
	val, ok := lookupEnv("BLOCK_HISTORY_ESTIMATOR_BLOCK_DELAY", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	if c.persistedCfg.BlockHistoryEstimatorBlockDelay.Valid {
		return uint16(c.persistedCfg.BlockHistoryEstimatorBlockDelay.Int64)
	}
	return c.defaultSet.blockHistoryEstimatorBlockDelay
}

// BlockHistoryEstimatorBlockHistorySize is the number of past blocks to keep in memory to
// use as a basis for calculating a percentile gas price
func (c *chainScopedConfig) BlockHistoryEstimatorBlockHistorySize() uint16 {
	val, ok := lookupEnv("BLOCK_HISTORY_ESTIMATOR_BLOCK_HISTORY_SIZE", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	if c.persistedCfg.BlockHistoryEstimatorBlockHistorySize.Valid {
		return uint16(c.persistedCfg.BlockHistoryEstimatorBlockHistorySize.Int64)
	}
	return c.defaultSet.blockHistoryEstimatorBlockHistorySize
}

// BlockHistoryEstimatorTransactionPercentile is the percentile gas price to choose. E.g.
// if the past transaction history contains four transactions with gas prices:
// [100, 200, 300, 400], picking 25 for this number will give a value of 200
func (c *chainScopedConfig) BlockHistoryEstimatorTransactionPercentile() uint16 {
	val, ok := lookupEnv("BLOCK_HISTORY_ESTIMATOR_TRANSACTION_PERCENTILE", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	return c.defaultSet.blockHistoryEstimatorTransactionPercentile
}

// GasEstimatorMode controls what type of gas estimator is used
func (c *chainScopedConfig) GasEstimatorMode() string {
	if c.EthereumDisabled() {
		return "FixedPrice"
	}
	val, ok := lookupEnv("GAS_ESTIMATOR_MODE", config.ParseString)
	if ok {
		return val.(string)
	}
	if c.persistedCfg.GasEstimatorMode.Valid {
		return c.persistedCfg.GasEstimatorMode.String
	}
	return c.defaultSet.gasEstimatorMode
}

// LinkContractAddress represents the address of the official LINK token
// contract on the current Chain
func (c *chainScopedConfig) LinkContractAddress() string {
	val, ok := lookupEnv("LINK_CONTRACT_ADDRESS", config.ParseString)
	if ok {
		return val.(string)
	}
	return c.defaultSet.linkContractAddress
}

func (c *chainScopedConfig) OCRContractConfirmations(override uint16) uint16 {
	if override != uint16(0) {
		return override
	}
	val, ok := lookupEnv("OCR_CONTRACT_CONFIRMATIONS", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	return c.defaultSet.ocrContractConfirmations
}

// MinIncomingConfirmations represents the minimum number of block
// confirmations that need to be recorded since a job run started before a task
// can proceed.
// MIN_INCOMING_CONFIRMATIONS=1 would kick off a job after seeing the transaction in a block
// MIN_INCOMING_CONFIRMATIONS=0 would kick off a job even before the transaction is mined, which is not supported
func (c *chainScopedConfig) MinIncomingConfirmations() uint32 {
	val, ok := lookupEnv("MIN_INCOMING_CONFIRMATIONS", config.ParseUint32)
	if ok {
		return val.(uint32)
	}
	return c.defaultSet.minIncomingConfirmations
}

// MinRequiredOutgoingConfirmations represents the default minimum number of block
// confirmations that need to be recorded on an outgoing ethtx task before the run can move onto the next task.
// This can be overridden on a per-task basis by setting the `MinRequiredOutgoingConfirmations` parameter.
// MIN_OUTGOING_CONFIRMATIONS=1 considers a transaction as "done" once it has been mined into one block
// MIN_OUTGOING_CONFIRMATIONS=0 would consider a transaction as "done" even before it has been mined
func (c *chainScopedConfig) MinRequiredOutgoingConfirmations() uint64 {
	val, ok := lookupEnv("MIN_REQUIRED_OUTGOING_CONFIRMATIONS", config.ParseUint64)
	if ok {
		return val.(uint64)
	}
	if c.persistedCfg.MinRequiredOutgoingConfirmations.Valid {
		return uint64(c.persistedCfg.MinRequiredOutgoingConfirmations.Int64)
	}
	return c.defaultSet.minRequiredOutgoingConfirmations
}

// MinimumContractPayment represents the minimum amount of LINK that must be
// supplied for a contract to be considered.
func (c *chainScopedConfig) MinimumContractPayment() *assets.Link {
	val, ok := lookupEnv("MINIMUM_CONTRACT_PAYMENT_LINK_JUELS", config.ParseLink)
	if ok {
		return val.(*assets.Link)
	}
	return c.defaultSet.minimumContractPayment
}

// EvmGasBumpTxDepth is the number of transactions to gas bump starting from oldest.
// Set to 0 for no limit (i.e. bump all)
func (c *chainScopedConfig) EvmGasBumpTxDepth() uint16 {
	val, ok := lookupEnv("ETH_GAS_BUMP_TX_DEPTH", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	if c.persistedCfg.EvmGasBumpTxDepth.Valid {
		return uint16(c.persistedCfg.EvmGasBumpTxDepth.Int64)
	}
	return c.defaultSet.gasBumpTxDepth
}

// EvmDefaultBatchSize controls the number of receipts fetched in each
// request in the EvmConfirmer
func (c *chainScopedConfig) EvmDefaultBatchSize() uint32 {
	val, ok := lookupEnv("ETH_RPC_DEFAULT_BATCH_SIZE", config.ParseUint32)
	if ok {
		return val.(uint32)
	}
	return c.defaultSet.rpcDefaultBatchSize
}

// EvmGasBumpPercent is the minimum percentage by which gas is bumped on each transaction attempt
// Change with care since values below geth's default will fail with "underpriced replacement transaction"
func (c *chainScopedConfig) EvmGasBumpPercent() uint16 {
	val, ok := lookupEnv("ETH_GAS_BUMP_PERCENT", config.ParseUint16)
	if ok {
		return val.(uint16)
	}
	if c.persistedCfg.EvmGasBumpPercent.Valid {
		return uint16(c.persistedCfg.EvmGasBumpPercent.Int64)
	}
	return c.defaultSet.gasBumpPercent
}

// EvmNonceAutoSync enables/disables running the NonceSyncer on application start
func (c *chainScopedConfig) EvmNonceAutoSync() bool {
	val, ok := lookupEnv("ETH_NONCE_AUTO_SYNC", config.ParseBool)
	if ok {
		return val.(bool)
	}
	if c.persistedCfg.EvmNonceAutoSync.Valid {
		return c.persistedCfg.EvmNonceAutoSync.Bool
	}
	return c.defaultSet.nonceAutoSync
}

// EvmGasLimitMultiplier is a factor by which a transaction's GasLimit is
// multiplied before transmission. So if the value is 1.1, and the GasLimit for
// a transaction is 10, 10% will be added before transmission.
//
// This factor is always applied, so includes Optimism L2 transactions which
// uses a default gas limit of 1 and is also applied to EvmGasLimitDefault.
func (c *chainScopedConfig) EvmGasLimitMultiplier() float32 {
	val, ok := lookupEnv("ETH_GAS_LIMIT_MULTIPLIER", config.ParseF32)
	if ok {
		return val.(float32)
	}
	if c.persistedCfg.EvmGasLimitMultiplier.Valid {
		return float32(c.persistedCfg.EvmGasLimitMultiplier.Float64)
	}
	return c.defaultSet.gasLimitMultiplier
}

// EvmHeadTrackerMaxBufferSize is the maximum number of heads that may be
// buffered in front of the head tracker before older heads start to be
// dropped. You may think of it as something like the maximum permittable "lag"
// for the head tracker before we start dropping heads to keep up.
func (c *chainScopedConfig) EvmHeadTrackerMaxBufferSize() uint {
	val, ok := lookupEnv("ETH_HEAD_TRACKER_MAX_BUFFER_SIZE", config.ParseUint64)
	if ok {
		return uint(val.(uint64))
	}
	if c.persistedCfg.EvmHeadTrackerMaxBufferSize.Valid {
		return uint(c.persistedCfg.EvmHeadTrackerMaxBufferSize.Int64)
	}
	return c.defaultSet.headTrackerMaxBufferSize
}

// EthTxReaperInterval controls how often the eth tx reaper should run
func (c *chainScopedConfig) EthTxReaperInterval() time.Duration {
	val, ok := lookupEnv("ETH_TX_REAPER_INTERVAL", config.ParseDuration)
	if ok {
		return val.(time.Duration)
	}
	return c.defaultSet.ethTxReaperInterval
}

// EthTxReaperThreshold represents how long any confirmed/fatally_errored eth_txes will hang around in the database.
// If the eth_tx is confirmed but still below ETH_FINALITY_DEPTH it will not be deleted even if it was created at a time older than this value.
// EXAMPLE
// With:
// EthTxReaperThreshold=1h
// EvmFinalityDepth=50
//
// Current head is 142, any eth_tx confirmed in block 91 or below will be reaped as long as its created_at was more than EthTxReaperThreshold ago
// Set to 0 to disable eth_tx reaping
func (c *chainScopedConfig) EthTxReaperThreshold() time.Duration {
	val, ok := lookupEnv("ETH_TX_REAPER_THRESHOLD", config.ParseDuration)
	if ok {
		return val.(time.Duration)
	}
	return c.defaultSet.ethTxReaperThreshold
}

// EvmLogBackfillBatchSize sets the batch size for calling FilterLogs when we backfill missing logs
func (c *chainScopedConfig) EvmLogBackfillBatchSize() uint32 {
	val, ok := lookupEnv("ETH_LOG_BACKFILL_BATCH_SIZE", config.ParseUint32)
	if ok {
		return val.(uint32)
	}
	if c.persistedCfg.EvmLogBackfillBatchSize.Valid {
		return uint32(c.persistedCfg.EvmLogBackfillBatchSize.Int64)
	}
	return c.defaultSet.logBackfillBatchSize
}

// EvmRPCDefaultBatchSize controls the number of receipts fetched in each
// request in the EvmConfirmer
func (c *chainScopedConfig) EvmRPCDefaultBatchSize() uint32 {
	val, ok := lookupEnv("ETH_RPC_DEFAULT_BATCH_SIZE", config.ParseUint32)
	if ok {
		return val.(uint32)
	}
	if c.persistedCfg.EvmRPCDefaultBatchSize.Valid {
		return uint32(c.persistedCfg.EvmRPCDefaultBatchSize.Int64)
	}
	return c.defaultSet.rpcDefaultBatchSize
}

// FlagsContractAddress represents the Flags contract address
func (c *chainScopedConfig) FlagsContractAddress() string {
	val, ok := lookupEnv("FLAGS_CONTRACT_ADDRESS", config.ParseString)
	if ok {
		return val.(string)
	}
	if c.persistedCfg.FlagsContractAddress.Valid {
		return c.persistedCfg.FlagsContractAddress.String
	}
	return c.defaultSet.flagsContractAddress
}

// BalanceMonitorEnabled enables the balance monitor
func (c *chainScopedConfig) BalanceMonitorEnabled() bool {
	if c.EthereumDisabled() {
		return false
	}
	val, ok := lookupEnv("BALANCE_MONITOR_ENABLED", config.ParseBool)
	if ok {
		return val.(bool)
	}
	return c.defaultSet.balanceMonitorEnabled
}

func lookupEnv(k string, parse func(string) (interface{}, error)) (interface{}, bool) {
	s, ok := os.LookupEnv(k)
	if ok {
		val, err := parse(s)
		if err != nil {
			logger.Errorw(
				fmt.Sprintf("Invalid value provided for %s, falling back to default.", s),
				"value", s,
				"key", k,
				"error", err)
			return nil, false
		}
		return val, true
	}
	return nil, false
}
