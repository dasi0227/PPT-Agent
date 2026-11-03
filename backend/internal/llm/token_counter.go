package llm

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

const tokenCountCacheSize = 1024

type tokenCountEntry struct {
	key    [sha256.Size]byte
	tokens int
}

type tokenCounter struct {
	codec tokenizer.Codec
	mu    sync.Mutex
	cache map[[sha256.Size]byte]*list.Element
	lru   list.List
}

var contextTokenCounter = sync.OnceValue(func() *tokenCounter {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	if err != nil {
		panic(fmt.Errorf("initialize context tokenizer: %w", err))
	}
	return &tokenCounter{codec: codec, cache: make(map[[sha256.Size]byte]*list.Element)}
})

func (c *tokenCounter) count(text string) int {
	if text == "" {
		return 0
	}
	key := sha256.Sum256([]byte(text))
	c.mu.Lock()
	if entry, ok := c.cache[key]; ok {
		c.lru.MoveToFront(entry)
		tokens := entry.Value.(tokenCountEntry).tokens
		c.mu.Unlock()
		return tokens
	}
	c.mu.Unlock()

	tokens, err := c.codec.Count(text)
	if err != nil {
		panic(fmt.Errorf("count context tokens: %w", err))
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.cache[key]; ok {
		c.lru.MoveToFront(entry)
		return entry.Value.(tokenCountEntry).tokens
	}
	c.cache[key] = c.lru.PushFront(tokenCountEntry{key: key, tokens: tokens})
	if c.lru.Len() > tokenCountCacheSize {
		oldest := c.lru.Back()
		delete(c.cache, oldest.Value.(tokenCountEntry).key)
		c.lru.Remove(oldest)
	}
	return tokens
}
