package policy

import (
	"fmt"
	"sort"

	"example.com/forestpulse/internal/model"
)

type SequenceWindow struct {
	First int64
	Last  int64
	Count int
}

func Window(readings []model.Reading) SequenceWindow {
	if len(readings) == 0 {
		return SequenceWindow{}
	}
	first, last := readings[0].Sequence, readings[0].Sequence
	for _, r := range readings[1:] {
		if r.Sequence < first {
			first = r.Sequence
		}
		if r.Sequence > last {
			last = r.Sequence
		}
	}
	return SequenceWindow{First: first, Last: last, Count: len(readings)}
}
func (w SequenceWindow) Complete() bool { return w.Count > 0 && w.Last-w.First+1 == int64(w.Count) }
func (w SequenceWindow) String() string {
	return fmt.Sprintf("%d..%d (%d readings)", w.First, w.Last, w.Count)
}
func ValidateOrdered(readings []model.Reading) error {
	if len(readings) == 0 {
		return fmt.Errorf("sequence: empty reading list")
	}
	copyRows := append([]model.Reading(nil), readings...)
	sort.Slice(copyRows, func(i, j int) bool { return copyRows[i].Sequence < copyRows[j].Sequence })
	for i := 1; i < len(copyRows); i++ {
		if copyRows[i].Sequence == copyRows[i-1].Sequence {
			return fmt.Errorf("sequence: duplicate %d", copyRows[i].Sequence)
		}
		if copyRows[i].Sequence != copyRows[i-1].Sequence+1 {
			return fmt.Errorf("sequence: gap between %d and %d", copyRows[i-1].Sequence, copyRows[i].Sequence)
		}
	}
	return nil
}
func ExpectedAfter(cursor int64, batch model.ReadingBatch) error {
	if batch.FirstSequence() != cursor+1 {
		return fmt.Errorf("sequence: expected %d, got %d", cursor+1, batch.FirstSequence())
	}
	return nil
}
