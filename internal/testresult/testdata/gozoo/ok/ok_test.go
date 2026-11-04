package ok

import "testing"

func TestPass(t *testing.T) {}

func TestFail(t *testing.T) { t.Fatalf("Add(1, 2) = %d", Add(1, 2)) }

func TestSkip(t *testing.T) { t.Skip("not today") }

func TestTable(t *testing.T) {
	t.Run("one", func(t *testing.T) {})
	t.Run("two words", func(t *testing.T) { t.Error("bad") })
	t.Run("nested", func(t *testing.T) {
		t.Run("deep", func(t *testing.T) {})
	})
}
