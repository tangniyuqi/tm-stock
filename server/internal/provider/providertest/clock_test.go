package providertest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func Test假时钟_起点与推进(t *testing.T) {
	def := NewFakeClock(time.Time{})
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !def.Now().Equal(want) {
		t.Fatalf("默认起点 = %v，期望 %v", def.Now(), want)
	}

	start := time.Date(2030, 5, 6, 7, 8, 9, 0, time.UTC)
	c := NewFakeClock(start)
	if !c.Now().Equal(start) {
		t.Fatalf("自定义起点 = %v，期望 %v", c.Now(), start)
	}
	c.Advance(90 * time.Second)
	if want := start.Add(90 * time.Second); !c.Now().Equal(want) {
		t.Fatalf("推进后 = %v，期望 %v", c.Now(), want)
	}
	c.Advance(0)
	c.Advance(-time.Hour)
	if want := start.Add(90 * time.Second); !c.Now().Equal(want) {
		t.Fatalf("非正的推进应被忽略，现在 = %v，期望 %v", c.Now(), want)
	}
}

func Test假时钟_Sleep记录时长并推进时间(t *testing.T) {
	c := NewFakeClock(time.Time{})
	before := c.Now()
	ctx := context.Background()

	for _, d := range []time.Duration{100 * time.Millisecond, 0, -time.Second, 2 * time.Second} {
		if err := c.Sleep(ctx, d); err != nil {
			t.Fatalf("Sleep(%v) 返回 %v", d, err)
		}
	}
	want := []time.Duration{100 * time.Millisecond, 0, -time.Second, 2 * time.Second}
	got := c.Sleeps()
	if len(got) != len(want) {
		t.Fatalf("Sleeps() = %v，期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Sleeps() = %v，期望 %v", got, want)
		}
	}
	// 只有正的时长才推进虚拟时间。
	if elapsed := c.Now().Sub(before); elapsed != 2100*time.Millisecond {
		t.Fatalf("虚拟时间推进了 %v，期望 2.1s", elapsed)
	}

	// Sleeps 返回副本。
	got[0] = time.Hour
	if c.Sleeps()[0] != 100*time.Millisecond {
		t.Fatal("Sleeps() 应返回副本")
	}
}

func Test假时钟_ctx已结束时Sleep不记录也不推进(t *testing.T) {
	c := NewFakeClock(time.Time{})
	before := c.Now()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c.Sleep(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("Sleep 应返回 ctx.Err()，实际 %v", err)
	}
	if len(c.Sleeps()) != 0 || !c.Now().Equal(before) {
		t.Fatalf("ctx 已结束时不应记录或推进: sleeps=%v now=%v", c.Sleeps(), c.Now())
	}
}

func Test假时钟_并发安全(t *testing.T) {
	c := NewFakeClock(time.Time{})
	before := c.Now()
	const workers = 100
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.Sleep(context.Background(), time.Millisecond)
			c.Advance(time.Millisecond)
			_ = c.Now()
			_ = c.Sleeps()
		}()
	}
	wg.Wait()
	if len(c.Sleeps()) != workers {
		t.Fatalf("Sleeps 数 = %d，期望 %d", len(c.Sleeps()), workers)
	}
	if elapsed := c.Now().Sub(before); elapsed != 2*workers*time.Millisecond {
		t.Fatalf("虚拟时间推进了 %v，期望 %v", elapsed, 2*workers*time.Millisecond)
	}
}
