package main

import (
	"fmt"
	"time"
)

// passNotifyLead is how long before AOS the "pass soon" notification fires.
const passNotifyLead = 5 * time.Minute

// passNotice carries the data for a "pass soon" OS notification. It is built
// under the frame lock and handed to a goroutine so the OS call is off-lock.
type passNotice struct {
	name  string
	aos   time.Time
	maxEl float64
}

// duePassNotification returns the soonest tracked pass that starts within
// passNotifyLead and marks it announced so it only fires once. It must be
// called under the frame lock.
func (s *State) duePassNotification(now time.Time) *passNotice {
	if !s.cfg.notifyEnabled() {
		return nil
	}
	var best *Pass
	for _, p := range s.passes {
		if p.notified {
			continue
		}
		if !p.Details.AOS.After(now) {
			// Already started or over: never announce it late, and mark
			// finished passes so they are not rescanned forever.
			if p.Details.LOS.Before(now) {
				p.notified = true
			}
			continue
		}
		if p.Details.AOS.Sub(now) <= passNotifyLead {
			if best == nil || p.Details.AOS.Before(best.Details.AOS) {
				best = p
			}
		}
	}
	if best == nil {
		return nil
	}
	best.notified = true
	return &passNotice{name: best.Name, aos: best.Details.AOS, maxEl: best.Details.MaxElevation}
}

// announcePass posts the OS notification in the background so spawning a
// notifier process never blocks the UI ticker.
func announcePass(p *passNotice) {
	if p == nil {
		return
	}
	title := "SatSat · pass in 5 minutes"
	body := fmt.Sprintf("%s rises at %s — max elevation %.0f°.",
		p.name, p.aos.Local().Format("15:04:05"), p.maxEl)
	go func() { _ = sendNotification(title, body) }()
}
