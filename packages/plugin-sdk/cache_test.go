package sdk_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("MemoryCache", func() {
	var cache *sdk.MemoryCache

	BeforeEach(func() {
		cache = sdk.NewMemoryCache(100 * time.Millisecond)
	})

	Describe("Basic Operations", func() {
		It("should store and retrieve values", func() {
			cache.Set("key1", "value1")
			
			value, found := cache.Get("key1")
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("value1"))
		})

		It("should return false for non-existent keys", func() {
			value, found := cache.Get("nonexistent")
			Expect(found).To(BeFalse())
			Expect(value).To(BeNil())
		})

		It("should overwrite existing values", func() {
			cache.Set("key1", "value1")
			cache.Set("key1", "value2")
			
			value, found := cache.Get("key1")
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("value2"))
		})

		It("should delete values", func() {
			cache.Set("key1", "value1")
			cache.Delete("key1")
			
			value, found := cache.Get("key1")
			Expect(found).To(BeFalse())
			Expect(value).To(BeNil())
		})

		It("should clear all values", func() {
			cache.Set("key1", "value1")
			cache.Set("key2", "value2")
			cache.Set("key3", "value3")
			
			cache.Clear()
			
			_, found1 := cache.Get("key1")
			_, found2 := cache.Get("key2")
			_, found3 := cache.Get("key3")
			
			Expect(found1).To(BeFalse())
			Expect(found2).To(BeFalse())
			Expect(found3).To(BeFalse())
		})
	})

	Describe("TTL Behavior", func() {
		It("should expire values after TTL", func() {
			cache.Set("key1", "value1")
			
			// Value should exist initially
			value, found := cache.Get("key1")
			Expect(found).To(BeTrue())
			Expect(value).To(Equal("value1"))
			
			// Wait for TTL to expire
			time.Sleep(150 * time.Millisecond)
			
			// Value should be expired
			value, found = cache.Get("key1")
			Expect(found).To(BeFalse())
			Expect(value).To(BeNil())
		})

		It("should support custom TTL per item", func() {
			cache.SetWithTTL("key1", "value1", 50*time.Millisecond)
			cache.SetWithTTL("key2", "value2", 200*time.Millisecond)
			
			// Both should exist initially
			_, found1 := cache.Get("key1")
			_, found2 := cache.Get("key2")
			Expect(found1).To(BeTrue())
			Expect(found2).To(BeTrue())
			
			// Wait for first TTL to expire
			time.Sleep(60 * time.Millisecond)
			
			// First should be expired, second should still exist
			_, found1 = cache.Get("key1")
			_, found2 = cache.Get("key2")
			Expect(found1).To(BeFalse())
			Expect(found2).To(BeTrue())
			
			// Wait for second TTL to expire
			time.Sleep(150 * time.Millisecond)
			
			// Both should be expired
			_, found1 = cache.Get("key1")
			_, found2 = cache.Get("key2")
			Expect(found1).To(BeFalse())
			Expect(found2).To(BeFalse())
		})
	})

	Describe("Metrics", func() {
		It("should track cache hits", func() {
			cache.Set("key1", "value1")
			
			initialHits := cache.GetHits()
			
			// Perform hits
			cache.Get("key1")
			cache.Get("key1")
			cache.Get("key1")
			
			hits := cache.GetHits()
			Expect(hits).To(Equal(initialHits + 3))
		})

		It("should track cache misses", func() {
			initialMisses := cache.GetMisses()
			
			// Perform misses
			cache.Get("nonexistent1")
			cache.Get("nonexistent2")
			cache.Get("nonexistent3")
			
			misses := cache.GetMisses()
			Expect(misses).To(Equal(initialMisses + 3))
		})

		It("should track expired entries as misses", func() {
			cache.Set("key1", "value1")
			
			// Wait for expiration
			time.Sleep(150 * time.Millisecond)
			
			initialMisses := cache.GetMisses()
			
			// Try to get expired entry
			cache.Get("key1")
			
			misses := cache.GetMisses()
			Expect(misses).To(Equal(initialMisses + 1))
		})
		
		It("should track evictions", func() {
			cache.Set("key1", "value1")
			
			// Wait for expiration
			time.Sleep(150 * time.Millisecond)
			
			// Wait for cleanup to run
			time.Sleep(100 * time.Millisecond)
			
			evictions := cache.GetEvictions()
			Expect(evictions).To(BeNumerically(">=", 1))
		})
	})

	Describe("Concurrent Access", func() {
		It("should handle concurrent reads and writes safely", func() {
			done := make(chan bool)
			
			// Start multiple goroutines writing
			for i := 0; i < 10; i++ {
				go func(id int) {
					for j := 0; j < 100; j++ {
						key := fmt.Sprintf("key%d-%d", id, j)
						value := fmt.Sprintf("value%d-%d", id, j)
						cache.Set(key, value)
					}
				}(i)
			}
			
			// Start multiple goroutines reading
			for i := 0; i < 10; i++ {
				go func(id int) {
					for j := 0; j < 100; j++ {
						key := fmt.Sprintf("key%d-%d", id, j)
						cache.Get(key)
					}
				}(i)
			}
			
			// Give goroutines time to complete
			go func() {
				time.Sleep(200 * time.Millisecond)
				done <- true
			}()
			
			Eventually(done, 1*time.Second).Should(Receive())
			
			// Verify some values exist
			value, found := cache.Get("key0-0")
			if found {
				Expect(value).To(Equal("value0-0"))
			}
		})

		It("should handle concurrent deletes safely", func() {
			// Populate cache
			for i := 0; i < 100; i++ {
				cache.Set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
			}
			
			done := make(chan bool)
			
			// Start multiple goroutines deleting
			for i := 0; i < 10; i++ {
				go func(start int) {
					for j := start; j < start+10; j++ {
						cache.Delete(fmt.Sprintf("key%d", j))
					}
				}(i * 10)
			}
			
			// Give goroutines time to complete
			go func() {
				time.Sleep(200 * time.Millisecond)
				done <- true
			}()
			
			Eventually(done, 1*time.Second).Should(Receive())
			
			// Verify all keys are deleted
			for i := 0; i < 100; i++ {
				_, found := cache.Get(fmt.Sprintf("key%d", i))
				Expect(found).To(BeFalse())
			}
		})
	})

	Describe("Different Value Types", func() {
		It("should store different types of values", func() {
			type TestStruct struct {
				Name  string
				Value int
			}
			
			// Store different types
			cache.Set("string", "test string")
			cache.Set("int", 42)
			cache.Set("float", 3.14)
			cache.Set("bool", true)
			cache.Set("struct", TestStruct{Name: "test", Value: 100})
			cache.Set("slice", []int{1, 2, 3})
			cache.Set("map", map[string]int{"a": 1, "b": 2})
			
			// Retrieve and verify
			strVal, _ := cache.Get("string")
			Expect(strVal).To(Equal("test string"))
			
			intVal, _ := cache.Get("int")
			Expect(intVal).To(Equal(42))
			
			floatVal, _ := cache.Get("float")
			Expect(floatVal).To(BeNumerically("~", 3.14))
			
			boolVal, _ := cache.Get("bool")
			Expect(boolVal).To(Equal(true))
			
			structVal, _ := cache.Get("struct")
			Expect(structVal).To(Equal(TestStruct{Name: "test", Value: 100}))
			
			sliceVal, _ := cache.Get("slice")
			Expect(sliceVal).To(Equal([]int{1, 2, 3}))
			
			mapVal, _ := cache.Get("map")
			Expect(mapVal).To(Equal(map[string]int{"a": 1, "b": 2}))
		})
	})
	
	Describe("Cache Lifecycle", func() {
		It("should properly close and stop cleanup goroutine", func() {
			cache.Set("key1", "value1")
			
			// Close the cache
			cache.Close()
			
			// Operations after close should not panic
			value, found := cache.Get("key1")
			Expect(found).To(BeFalse())
			Expect(value).To(BeNil())
			
			// Set should be ignored after close
			cache.Set("key2", "value2")
			_, found = cache.Get("key2")
			Expect(found).To(BeFalse())
		})
		
		It("should handle multiple close calls gracefully", func() {
			// Multiple closes should not panic
			cache.Close()
			cache.Close()
			cache.Close()
		})
	})
})
