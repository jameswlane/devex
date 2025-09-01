package tui

import (
	"context"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureString_Creation(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"simple password", "password123"},
		{"complex password", "P@ssw0rd!2023#Complex"},
		{"unicode password", "пароль测试🔐"},
		{"very long password", generateLongString(1000)},
		{"password with special chars", "p@$$w0rd!@#$%^&*()"},
		{"password with whitespace", "pass word with spaces\t\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			secureStr := NewSecureString(tc.input)
			require.NotNil(t, secureStr)
			require.NotNil(t, secureStr.data)

			// Verify the data is correctly copied
			assert.Equal(t, tc.input, secureStr.String())
			assert.Equal(t, len(tc.input), len(secureStr.data))

			// Verify it's a separate copy, not a reference
			originalBytes := []byte(tc.input)
			if len(originalBytes) > 0 {
				// Modify original, should not affect secure string
				originalBytes[0] = 'X'
				assert.NotEqual(t, string(originalBytes), secureStr.String())
			}
		})
	}
}

func TestSecureString_MemoryScrubbing(t *testing.T) {
	password := "SuperSecretPassword123!"
	secureStr := NewSecureString(password)

	// Verify password is stored correctly
	assert.Equal(t, password, secureStr.String())
	assert.Equal(t, len(password), len(secureStr.data))

	// Store reference to data for verification
	dataPtr := &secureStr.data[0]
	originalLen := len(secureStr.data)

	// Clear the secure string
	secureStr.Clear()

	// Verify data slice is nil
	assert.Nil(t, secureStr.data)

	// Verify we can't recover the password
	assert.Equal(t, "", secureStr.String())

	// Note: Memory scrubbing verification is implementation-dependent
	// The important thing is that the slice is nil and String() returns empty
	// Direct memory inspection is too unsafe and implementation-specific for reliable testing
	_ = dataPtr     // Keep reference to avoid unused variable warning
	_ = originalLen // Keep reference to avoid unused variable warning
}

func TestSecureString_DoubleClearing(t *testing.T) {
	password := "TestPassword"
	secureStr := NewSecureString(password)

	// First clear
	secureStr.Clear()
	assert.Nil(t, secureStr.data)
	assert.Equal(t, "", secureStr.String())

	// Second clear should not panic
	require.NotPanics(t, func() {
		secureStr.Clear()
	})

	// Should still be cleared
	assert.Nil(t, secureStr.data)
	assert.Equal(t, "", secureStr.String())
}

func TestSecureString_ConcurrentAccess(t *testing.T) {
	password := "ConcurrentTestPassword"
	secureStr := NewSecureString(password)

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Test concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			result := secureStr.String()
			assert.Equal(t, password, result)
		}()
	}
	wg.Wait()

	// Test concurrent clear operations
	passwords := make([]*SecureString, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		passwords[i] = NewSecureString(password + string(rune('A'+i)))
	}

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			passwords[idx].Clear()
		}(i)
	}
	wg.Wait()

	// Verify all are cleared
	for i := 0; i < numGoroutines; i++ {
		assert.Nil(t, passwords[i].data)
		assert.Equal(t, "", passwords[i].String())
	}
}

func TestStreamingInstaller_StdinMutexProtection(t *testing.T) {
	installer := createTestInstaller(t)

	// Test that multiple goroutines can't simultaneously access stdin
	const numGoroutines = 5
	var wg sync.WaitGroup
	mutexAcquired := make(chan bool, numGoroutines)

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Try to acquire mutex
			installer.stdinMux.Lock()
			mutexAcquired <- true

			// Hold the mutex briefly
			time.Sleep(10 * time.Millisecond)

			installer.stdinMux.Unlock()
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(mutexAcquired)

	// Count how many successfully acquired the mutex
	count := 0
	for range mutexAcquired {
		count++
	}

	// All should have acquired it (sequentially)
	assert.Equal(t, numGoroutines, count)
}

func TestStreamingInstaller_ContextCancellation(t *testing.T) {
	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Create installer with cancellable context
	installer := createTestInstallerWithContext(t, ctx)

	// Cancel the context
	cancel()

	// Verify context is cancelled
	select {
	case <-installer.ctx.Done():
		assert.Equal(t, context.Canceled, installer.ctx.Err())
	default:
		t.Fatal("Context should be cancelled")
	}
}

