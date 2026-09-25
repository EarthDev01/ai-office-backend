// Package docs ฝังคู่มือการใช้งานคอนโซลเข้าไปในตัวโปรแกรม — หน้า /guide และผู้ช่วย AI ในคอนโซลอ่านจากไฟล์เดียวกัน
package docs

import _ "embed"

// Guide คือเนื้อหา guide.md (Markdown)
//
//go:embed guide.md
var Guide string
