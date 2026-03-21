package ollama

import (
	"math"
	"testing"
)

func TestFloat32Roundtrip(t *testing.T) {
	original := []float32{1.0, -2.5, 0.0, 3.14159, math.MaxFloat32, math.SmallestNonzeroFloat32}
	bytes := Float32ToBytes(original)
	result := BytesToFloat32(bytes)

	if len(result) != len(original) {
		t.Fatalf("length mismatch: %d vs %d", len(result), len(original))
	}

	for i := range original {
		if result[i] != original[i] {
			t.Errorf("index %d: expected %f, got %f", i, original[i], result[i])
		}
	}
}

func TestFloat32Roundtrip_Empty(t *testing.T) {
	bytes := Float32ToBytes(nil)
	result := BytesToFloat32(bytes)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d elements", len(result))
	}
}

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1, 2, 3}
	sim := CosineSimilarity(a, a)
	if math.Abs(float64(sim)-1.0) > 1e-6 {
		t.Errorf("identical vectors should have similarity ~1.0, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{0, 1, 0}
	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)) > 1e-6 {
		t.Errorf("orthogonal vectors should have similarity ~0, got %f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	sim := CosineSimilarity(a, b)
	if math.Abs(float64(sim)+1.0) > 1e-6 {
		t.Errorf("opposite vectors should have similarity ~-1.0, got %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{1, 2, 3}
	zero := []float32{0, 0, 0}
	sim := CosineSimilarity(a, zero)
	if sim != 0 {
		t.Errorf("zero vector should give similarity 0, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	sim := CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("different-length vectors should give 0, got %f", sim)
	}
}

func TestCosineSimilarity_Similar(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{1, 2, 4} // slightly different
	sim := CosineSimilarity(a, b)
	if sim < 0.98 || sim > 1.0 {
		t.Errorf("similar vectors should have high similarity, got %f", sim)
	}
}
