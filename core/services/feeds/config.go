package feeds

import (
	"math/big"
	"time"

	"github.com/smartcontractkit/chainlink/core/store/models"
)

//go:generate mockery --name Config --output ./mocks/ --case=underscore

type Config interface {
	ChainID() *big.Int
	Dev() bool
	DefaultHTTPTimeout() models.Duration

	FeatureOffchainReporting() bool
	FeatureOffchainReporting2() bool
	OCRBlockchainTimeout(override time.Duration) time.Duration
	OCRContractConfirmations(override uint16) uint16
	OCRContractPollInterval(override time.Duration) time.Duration
	OCRContractSubscribeInterval(override time.Duration) time.Duration
	OCRContractTransmitterTransmitTimeout() time.Duration
	OCRDatabaseTimeout() time.Duration
	OCRObservationTimeout(override time.Duration) time.Duration
	OCRObservationGracePeriod() time.Duration

	OCR2BlockchainTimeout() time.Duration
	OCR2ContractConfirmations() uint16
	OCR2ContractPollInterval() time.Duration
	OCR2ContractSubscribeInterval() time.Duration
	OCR2ContractTransmitterTransmitTimeout() time.Duration
	OCR2DatabaseTimeout() time.Duration
	OCR2ObservationTimeout() time.Duration
	OCR2ObservationGracePeriod() time.Duration
}