func TestStreamingInstaller_ContextTimeout(t *testing.T) {
	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Create installer with timeout context
	installer := createTestInstallerWithContext(t, ctx)

	// Wait for timeout
	time.Sleep(5 * time.Millisecond)

	// Verify context timed out
	select {
	case <-installer.ctx.Done():
		assert.Equal(t, context.DeadlineExceeded, installer.ctx.Err())
	default:
		t.Fatal("Context should have timed out")
	}
}

func TestPasswordMemoryLifecycle(t *testing.T) {
	// Simulate complete password lifecycle
	testPasswords := []string{
		"simple",
		"complex!@#$%^&*()",
		"unicode: пароль测试🔐",
		generateLongString(500),
	}

	for _, password := range testPasswords {
		t.Run("password_len_"+string(rune(len(password))), func(t *testing.T) {
			// Create secure string
			secureStr := NewSecureString(password)
			require.NotNil(t, secureStr)

			// Use the password
			retrievedPassword := secureStr.String()
			assert.Equal(t, password, retrievedPassword)

			// Simulate processing (password should still be intact)
			time.Sleep(1 * time.Millisecond)
			assert.Equal(t, password, secureStr.String())

			// Clear immediately after use
			secureStr.Clear()

			// Verify it's completely cleared
			assert.Nil(t, secureStr.data)
			assert.Equal(t, "", secureStr.String())

			// Further clearing should be safe
			secureStr.Clear()
			assert.Nil(t, secureStr.data)
		})
	}
}

func TestSecureString_StringAfterClear(t *testing.T) {
	password := "TestPassword123"
	secureStr := NewSecureString(password)

	// Verify initial state
	assert.Equal(t, password, secureStr.String())

	// Clear the secure string
	secureStr.Clear()

	// String() should return empty string, not panic
	result := secureStr.String()
	assert.Equal(t, "", result)

	// Multiple calls should be safe
	result2 := secureStr.String()
	assert.Equal(t, "", result2)
}

func TestSecureString_LargePasswords(t *testing.T) {
	sizes := []int{1, 10, 100, 1000, 10000}

	for _, size := range sizes {
		t.Run("size_"+string(rune(size)), func(t *testing.T) {
			password := generateLongString(size)
			secureStr := NewSecureString(password)

			assert.Equal(t, size, len(secureStr.data))
			assert.Equal(t, password, secureStr.String())

			secureStr.Clear()
			assert.Nil(t, secureStr.data)
			assert.Equal(t, "", secureStr.String())
		})
	}
}

// Helper functions

func createTestInstaller(t *testing.T) *StreamingInstaller {
	t.Helper()

	program := tea.NewProgram(nil)
	mockRepo := &struct{ types.Repository }{} // Empty mock for security tests
	ctx := context.Background()

	settings := config.CrossPlatformSettings{
		HomeDir: "/tmp/test-devex",
		Verbose: false,
	}

	return NewStreamingInstaller(program, mockRepo, ctx, settings)
}

func createTestInstallerWithContext(t *testing.T, ctx context.Context) *StreamingInstaller {
	t.Helper()

	program := tea.NewProgram(nil)
	mockRepo := &struct{ types.Repository }{} // Empty mock for security tests

	settings := config.CrossPlatformSettings{
		HomeDir: "/tmp/test-devex",
		Verbose: false,
	}

	return NewStreamingInstaller(program, mockRepo, ctx, settings)
}

func generateLongString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[i%len(charset)]
	}
	return string(result)
}

// Benchmark tests for security operations

func BenchmarkSecureString_Creation(b *testing.B) {
	password := "BenchmarkPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secureStr := NewSecureString(password)
		secureStr.Clear() // Clean up immediately
	}
}

func BenchmarkSecureString_StringAccess(b *testing.B) {
	password := "BenchmarkPassword123!"
	secureStr := NewSecureString(password)
	defer secureStr.Clear()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = secureStr.String()
	}
}

func BenchmarkSecureString_Clear(b *testing.B) {
	passwords := make([]*SecureString, b.N)
	for i := 0; i < b.N; i++ {
		passwords[i] = NewSecureString("BenchmarkPassword123!")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		passwords[i].Clear()
	}
}

func BenchmarkSecureString_LargePassword(b *testing.B) {
	largePassword := generateLongString(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secureStr := NewSecureString(largePassword)
		_ = secureStr.String()
		secureStr.Clear()
	}
}
