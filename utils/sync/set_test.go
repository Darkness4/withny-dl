package sync

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSet(t *testing.T) {
	s := NewSet[string]()
	require.NotNil(t, s)
	assert.Equal(t, 0, s.Len())
}

func TestSyncSet_Add(t *testing.T) {
	s := NewSet[string]()

	s.Set("key1")
	assert.True(t, s.Contains("key1"))
	assert.Equal(t, 1, s.Len())

	// Adding duplicate should not increase length
	s.Set("key1")
	assert.Equal(t, 1, s.Len())

	s.Set("key2")
	assert.Equal(t, 2, s.Len())
}

func TestSyncSet_Contains(t *testing.T) {
	s := NewSet[string]()

	assert.False(t, s.Contains("nonexistent"))

	s.Set("exists")
	assert.True(t, s.Contains("exists"))
	assert.False(t, s.Contains("notexists"))
}

func TestSyncSet_Remove(t *testing.T) {
	s := NewSet[string]()

	s.Set("key1")
	s.Set("key2")

	s.Release("key1")
	assert.False(t, s.Contains("key1"))
	assert.True(t, s.Contains("key2"))
	assert.Equal(t, 1, s.Len())

	// Removing non-existent key should be safe
	s.Release("nonexistent")
	assert.Equal(t, 1, s.Len())
}

func TestSyncSet_Len(t *testing.T) {
	s := NewSet[int]()

	assert.Equal(t, 0, s.Len())

	for i := 0; i < 10; i++ {
		s.Set(i)
	}
	assert.Equal(t, 10, s.Len())

	for i := 0; i < 5; i++ {
		s.Release(i)
	}
	assert.Equal(t, 5, s.Len())
}

func TestSyncSet_DifferentTypes(t *testing.T) {
	t.Run("int type", func(t *testing.T) {
		intSet := NewSet[int]()
		intSet.Set(42)
		assert.True(t, intSet.Contains(42))
	})

	t.Run("struct type", func(t *testing.T) {
		type Person struct {
			ID   int
			Name string
		}
		personSet := NewSet[Person]()
		p := Person{ID: 1, Name: "Alice"}
		personSet.Set(p)
		assert.True(t, personSet.Contains(p))
	})
}

func TestSyncSet_ConcurrentAdd(t *testing.T) {
	s := NewSet[int]()
	var wg sync.WaitGroup

	numGoroutines := 100
	itemsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for j := 0; j < itemsPerGoroutine; j++ {
				s.Set(offset*itemsPerGoroutine + j)
			}
		}(i)
	}

	wg.Wait()

	expected := numGoroutines * itemsPerGoroutine
	assert.Equal(t, expected, s.Len())
}

func TestSyncSet_ConcurrentAddRemove(t *testing.T) {
	s := NewSet[int]()
	var addWg sync.WaitGroup
	var removeWg sync.WaitGroup

	numGoroutines := 50

	// Add items
	for i := range numGoroutines {
		addWg.Add(1)
		go func(val int) {
			defer addWg.Done()
			s.Set(val)
		}(i)
	}

	// Wait for all adds to complete
	addWg.Wait()
	assert.Equal(t, numGoroutines, s.Len())

	// Remove items concurrently
	for i := 0; i < numGoroutines/2; i++ {
		removeWg.Add(1)
		go func(val int) {
			defer removeWg.Done()
			s.Release(val)
		}(i)
	}

	removeWg.Wait()

	expected := numGoroutines - numGoroutines/2
	assert.Equal(t, expected, s.Len())

	// Verify the correct items remain
	for i := 0; i < numGoroutines/2; i++ {
		assert.False(t, s.Contains(i), "item %d should be removed", i)
	}
	for i := numGoroutines / 2; i < numGoroutines; i++ {
		assert.True(t, s.Contains(i), "item %d should still exist", i)
	}
}

func TestSyncSet_ConcurrentContains(t *testing.T) {
	s := NewSet[string]()
	var wg sync.WaitGroup

	// Pre-populate
	for i := 0; i < 100; i++ {
		s.Set("key" + string(rune(i)))
	}

	// Concurrent reads
	numReaders := 100
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				s.Contains("key" + string(rune(j%100)))
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions detected
}

func TestSyncSet_ConcurrentMixed(t *testing.T) {
	s := NewSet[int]()
	var wg sync.WaitGroup

	operations := 1000

	// Writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				s.Set(id*operations + j)
			}
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				s.Contains(j)
				s.Len()
			}
		}()
	}

	// Removers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				s.Release(id*operations + j)
			}
		}(i)
	}

	wg.Wait()
	// Test passes if no race conditions or deadlocks
}

func TestSyncSet_MultipleOperations(t *testing.T) {
	s := NewSet[string]()

	// Add multiple items
	items := []string{"a", "b", "c", "d", "e"}
	for _, item := range items {
		s.Set(item)
	}

	assert.Equal(t, len(items), s.Len())

	// Verify all items exist
	for _, item := range items {
		assert.True(t, s.Contains(item))
	}

	// Remove some items
	s.Release("b")
	s.Release("d")

	assert.Equal(t, 3, s.Len())
	assert.True(t, s.Contains("a"))
	assert.False(t, s.Contains("b"))
	assert.True(t, s.Contains("c"))
	assert.False(t, s.Contains("d"))
	assert.True(t, s.Contains("e"))
}

func TestSyncSet_EmptyOperations(t *testing.T) {
	s := NewSet[string]()

	// Operations on empty set
	assert.False(t, s.Contains("anything"))
	assert.Equal(t, 0, s.Len())

	// Remove from empty set should not panic
	s.Release("nonexistent")
	assert.Equal(t, 0, s.Len())
}
