package migrations

import (
	"fmt"
	"math/big"
	"os"

	"gorm.io/gorm"
)

const up56 = `
CREATE TABLE evm_chains (
	id numeric(78,0) PRIMARY KEY,
	cfg jsonb NOT NULL DEFAULT '{}',
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE TABLE nodes (
	id serial PRIMARY KEY,
	name varchar(256) NOT NULL CHECK (name != ''),
	evm_chain_id numeric(78,0) NOT NULL REFERENCES evm_chains (id),
	ws_url text CHECK (ws_url != ''),
	http_url text CHECK (http_url != ''),
	send_only bool NOT NULL CONSTRAINT primary_or_sendonly CHECK (
		(send_only AND ws_url IS NULL AND http_url IS NOT NULL)
		OR
		(NOT send_only AND ws_url IS NOT NULL)
	),
	created_at timestamptz NOT NULL,
	updated_at timestamptz NOT NULL
);

CREATE INDEX idx_nodes_evm_chain_id ON nodes (evm_chain_id);
CREATE UNIQUE INDEX idx_nodes_unique_name ON nodes (lower(name));

ALTER TABLE eth_txes ADD COLUMN evm_chain_id numeric(78,0) REFERENCES evm_chains (id);
ALTER TABLE log_broadcasts ADD COLUMN evm_chain_id numeric(78,0) REFERENCES evm_chains (id);
ALTER TABLE heads ADD COLUMN evm_chain_id numeric(78,0) REFERENCES evm_chains (id);
ALTER TABLE offchainreporting_oracle_specs ADD COLUMN evm_chain_id numeric(78,0) REFERENCES evm_chains (id);

INSERT INTO evm_chains (id, created_at, updated_at) VALUES (%[1]s, NOW(), NOW());
UPDATE eth_txes SET evm_chain_id = %[1]s;
UPDATE log_broadcasts SET evm_chain_id = %[1]s;
UPDATE heads SET evm_chain_id = %[1]s;

DROP INDEX IF EXISTS idx_eth_txes_min_unconfirmed_nonce_for_key;
DROP INDEX IF EXISTS idx_eth_txes_nonce_from_address;
DROP INDEX IF EXISTS idx_only_one_in_progress_tx_per_account;
DROP INDEX IF EXISTS idx_eth_txes_state_from_address;
DROP INDEX IF EXISTS idx_eth_txes_unstarted_subject_id;
CREATE INDEX idx_eth_txes_min_unconfirmed_nonce_for_key_evm_chain_id ON eth_txes(evm_chain_id, from_address, nonce) WHERE state = 'unconfirmed'::eth_txes_state;
CREATE UNIQUE INDEX idx_eth_txes_nonce_from_address_per_evm_chain_id ON eth_txes(evm_chain_id, from_address, nonce);
CREATE UNIQUE INDEX idx_only_one_in_progress_tx_per_account_id_per_evm_chain_id ON eth_txes(evm_chain_id, from_address) WHERE state = 'in_progress'::eth_txes_state;
CREATE INDEX idx_eth_txes_state_from_address_evm_chain_id ON eth_txes(evm_chain_id, from_address, state) WHERE state <> 'confirmed'::eth_txes_state;
CREATE INDEX idx_eth_txes_unstarted_subject_id_evm_chain_id ON eth_txes(evm_chain_id, subject, id) WHERE subject IS NOT NULL AND state = 'unstarted'::eth_txes_state;

DROP INDEX IF EXISTS idx_heads_hash;
DROP INDEX IF EXISTS idx_heads_number;
CREATE UNIQUE INDEX idx_heads_evm_chain_id_hash ON heads(evm_chain_id, hash);
CREATE INDEX idx_heads_evm_chain_id_number ON heads(evm_chain_id, number);

ALTER TABLE eth_txes ALTER COLUMN evm_chain_id SET NOT NULL;
ALTER TABLE log_broadcasts ALTER COLUMN evm_chain_id SET NOT NULL;
ALTER TABLE heads ALTER COLUMN evm_chain_id SET NOT NULL;
`

const down56 = `
ALTER TABLE heads DROP COLUMN evm_chain_id;
ALTER TABLE log_broadcasts DROP COLUMN evm_chain_id;
ALTER TABLE eth_txes DROP COLUMN evm_chain_id;

CREATE UNIQUE INDEX idx_heads_hash ON heads(hash bytea_ops);
CREATE INDEX idx_heads_number ON heads(number int8_ops);

CREATE INDEX idx_eth_txes_min_unconfirmed_nonce_for_key ON eth_txes(from_address bytea_ops,nonce int8_ops) WHERE state = 'unconfirmed'::eth_txes_state;
CREATE UNIQUE INDEX idx_eth_txes_nonce_from_address ON eth_txes(from_address bytea_ops,nonce int8_ops);
CREATE UNIQUE INDEX idx_only_one_in_progress_tx_per_account_id ON eth_txes(from_address bytea_ops) WHERE state = 'in_progress'::eth_txes_state;
CREATE INDEX idx_eth_txes_state_from_address ON eth_txes(from_address bytea_ops,state enum_ops) WHERE state <> 'confirmed'::eth_txes_state;
CREATE INDEX idx_eth_txes_unstarted_subject_id ON eth_txes(subject uuid_ops,id int8_ops) WHERE subject IS NOT NULL AND state = 'unstarted'::eth_txes_state;

DROP TABLE nodes;
DROP TABLE evm_chains;
`

func init() {
	Migrations = append(Migrations, &Migration{
		ID: "0056_multichain",
		Migrate: func(db *gorm.DB) error {
			chainIDStr := os.Getenv("ETH_CHAIN_ID")
			if chainIDStr == "" {
				chainIDStr = "1"
			}
			chainID, ok := new(big.Int).SetString(chainIDStr, 10)
			if !ok {
				panic(fmt.Sprintf("ETH_CHAIN_ID was invalid, expected a number, got: %s", chainIDStr))
			}

			sql := fmt.Sprintf(up56, chainID.String())
			return db.Exec(sql).Error
		},
		Rollback: func(db *gorm.DB) error {
			return db.Exec(down56).Error
		},
	})
}
