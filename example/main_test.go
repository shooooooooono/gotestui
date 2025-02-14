package example

import (
	"testing"
	"time"
)

func TestExample(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		t.Log("aaaaa")
	})
	t.Run("test", func(t *testing.T) {
		t.Log("aaaaa")
	})
	t.Run("example test 01", func(t *testing.T) {
		t.Run("pass", func(t *testing.T) {
			t.Log("Passlog01")
			t.Log("Passlog02")
			t.Log("Passlog03")
			t.Log("Passlog04")
		})
		t.Run("fail", func(t *testing.T) {
			t.Log("Faillog01")
			t.Log("Faillog02")
			t.Log("Faillog03")
			t.Log("Faillog04")

			t.Run("fail child", func(t *testing.T) {
				t.Log("Faillog Child01")
				t.Fail()
			})

		})
		t.Run("skip", func(t *testing.T) {
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
	})
}

// {
//   "Time": "2025-02-08T19:06:48.770859257+09:00",
//   "Action": "start",
//   "Package": "github.com/shooooooooono/gotestui/example"
// }
// {
//   "Time": "2025-02-08T19:06:48.773041675+09:00",
//   "Action": "run",
//   "Package": "github.com/shooooooooono/gotestui/example",
//   "Test": "TestExample"
// }
