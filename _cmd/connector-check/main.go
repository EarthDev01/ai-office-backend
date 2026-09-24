// connector-check ตรวจ connector ทีละ kind: โหลด + ครอบ 31 ข้อ + contract test
//
//	go run ./_cmd/connector-check connectors/office-v10x [connectors/office-abatech …]
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"ai-office-backend/internal/core/connector"
)

// รอบแรก (spec §4 · ภาคผนวก ก) ไม่รวมหมวด E — ต้องตรงกับ test/connectors_test.go
var firstRound = []string{
	"BQ-01", "BQ-02", "BQ-03", "BQ-04", "BQ-05", "BQ-06", "BQ-07", "BQ-08", "BQ-09", "BQ-10", "BQ-11", "BQ-12", "BQ-13",
	"BQ-29", "BQ-35",
	"BQ-41", "BQ-42", "BQ-43", "BQ-44", "BQ-45", "BQ-46", "BQ-47", "BQ-48", "BQ-49", "BQ-50", "BQ-51",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: connector-check connectors/<kind> …")
		os.Exit(2)
	}
	bad := false
	for _, dir := range os.Args[1:] {
		fmt.Printf("== %s\n", dir)
		c, err := connector.Load(dir)
		if err != nil {
			fmt.Printf("✗ โหลดไม่ผ่าน:\n%v\n", err)
			bad = true
			continue
		}
		fmt.Printf("✓ โหลดผ่าน · tool %d · เมนู %d · guide %d · ตารางสถานะ %d · fact %d\n",
			len(c.Tools), len(c.Menus.Menus), len(c.Menus.Guides), len(c.Statuses.Tables), len(c.Host.Facts))

		cov := c.Coverage()
		var missing []string
		for _, q := range firstRound {
			if len(cov[q]) == 0 {
				missing = append(missing, q)
			}
		}
		qs := make([]string, 0, len(cov))
		for q := range cov {
			qs = append(qs, q)
		}
		sort.Strings(qs)
		for _, q := range qs {
			fmt.Printf("   %s ← %s\n", q, strings.Join(cov[q], ", "))
		}
		if len(missing) > 0 {
			fmt.Printf("✗ ยังไม่ครอบ: %v\n", missing)
			bad = true
		} else {
			fmt.Printf("✓ ครอบ %d ข้อรอบแรก\n", len(firstRound))
		}

		files, err := c.LoadContracts()
		if err != nil {
			fmt.Printf("✗ contract-tests อ่านไม่ได้: %v\n", err)
			bad = true
			continue
		}
		tested := map[string]bool{}
		fails, cases := 0, 0
		for _, f := range files {
			tested[f.Tool] = true
			cases += len(f.Cases)
			for _, e := range c.RunContract(f) {
				fmt.Printf("✗ %v\n", e)
				fails++
			}
		}
		for _, t := range c.Tools {
			if !tested[t.Name] {
				fmt.Printf("✗ tool %s ไม่มี contract test\n", t.Name)
				fails++
			}
		}
		if fails > 0 {
			bad = true
		} else {
			fmt.Printf("✓ contract %d ไฟล์ %d case ผ่าน\n", len(files), cases)
		}
	}
	if bad {
		os.Exit(1)
	}
}
