package model

import (
	"reflect"
	"testing"
)

func TestOrderClaims_ExplicitOrderFirstThenIncomingFallback(t *testing.T) {
	claim := func(id string, order int) Claim { return Claim{ID: id, Order: order} }
	claims := []Claim{
		claim("no-order-a", 0),
		claim("order-5", 5),
		claim("no-order-b", 0),
		claim("order-2", 2),
	}

	got := OrderClaims(claims)

	var gotIDs []string
	for _, c := range got {
		gotIDs = append(gotIDs, c.ID)
	}
	want := []string{"order-2", "order-5", "no-order-a", "no-order-b"}
	if len(gotIDs) != len(want) {
		t.Fatalf("got %v, want %v", gotIDs, want)
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Fatalf("got %v, want %v", gotIDs, want)
		}
	}
}

func TestOrderClaims_DoesNotMutateInput(t *testing.T) {
	claims := []Claim{{ID: "a", Order: 2}, {ID: "b", Order: 1}}
	inputCopy := append([]Claim(nil), claims...)
	_ = OrderClaims(claims)
	for i := range claims {
		if !reflect.DeepEqual(claims[i], inputCopy[i]) {
			t.Fatalf("OrderClaims mutated its input slice at index %d", i)
		}
	}
}
