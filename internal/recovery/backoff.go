package recovery

import "time"

type Backoff struct {
	Initial  time.Duration
	Maximum  time.Duration
	Attempts int
}

func (b Backoff) Delay() time.Duration {
	if b.Initial <= 0 {
		b.Initial = time.Second
	}
	if b.Maximum <= 0 {
		b.Maximum = time.Minute
	}
	d := b.Initial
	for i := 0; i < b.Attempts; i++ {
		if d >= b.Maximum/2 {
			return b.Maximum
		}
		d *= 2
	}
	if d > b.Maximum {
		return b.Maximum
	}
	return d
}
func (b *Backoff) Failed()               { b.Attempts++ }
func (b *Backoff) Succeeded()            { b.Attempts = 0 }
func (b Backoff) Exhausted(max int) bool { return max > 0 && b.Attempts >= max }
