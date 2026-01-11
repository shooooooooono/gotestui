package examples

import (
	"testing"
	"time"
)

func TestParallel(t *testing.T) {
	t.Run("worker_1", func(t *testing.T) {
		t.Parallel()
		for i := 1; i <= 3; i++ {
			t.Logf("worker_1: step %d/3", i)
			time.Sleep(1 * time.Second)
		}
	})
	t.Run("worker_2", func(t *testing.T) {
		t.Parallel()
		for i := 1; i <= 4; i++ {
			t.Logf("worker_2: step %d/4", i)
			time.Sleep(800 * time.Millisecond)
		}
	})
	t.Run("worker_3", func(t *testing.T) {
		t.Parallel()
		for i := 1; i <= 5; i++ {
			t.Logf("worker_3: step %d/5", i)
			time.Sleep(600 * time.Millisecond)
		}
	})
	t.Run("worker_fail", func(t *testing.T) {
		t.Parallel()
		t.Log("worker_fail: starting...")
		time.Sleep(2 * time.Second)
		t.Log("worker_fail: about to fail")
		t.Fail()
	})
}

func TestExample(t *testing.T) {
	t.Run("count1", func(t *testing.T) {
		t.Log("aaaaa")
	})
	t.Run("count2", func(t *testing.T) {
		t.Log("aaaaa")
	})
	t.Run("example test 01", func(t *testing.T) {
		t.Run("test_pass", func(t *testing.T) {
			t.Log("Passlog01")
			t.Log("Passlog02")
			t.Log("Passlog03")
			t.Log("Passlog04")
		})
		t.Run("test_fail", func(t *testing.T) {
			t.Log("Faillog01")
			t.Log("Faillog02")
			t.Log("Faillog03")
			t.Log("Faillog04")

			t.Run("test_fail_child", func(t *testing.T) {
				t.Log("Faillog Child01")
				t.Fail()
			})

		})
		t.Run("test_skip", func(t *testing.T) {
			time.Sleep(1 * time.Second)
			t.Log("Skiplog01")
			time.Sleep(1 * time.Second)
			t.Log("Skiplog02")
			time.Sleep(1 * time.Second)
			t.Log("Skiplog03")
			time.Sleep(1 * time.Second)
			t.Log("Skiplog04")
			t.Skip()
		})
		t.Run("test_long_log", func(t *testing.T) {
			for i := 1; i <= 100; i++ {
				t.Logf("Line %03d: This is a long log message for testing scroll functionality in the log viewer.", i)
			}
		})
	})
}
