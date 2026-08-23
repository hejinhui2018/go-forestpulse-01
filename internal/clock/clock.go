package clock

import "time"

type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type Real struct{}

func (Real) Now() time.Time                         { return time.Now().UTC() }
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }

type Fixed struct{ Current time.Time }

func (f *Fixed) Now() time.Time { return f.Current }
func (f *Fixed) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- f.Current.Add(d)
	return ch
}
func (f *Fixed) Advance(d time.Duration) { f.Current = f.Current.Add(d) }
