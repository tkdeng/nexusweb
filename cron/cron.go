package cron

import (
	"io"
	"sync"
	"time"

	"github.com/tkdeng/goutil"
)

type cronJob struct {
	interval int64
	last     *int64
	cb       func() bool
}

var cron map[string]cronJob = map[string]cronJob{}
var cronMU sync.Mutex

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Second)

			now := time.Now().UnixMilli()

			cronMU.Lock()

			for key, c := range cron {
				if now > *c.last+c.interval {
					*cron[key].last = now
					if !c.cb() {
						delete(cron, key)
					}
				}
			}

			cronMU.Unlock()
		}
	}()
}

// NewCron adds a new, unnamed cron job to the queue
//
// minimum interval: 1 minute
//
// in the callback, return true to keep the job running,
// and return false to end the job
func New(interval time.Duration, cb func() bool) error {
	intrv := interval.Milliseconds()
	if intrv < 60000 {
		intrv = 60000
	}

	now := time.Now().UnixMilli()

	cronMU.Lock()
	defer cronMU.Unlock()

	name := "+job:" + string(goutil.URandBytes(16))

	loops := 1000
	for loops > 0 {
		if _, ok := cron[name]; !ok {
			break
		}
		loops--
		name += string(goutil.URandBytes(16))
	}

	if _, ok := cron[name]; ok {
		return io.EOF
	}

	cron[name] = cronJob{
		interval: intrv,
		last:     &now,
		cb:       cb,
	}

	return nil
}

// SetCron adds or overwrites a named cron job
func Set(name string, interval time.Duration, cb func() bool) {
	name = "#job:" + name

	intrv := interval.Milliseconds()
	if intrv < 60000 {
		intrv = 60000
	}

	now := time.Now().UnixMilli()

	cronMU.Lock()
	defer cronMU.Unlock()

	cron[name] = cronJob{
		interval: intrv,
		last:     &now,
		cb:       cb,
	}
}

// HasCron checks if a named cron job exists
func Has(name string) bool {
	name = "#job:" + name

	cronMU.Lock()
	defer cronMU.Unlock()

	if _, ok := cron[name]; ok {
		return true
	}
	return false
}

// DelCron removes a named cron job
func Del(name string) {
	name = "#job:" + name

	cronMU.Lock()
	defer cronMU.Unlock()

	delete(cron, name)
}
