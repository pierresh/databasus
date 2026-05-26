package cache_utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_CacheUtil_SetAndGet_ReturnsStoredValue(t *testing.T) {
	c := NewCacheUtil[string]("test:user:")

	value := "John Doe"
	c.Set("user1", &value)

	retrieved := c.Get("user1")
	assert.NotNil(t, retrieved)
	assert.Equal(t, value, *retrieved)
}

func Test_CacheUtil_ClearAll_RemovesAllEntries(t *testing.T) {
	c := NewCacheUtil[string]("test:clear:")

	keys := []string{"a", "b", "c"}
	for _, k := range keys {
		v := "value:" + k
		c.Set(k, &v)
	}

	c.ClearAll()

	for _, k := range keys {
		assert.Nil(t, c.Get(k), "key %s should be gone after ClearAll", k)
	}
}

func Test_CacheUtil_SetWithExpiration_SetsCorrectTTL(t *testing.T) {
	c := NewCacheUtil[string]("test:ttl:")

	value := "expires soon"
	c.SetWithExpiration("key1", &value, 100*time.Millisecond)

	assert.NotNil(t, c.Get("key1"), "value should be present before expiry")

	time.Sleep(150 * time.Millisecond)

	assert.Nil(t, c.Get("key1"), "value should be gone after expiry")
}

func Test_CacheUtil_Invalidate_RemovesEntry(t *testing.T) {
	c := NewCacheUtil[string]("test:inv:")

	value := "to remove"
	c.Set("k", &value)
	c.Invalidate("k")

	assert.Nil(t, c.Get("k"))
}

func Test_CacheUtil_GetAndDelete_RemovesOnRead(t *testing.T) {
	c := NewCacheUtil[string]("test:gad:")

	value := "one-shot"
	c.Set("k", &value)

	first := c.GetAndDelete("k")
	assert.NotNil(t, first)
	assert.Equal(t, value, *first)

	second := c.GetAndDelete("k")
	assert.Nil(t, second)
}
