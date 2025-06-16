package storage

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"go-blockchain/pkg/blockchain"
)

// LevelDBStorage implements blockchain storage using LevelDB
type LevelDBStorage struct {
	db    *leveldb.DB
	mutex sync.RWMutex
	path  string
}

// NewLevelDBStorage creates a new LevelDB storage instance
func NewLevelDBStorage(dbPath string) (*LevelDBStorage, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open LevelDB: %w", err)
	}

	storage := &LevelDBStorage{
		db:   db,
		path: dbPath,
	}

	return storage, nil
}

// Close closes the LevelDB connection
func (s *LevelDBStorage) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// SaveBlock saves a block to LevelDB
func (s *LevelDBStorage) SaveBlock(block *blockchain.Block) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Serialize the block
	blockBytes, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}

	// Save by block hash
	hashKey := fmt.Sprintf("block_hash_%x", block.CurrentBlockHash)
	if err := s.db.Put([]byte(hashKey), blockBytes, nil); err != nil {
		return fmt.Errorf("failed to save block by hash: %w", err)
	}

	// Save by block index
	indexKey := fmt.Sprintf("block_index_%d", block.Index)
	if err := s.db.Put([]byte(indexKey), blockBytes, nil); err != nil {
		return fmt.Errorf("failed to save block by index: %w", err)
	}

	// Update latest block index
	latestIndexKey := "latest_block_index"
	indexBytes := []byte(strconv.FormatInt(block.Index, 10))
	if err := s.db.Put([]byte(latestIndexKey), indexBytes, nil); err != nil {
		return fmt.Errorf("failed to update latest block index: %w", err)
	}

	// Update block count
	countKey := "block_count"
	countBytes := []byte(strconv.FormatInt(block.Index+1, 10))
	if err := s.db.Put([]byte(countKey), countBytes, nil); err != nil {
		return fmt.Errorf("failed to update block count: %w", err)
	}

	return nil
}

// GetBlock retrieves a block by hash
func (s *LevelDBStorage) GetBlock(hash []byte) (*blockchain.Block, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := fmt.Sprintf("block_hash_%x", hash)
	blockBytes, err := s.db.Get([]byte(key), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block not found")
		}
		return nil, fmt.Errorf("failed to get block: %w", err)
	}

	var block blockchain.Block
	if err := json.Unmarshal(blockBytes, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetBlockByIndex retrieves a block by index
func (s *LevelDBStorage) GetBlockByIndex(index int64) (*blockchain.Block, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := fmt.Sprintf("block_index_%d", index)
	blockBytes, err := s.db.Get([]byte(key), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("block not found")
		}
		return nil, fmt.Errorf("failed to get block by index: %w", err)
	}

	var block blockchain.Block
	if err := json.Unmarshal(blockBytes, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	return &block, nil
}

// GetLatestBlock retrieves the latest block from storage
func (s *LevelDBStorage) GetLatestBlock() (*blockchain.Block, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Get latest block index
	latestIndexBytes, err := s.db.Get([]byte("latest_block_index"), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, fmt.Errorf("no blocks found")
		}
		return nil, fmt.Errorf("failed to get latest block index: %w", err)
	}

	latestIndex, err := strconv.ParseInt(string(latestIndexBytes), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse latest index: %w", err)
	}

	return s.GetBlockByIndex(latestIndex)
}

// GetBlockCount returns the total number of blocks
func (s *LevelDBStorage) GetBlockCount() (int64, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	countBytes, err := s.db.Get([]byte("block_count"), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get block count: %w", err)
	}

	count, err := strconv.ParseInt(string(countBytes), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block count: %w", err)
	}

	return count, nil
}

// LoadBlockchain loads the entire blockchain from storage
func (s *LevelDBStorage) LoadBlockchain() (*blockchain.Blockchain, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Create a new blockchain
	bc := blockchain.NewBlockchain()

	// Get block count
	count, err := s.GetBlockCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get block count: %w", err)
	}

	if count <= 1 {
		// Only genesis block or empty
		return bc, nil
	}

	// Load blocks from index 1 (skip genesis since it's already created)
	for i := int64(1); i < count; i++ {
		block, err := s.GetBlockByIndex(i)
		if err != nil {
			return nil, fmt.Errorf("failed to load block at index %d: %w", i, err)
		}

		if err := bc.AddBlock(block); err != nil {
			return nil, fmt.Errorf("failed to add block to blockchain: %w", err)
		}
	}

	return bc, nil
}

// SaveBlockchain saves the entire blockchain to storage
func (s *LevelDBStorage) SaveBlockchain(bc *blockchain.Blockchain) error {
	blocks := bc.GetAllBlocks()

	for _, block := range blocks {
		if err := s.SaveBlock(block); err != nil {
			return fmt.Errorf("failed to save block %d: %w", block.Index, err)
		}
	}

	return nil
}

// HasBlock checks if a block exists in storage
func (s *LevelDBStorage) HasBlock(hash []byte) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := fmt.Sprintf("block_hash_%x", hash)
	_, err := s.db.Get([]byte(key), nil)
	return err == nil
}

// GetBlocksFromIndex returns blocks starting from a specific index
func (s *LevelDBStorage) GetBlocksFromIndex(startIndex int64, limit int) ([]*blockchain.Block, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var blocks []*blockchain.Block
	currentIndex := startIndex

	for len(blocks) < limit {
		block, err := s.GetBlockByIndex(currentIndex)
		if err != nil {
			// No more blocks available
			break
		}

		blocks = append(blocks, block)
		currentIndex++
	}

	return blocks, nil
}

// Stats returns storage statistics
func (s *LevelDBStorage) Stats() (map[string]interface{}, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	count, err := s.GetBlockCount()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"database_path": s.path,
		"block_count":   count,
	}

	// Try to get latest block info
	if latest, err := s.GetLatestBlock(); err == nil {
		stats["latest_block_index"] = latest.Index
		stats["latest_block_hash"] = fmt.Sprintf("%x", latest.CurrentBlockHash)
	}

	return stats, nil
} 