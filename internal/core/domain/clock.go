package domain

import (
	"sync"
	"time"
)

var (
	bkkOnce sync.Once
	bkkLoc  *time.Location
)

// Bangkok = timezone หลักของระบบ (รอบเดือนโควตา · วันของ rollup) · ไม่มี tzdata ก็ยังถูก (+07:00 ไม่มี DST)
func Bangkok() *time.Location {
	bkkOnce.Do(func() {
		l, err := time.LoadLocation("Asia/Bangkok")
		if err != nil {
			l = time.FixedZone("ICT", 7*3600)
		}
		bkkLoc = l
	})
	return bkkLoc
}

// PeriodOf = รอบโควตา YYYY-MM ตามเวลาไทย (ตัดรอบวันที่ 1 00:00 Bangkok — AC-25)
func PeriodOf(t time.Time) string { return t.In(Bangkok()).Format("2006-01") }

// DateOf = วัน YYYY-MM-DD ตามเวลาไทย
func DateOf(t time.Time) string { return t.In(Bangkok()).Format("2006-01-02") }
